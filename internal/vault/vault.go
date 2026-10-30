// Package vault encrypts complete SQLite snapshots. Only authenticated ciphertext
// is written to disk; the data key exists in process memory while unlocked.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const iterations = 600000
const maxSize = 512 * 1024 * 1024

var magic = []byte("AOMVAULT1\n")
var ErrCredentials = errors.New("账号或密码不正确，或加密文件已损坏")

type header struct {
	Username   string `json:"username"`
	Salt       string `json:"salt"`
	Iterations int    `json:"iterations"`
}
type File struct {
	mu       sync.Mutex
	path     string
	key      []byte
	metadata header
}

func PasswordValid(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return errors.New("密码须为12–128字节，请使用足够长的独立密码")
	}
	return nil
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

func UsernameValid(name string) bool { return usernamePattern.MatchString(name) }
func New(path string) *File          { return &File{path: path} }
func (f *File) Exists() (bool, error) {
	_, e := os.Stat(f.path)
	if os.IsNotExist(e) {
		return false, nil
	}
	return e == nil, e
}
func (f *File) Close() { f.mu.Lock(); defer f.mu.Unlock(); clear(f.key); f.key = nil }
func derive(password string, h header) ([]byte, error) {
	salt, e := base64.StdEncoding.DecodeString(h.Salt)
	if e != nil || len(salt) != 32 || h.Iterations != iterations || !UsernameValid(h.Username) {
		return nil, ErrCredentials
	}
	return pbkdf2.Key(sha256.New, password, salt, iterations, 32)
}
func (f *File) Open(username, password string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	image, key, h, e := read(f.path, username, password)
	if e != nil {
		return nil, e
	}
	clear(f.key)
	f.key = key
	f.metadata = h
	return image, nil
}
func (f *File) Verify(username, password string) bool {
	image, key, _, e := read(f.path, username, password)
	clear(image)
	clear(key)
	return e == nil
}
func newHeader(username string) (header, error) {
	salt := make([]byte, 32)
	if _, e := rand.Read(salt); e != nil {
		return header{}, e
	}
	return header{Username: username, Salt: base64.StdEncoding.EncodeToString(salt), Iterations: iterations}, nil
}
func (f *File) Create(username, password string, image []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !UsernameValid(username) {
		return errors.New("账号须为3–64位字母、数字、点、短横线或下划线")
	}
	if e := PasswordValid(password); e != nil {
		return e
	}
	if _, e := os.Stat(f.path); e == nil {
		return errors.New("加密数据已存在，不能重新初始化")
	} else if !os.IsNotExist(e) {
		return e
	}
	h, e := newHeader(username)
	if e != nil {
		return e
	}
	key, e := derive(password, h)
	if e != nil {
		return e
	}
	if e = write(f.path, image, key, h); e != nil {
		clear(key)
		return e
	}
	f.key = key
	f.metadata = h
	return nil
}
func (f *File) Save(image []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.key) != 32 {
		return errors.New("数据库尚未解锁")
	}
	return write(f.path, image, f.key, f.metadata)
}
func (f *File) ChangePassword(username, oldPassword, newPassword string, image []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e := PasswordValid(newPassword); e != nil {
		return e
	}
	old, key, _, e := read(f.path, username, oldPassword)
	clear(old)
	clear(key)
	if e != nil {
		return ErrCredentials
	}
	h, e := newHeader(username)
	if e != nil {
		return e
	}
	nextKey, e := derive(newPassword, h)
	if e != nil {
		return e
	}
	if e = write(f.path, image, nextKey, h); e != nil {
		clear(nextKey)
		return e
	}
	clear(f.key)
	f.key = nextKey
	f.metadata = h
	return nil
}
func read(path, username, password string) ([]byte, []byte, header, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, header{}, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, nil, header{}, e
	}
	if st.Size() > maxSize || st.Size() < int64(len(magic)+4+12+16) {
		return nil, nil, header{}, ErrCredentials
	}
	prefix := make([]byte, len(magic)+4)
	if _, e = io.ReadFull(f, prefix); e != nil || !bytes.Equal(prefix[:len(magic)], magic) {
		return nil, nil, header{}, ErrCredentials
	}
	n := binary.BigEndian.Uint32(prefix[len(magic):])
	if n == 0 || n > 4096 {
		return nil, nil, header{}, ErrCredentials
	}
	raw := make([]byte, n)
	if _, e = io.ReadFull(f, raw); e != nil {
		return nil, nil, header{}, ErrCredentials
	}
	var h header
	if e = json.Unmarshal(raw, &h); e != nil {
		return nil, nil, h, ErrCredentials
	}
	key, e := derive(password, h)
	if e != nil {
		return nil, nil, h, ErrCredentials
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	data, e := io.ReadAll(io.LimitReader(f, maxSize))
	if e != nil || len(data) < gcm.NonceSize()+gcm.Overhead() {
		clear(key)
		return nil, nil, h, ErrCredentials
	}
	aad := append(prefix, raw...)
	image, e := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], aad)
	if e != nil || h.Username != username {
		clear(key)
		clear(image)
		return nil, nil, h, ErrCredentials
	}
	return image, key, h, nil
}
func write(path string, image, key []byte, h header) error {
	if len(image) > maxSize-8192 {
		return errors.New("加密数据超过512MiB上限，请整理审计和数据")
	}
	raw, e := json.Marshal(h)
	if e != nil {
		return e
	}
	aad := make([]byte, len(magic)+4+len(raw))
	copy(aad, magic)
	binary.BigEndian.PutUint32(aad[len(magic):], uint32(len(raw)))
	copy(aad[len(magic)+4:], raw)
	block, e := aes.NewCipher(key)
	if e != nil {
		return e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return e
	}
	ciphertext := gcm.Seal(nil, nonce, image, aad)
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	temp, e := os.CreateTemp(filepath.Dir(path), ".aom-encrypted-*")
	if e != nil {
		return e
	}
	name := temp.Name()
	defer os.Remove(name)
	defer temp.Close()
	if e = temp.Chmod(0600); e != nil {
		return e
	}
	for _, chunk := range [][]byte{aad, nonce, ciphertext} {
		if _, e = temp.Write(chunk); e != nil {
			return e
		}
	}
	if e = temp.Sync(); e != nil {
		return e
	}
	if e = temp.Close(); e != nil {
		return e
	}
	if e = replaceFile(name, path); e != nil {
		return fmt.Errorf("保存加密数据库失败：%w", e)
	}
	return nil
}
