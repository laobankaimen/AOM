package server

import (
	"aom/internal/core"
	"aom/internal/vault"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "aom_session"
const sessionLifetime = 8 * time.Hour
const idleLifetime = 30 * time.Minute

type loginSession struct {
	username      string
	created, last time.Time
}
type attemptWindow struct {
	start time.Time
	count int
}
type AuthServer struct {
	closed                        bool
	mu                            sync.Mutex
	vault                         *vault.File
	store                         *core.Store
	application                   http.Handler
	public                        http.Handler
	web                           fs.FS
	dataDir, setupToken, lockPath string
	sessions                      map[[32]byte]loginSession
	attempts                      map[string]attemptWindow
	global                        attemptWindow
	now                           func() time.Time
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, e := rand.Read(bytes); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func NewAuthServer(dataDir string, web fs.FS, setupToken string) (*AuthServer, error) {
	if e := os.MkdirAll(dataDir, 0700); e != nil {
		return nil, e
	}
	lockPath := filepath.Join(dataDir, ".aom.lock")
	lock, e := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, fmt.Errorf("数据目录正在使用或保留了异常退出锁，请确认旧进程已停止后移除 .aom.lock：%w", e)
	}
	fmt.Fprintf(lock, "pid=%d\n", os.Getpid())
	lock.Close()
	fail := func(e error) (*AuthServer, error) { os.Remove(lockPath); return nil, e }
	file := vault.New(filepath.Join(dataDir, "app.vault"))
	exists, e := file.Exists()
	if e != nil {
		return fail(e)
	}
	if !exists {
		if setupToken == "" {
			setupToken, e = randomToken()
			if e != nil {
				return fail(e)
			}
		} else if len(setupToken) < 32 {
			return fail(errors.New("初始化令牌至少32字符"))
		}
	} else {
		setupToken = ""
	}
	a := &AuthServer{vault: file, dataDir: dataDir, web: web, setupToken: setupToken, lockPath: lockPath, sessions: map[[32]byte]loginSession{}, attempts: map[string]attemptWindow{}, now: time.Now}
	a.public = newApplication(nil, web)
	return a, nil
}
func (a *AuthServer) SetupToken() string { return a.setupToken }
func (a *AuthServer) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	a.sessions = map[[32]byte]loginSession{}
	var e error
	if a.store != nil {
		e = a.store.Close()
	}
	a.vault.Close()
	os.Remove(a.lockPath)
	return e
}
func (a *AuthServer) legacyExists() bool {
	info, e := os.Stat(filepath.Join(a.dataDir, "app.db"))
	return e == nil && !info.IsDir()
}
func (a *AuthServer) session(r *http.Request) ([32]byte, loginSession, bool) {
	cookie, e := r.Cookie(sessionCookie)
	if e != nil || len(cookie.Value) > 128 {
		return [32]byte{}, loginSession{}, false
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	session, ok := a.sessions[hash]
	now := a.now()
	if !ok || now.Sub(session.created) > sessionLifetime || now.Sub(session.last) > idleLifetime {
		delete(a.sessions, hash)
		return hash, loginSession{}, false
	}
	if r.URL.Path != "/api/auth/status" {
		session.last = now
		a.sessions[hash] = session
	}
	return hash, session, true
}
func (a *AuthServer) issue(w http.ResponseWriter, r *http.Request, username string) error {
	token, e := randomToken()
	if e != nil {
		return e
	}
	now := a.now()
	for key, s := range a.sessions {
		if now.Sub(s.created) > sessionLifetime || now.Sub(s.last) > idleLifetime {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= 32 {
		var oldest [32]byte
		var at time.Time
		for key, s := range a.sessions {
			if at.IsZero() || s.last.Before(at) {
				oldest = key
				at = s.last
			}
		}
		delete(a.sessions, oldest)
	}
	a.sessions[sha256.Sum256([]byte(token))] = loginSession{username: username, created: now, last: now}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds())})
	return nil
}
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func requestIP(r *http.Request) string {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *AuthServer) allowAttempt(r *http.Request) bool {
	now := a.now()
	const window = 15 * time.Minute
	if now.Sub(a.global.start) >= window {
		a.global = attemptWindow{start: now}
	}
	if a.global.count >= 60 {
		return false
	}
	key := requestIP(r)
	entry := a.attempts[key]
	if now.Sub(entry.start) >= window {
		entry = attemptWindow{start: now}
	}
	if entry.count >= 10 {
		return false
	}
	for ip, value := range a.attempts {
		if now.Sub(value.start) >= window {
			delete(a.attempts, ip)
		}
	}
	if len(a.attempts) >= 1024 {
		return false
	}
	entry.count++
	a.attempts[key] = entry
	a.global.count++
	return true
}
func safeOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		return e == nil && u.Host == r.Host && u.Scheme == scheme
	}
	return true
}
func trustedTransport(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	ip := net.ParseIP(requestIP(r))
	return ip != nil && ip.IsLoopback()
}
func (a *AuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		a.public.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/api/health" && r.Method == "GET" {
		reply(w, 200, map[string]string{"status": "ok"})
		return
	}
	if !trustedTransport(r) {
		reply(w, 403, map[string]string{"message": "局域网访问必须使用HTTPS；HTTP仅允许本机访问"})
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if !safeOrigin(r) {
			reply(w, 403, map[string]string{"message": "仅允许同源请求"})
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			reply(w, 415, map[string]string{"message": "请求须为application/json"})
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		reply(w, 503, map[string]string{"message": "服务正在关闭"})
		return
	}
	hash, current, authenticated := a.session(r)
	if r.URL.Path == "/api/auth/status" && r.Method == "GET" {
		exists, e := a.vault.Exists()
		if e != nil {
			internal(w, e)
			return
		}
		reply(w, 200, map[string]any{"authenticated": authenticated, "setupRequired": !exists, "legacyAvailable": !exists && a.legacyExists(), "legacyFilePresent": authenticated && a.legacyExists(), "username": current.username, "encrypted": true})
		return
	}
	if r.URL.Path == "/api/auth/login" && r.Method == "POST" {
		a.login(w, r)
		return
	}
	if r.URL.Path == "/api/auth/setup" && r.Method == "POST" {
		a.setup(w, r)
		return
	}
	if !authenticated || a.application == nil {
		clearSessionCookie(w, r)
		reply(w, 401, map[string]string{"message": "请先登录，或会话已过期"})
		return
	}
	if r.URL.Path == "/api/auth/logout" && r.Method == "POST" {
		delete(a.sessions, hash)
		clearSessionCookie(w, r)
		w.Header().Set("Clear-Site-Data", `"cache"`)
		reply(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.URL.Path == "/api/auth/password" && r.Method == "POST" {
		a.password(w, r, current.username)
		return
	}
	a.application.ServeHTTP(w, r)
}
func (a *AuthServer) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !decode(w, r, &req) {
		return
	}
	if len(req.Password) > 128 || !vault.UsernameValid(req.Username) {
		reply(w, 401, map[string]string{"message": "账号或密码不正确"})
		return
	}
	if !a.allowAttempt(r) {
		w.Header().Set("Retry-After", "900")
		reply(w, 429, map[string]string{"message": "尝试次数过多，请15分钟后再试"})
		return
	}
	if a.store == nil {
		image, e := a.vault.Open(req.Username, req.Password)
		if e != nil {
			reply(w, 401, map[string]string{"message": "账号或密码不正确，或加密数据无法解锁"})
			return
		}
		defer clear(image)
		store, e := core.OpenMemory(image, a.vault.Save)
		if e != nil {
			a.vault.Close()
			internal(w, e)
			return
		}
		a.store = store
		a.application = newApplication(store, a.web)
	} else if !a.vault.Verify(req.Username, req.Password) {
		reply(w, 401, map[string]string{"message": "账号或密码不正确"})
		return
	}
	if e := a.issue(w, r, req.Username); e != nil {
		internal(w, e)
		return
	}
	delete(a.attempts, requestIP(r))
	reply(w, 200, map[string]any{"authenticated": true, "username": req.Username, "legacyFilePresent": a.legacyExists()})
}
func (a *AuthServer) setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token          string `json:"token"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		ImportExisting bool   `json:"importExisting"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !decode(w, r, &req) {
		return
	}
	exists, e := a.vault.Exists()
	if e != nil {
		internal(w, e)
		return
	}
	if exists || a.setupToken == "" {
		reply(w, 409, map[string]string{"message": "账号已初始化，请登录"})
		return
	}
	if !a.allowAttempt(r) {
		reply(w, 429, map[string]string{"message": "尝试次数过多，请15分钟后再试"})
		return
	}
	expected := sha256.Sum256([]byte(a.setupToken))
	supplied := sha256.Sum256([]byte(req.Token))
	if subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
		reply(w, 403, map[string]string{"message": "初始化令牌不正确，请查看后台控制台"})
		return
	}
	if e = vault.PasswordValid(req.Password); e != nil || !vault.UsernameValid(req.Username) {
		reply(w, 400, map[string]string{"message": "账号须为3–64位字母/数字/._-，密码须为12–128字节"})
		return
	}
	var image []byte
	if a.legacyExists() {
		if !req.ImportExisting {
			reply(w, 400, map[string]string{"message": "检测到现有app.db，请勾选导入现有数据，避免创建空数据库"})
			return
		}
		image, e = core.ReadPlaintextImage(filepath.Join(a.dataDir, "app.db"))
		if e != nil {
			internal(w, e)
			return
		}
		defer clear(image)
	}
	store, e := core.OpenMemory(image, nil)
	if e != nil {
		internal(w, e)
		return
	}
	snapshot, e := store.ExportImage()
	store.Close()
	if e != nil {
		internal(w, e)
		return
	}
	defer clear(snapshot)
	if e = a.vault.Create(req.Username, req.Password, snapshot); e != nil {
		internal(w, e)
		return
	}
	store, e = core.OpenMemory(snapshot, a.vault.Save)
	if e != nil {
		a.vault.Close()
		internal(w, e)
		return
	}
	a.store = store
	a.application = newApplication(store, a.web)
	a.setupToken = ""
	if e = a.issue(w, r, req.Username); e != nil {
		internal(w, e)
		return
	}
	delete(a.attempts, requestIP(r))
	reply(w, 201, map[string]any{"authenticated": true, "username": req.Username, "legacyFilePresent": a.legacyExists()})
}
func (a *AuthServer) password(w http.ResponseWriter, r *http.Request, username string) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !decode(w, r, &req) {
		return
	}
	if !a.allowAttempt(r) {
		reply(w, 429, map[string]string{"message": "尝试次数过多，请15分钟后再试"})
		return
	}
	if e := vault.PasswordValid(req.NewPassword); e != nil {
		reply(w, 400, map[string]string{"message": e.Error()})
		return
	}
	if len(req.CurrentPassword) > 128 {
		reply(w, 400, map[string]string{"message": "密码长度无效"})
		return
	}
	image, e := a.store.ExportImage()
	if e != nil {
		internal(w, e)
		return
	}
	defer clear(image)
	if e = a.vault.ChangePassword(username, req.CurrentPassword, req.NewPassword, image); e != nil {
		if errors.Is(e, vault.ErrCredentials) {
			reply(w, 401, map[string]string{"message": "当前密码不正确"})
		} else {
			internal(w, e)
		}
		return
	}
	a.sessions = map[[32]byte]loginSession{}
	delete(a.attempts, requestIP(r))
	clearSessionCookie(w, r)
	reply(w, 200, map[string]bool{"reauthenticate": true})
}
