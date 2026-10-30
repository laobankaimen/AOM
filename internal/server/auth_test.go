package server

import (
	"aom/internal/core"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const setupTestToken = "test-only-initialization-token-123456789"
const authTestPassword = "Only-for-unit-tests-2026!"

func newAuthTest(t *testing.T) (*AuthServer, string) {
	t.Helper()
	dir := t.TempDir()
	a, e := NewAuthServer(dir, fstest.MapFS{"index.html": {Data: []byte("<html>LOGIN</html>")}}, setupTestToken)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { a.Close() })
	return a, dir
}
func authCall(a http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.RemoteAddr = "127.0.0.1:12345"
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func setupAuth(t *testing.T, a *AuthServer) *http.Cookie {
	t.Helper()
	w := authCall(a, "POST", "/api/auth/setup", map[string]any{"token": setupTestToken, "username": "admin", "password": authTestPassword}, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("session missing")
	}
	return cookies[0]
}
func TestAuthenticationProtectsEveryBusinessRoute(t *testing.T) {
	a, dir := newAuthTest(t)
	for _, route := range []string{"/api/state", "/api/audits", "/api/audits/1", "/api/export/devices.csv", "/api/export/plan.json", "/api/unknown"} {
		w := authCall(a, "GET", route, nil, nil)
		if w.Code != 401 {
			t.Fatalf("%s not protected: %d", route, w.Code)
		}
		if strings.Contains(w.Body.String(), "点位") {
			t.Fatal("data leaked")
		}
	}
	if w := authCall(a, "GET", "/", nil, nil); w.Code != 200 {
		t.Fatal("login assets blocked")
	}
	if w := authCall(a, "GET", "/api/health", nil, nil); w.Code != 200 {
		t.Fatal("health unavailable")
	}
	bad := authCall(a, "POST", "/api/auth/setup", map[string]string{"token": "bad", "username": "admin", "password": authTestPassword}, nil)
	if bad.Code != 403 {
		t.Fatal("setup without token allowed")
	}
	cookie := setupAuth(t, a)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge != 28800 {
		t.Fatalf("unsafe cookie: %+v", cookie)
	}
	if w := authCall(a, "GET", "/api/state", nil, cookie); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if _, e := os.Stat(filepath.Join(dir, "app.db")); !os.IsNotExist(e) {
		t.Fatal("plaintext database created")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "app.vault"))
	if bytes.Contains(data, []byte("点位")) || bytes.Contains(data, []byte(authTestPassword)) {
		t.Fatal("vault contains plaintext")
	}
	if w := authCall(a, "POST", "/api/auth/setup", map[string]string{"token": setupTestToken, "username": "admin", "password": authTestPassword}, nil); w.Code != 409 {
		t.Fatal("setup can overwrite")
	}
	if w := authCall(a, "POST", "/api/auth/logout", map[string]any{}, cookie); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := authCall(a, "GET", "/api/state", nil, cookie); w.Code != 401 {
		t.Fatal("logout did not revoke")
	}
}
func TestRestartPasswordRotationAndAllSessionRevocation(t *testing.T) {
	a, dir := newAuthTest(t)
	cookie := setupAuth(t, a)
	s, _ := a.store.Snapshot()
	_, e := a.store.Apply(s.Revision, "private", func(s *core.State) (string, error) {
		s.Projects[0].Name = "机密项目名称"
		return "机密审计摘要", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	b, e := NewAuthServer(dir, a.web, "")
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if b.SetupToken() != "" {
		t.Fatal("restart exposed setup token")
	}
	if w := authCall(b, "GET", "/api/state", nil, cookie); w.Code != 401 {
		t.Fatal("session survived restart")
	}
	w := authCall(b, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": authTestPassword}, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	one := w.Result().Cookies()[0]
	w = authCall(b, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": authTestPassword}, nil)
	two := w.Result().Cookies()[0]
	if w := authCall(b, "GET", "/api/state", nil, one); !strings.Contains(w.Body.String(), "机密项目名称") {
		t.Fatal("persisted state missing")
	}
	w = authCall(b, "POST", "/api/auth/password", map[string]string{"currentPassword": authTestPassword, "newPassword": "Changed-password-for-unit-tests!"}, one)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, c := range []*http.Cookie{one, two} {
		if authCall(b, "GET", "/api/state", nil, c).Code != 401 {
			t.Fatal("old session not invalidated")
		}
	}
	if authCall(b, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": authTestPassword}, nil).Code != 401 {
		t.Fatal("old password accepted")
	}
	if authCall(b, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": "Changed-password-for-unit-tests!"}, nil).Code != 200 {
		t.Fatal("new password rejected")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "app.vault"))
	for _, secret := range []string{"机密项目名称", "机密审计摘要", authTestPassword} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("sensitive data leaked")
		}
	}
}
func TestIdleExpirationPollingDoesNotKeepSessionAlive(t *testing.T) {
	a, _ := newAuthTest(t)
	now := time.Now()
	a.now = func() time.Time { return now }
	cookie := setupAuth(t, a)
	now = now.Add(20 * time.Minute)
	if authCall(a, "GET", "/api/auth/status", nil, cookie).Code != 200 {
		t.Fatal("status failed")
	}
	now = now.Add(11 * time.Minute)
	if authCall(a, "GET", "/api/state", nil, cookie).Code != 401 {
		t.Fatal("polling extended idle session")
	}
}
func TestLoginRateLimitAndCSRFTransport(t *testing.T) {
	a, _ := newAuthTest(t)
	setupAuth(t, a)
	for i := 0; i < 10; i++ {
		if w := authCall(a, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": "wrong-password"}, nil); w.Code != 401 {
			t.Fatal("unexpected failed login", w.Code)
		}
	}
	if authCall(a, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": authTestPassword}, nil).Code != 429 {
		t.Fatal("missing rate limit")
	}
	for _, origin := range []string{"https://evil.example", "https://example.com", "null"} {
		r := httptest.NewRequest("POST", "http://example.com/api/auth/login", strings.NewReader(`{}`))
		r.RemoteAddr = "127.0.0.1:123"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin mutation allowed")
		}
	}
	r := httptest.NewRequest("POST", "http://example.com/api/auth/login", strings.NewReader(`{}`))
	r.RemoteAddr = "192.168.1.20:4444"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("plain LAN/proxy spoof accepted")
	}
}
func TestHTTPSCookieAndPlaintextImport(t *testing.T) {
	a, dir := newAuthTest(t)
	plain, e := core.Open(filepath.Join(dir, "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	s, _ := plain.Snapshot()
	_, e = plain.Apply(s.Revision, "original", func(s *core.State) (string, error) {
		s.Projects[0].Name = "导入原有项目"
		return "原有审计", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	plain.Close()
	before, _ := os.ReadFile(filepath.Join(dir, "app.db"))
	body, _ := json.Marshal(map[string]any{"token": setupTestToken, "username": "admin", "password": authTestPassword, "importExisting": true})
	r := httptest.NewRequest("POST", "https://example.com/api/auth/setup", bytes.NewReader(body))
	r.RemoteAddr = "192.168.1.20:4444"
	r.TLS = &tls.ConnectionState{}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("HTTPS cookie not Secure")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "app.db"))
	if !bytes.Equal(before, after) {
		t.Fatal("source modified")
	}
	state, _ := a.store.Snapshot()
	if state.Projects[0].Name != "导入原有项目" {
		t.Fatal("import lost state")
	}
	audits, _ := a.store.Audits()
	if len(audits) != 2 {
		t.Fatal("import lost audits")
	}
}
func TestDirectoryLockAndWrongVaultFailClosed(t *testing.T) {
	a, dir := newAuthTest(t)
	if other, e := NewAuthServer(dir, a.web, setupTestToken); e == nil {
		other.Close()
		t.Fatal("duplicate server accepted")
	}
	setupAuth(t, a)
	a.Close()
	path := filepath.Join(dir, "app.vault")
	data, _ := os.ReadFile(path)
	data[len(data)-1] ^= 1
	os.WriteFile(path, data, 0600)
	b, e := NewAuthServer(dir, a.web, "")
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	if b.SetupToken() != "" {
		t.Fatal("corrupt vault can be initialized again")
	}
	if authCall(b, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": authTestPassword}, nil).Code != 401 {
		t.Fatal("corrupt vault opened")
	}
	if b.store != nil {
		t.Fatal("untrusted data store initialized")
	}
}

func TestCertificateCreationAndReuse(t *testing.T) {
	dir := t.TempDir()
	cert, key, e := EnsureLocalCertificate(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tls.LoadX509KeyPair(cert, key); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(key)
	cert2, key2, e := EnsureLocalCertificate(dir)
	if e != nil || cert2 != cert || key2 != key {
		t.Fatal("certificate not reused")
	}
	after, _ := os.ReadFile(key)
	if !bytes.Equal(before, after) {
		t.Fatal("key changed unexpectedly")
	}
	os.Remove(cert)
	if _, _, e = EnsureLocalCertificate(dir); e == nil {
		t.Fatal("partial certificate silently overwritten")
	}
}
