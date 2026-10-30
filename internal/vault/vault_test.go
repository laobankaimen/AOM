package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultEncryptsAuthenticatesAndChangesPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.vault")
	f := New(path)
	defer f.Close()
	plain := []byte("IP=10.9.8.7 SN=secret-serial audit=private")
	if e := f.Create("admin", "a sufficiently long password", plain); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(data, plain) || bytes.Contains(data, []byte("10.9.8.7")) {
		t.Fatal("plaintext leaked")
	}
	if _, e = f.Open("admin", "wrong-password"); e == nil {
		t.Fatal("wrong password accepted")
	}
	if _, e = f.Open("someone", "a sufficiently long password"); e == nil {
		t.Fatal("wrong username accepted")
	}
	image, e := f.Open("admin", "a sufficiently long password")
	if e != nil || !bytes.Equal(image, plain) {
		t.Fatal("decrypt failed", e)
	}
	clear(image)
	if e = f.ChangePassword("admin", "a sufficiently long password", "a different long password", plain); e != nil {
		t.Fatal(e)
	}
	if f.Verify("admin", "a sufficiently long password") {
		t.Fatal("old password still works")
	}
	if !f.Verify("admin", "a different long password") {
		t.Fatal("new password does not work")
	}
	data, _ = os.ReadFile(path)
	data[len(data)-1] ^= 1
	os.WriteFile(path, data, 0600)
	if _, e = f.Open("admin", "a different long password"); e == nil {
		t.Fatal("tampered data accepted")
	}
}
func TestVaultFreshNoncesAndNoPlainTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.vault")
	f := New(path)
	defer f.Close()
	if e := f.Create("admin", "long private phrase", []byte("private")); e != nil {
		t.Fatal(e)
	}
	a, _ := os.ReadFile(path)
	if e := f.Save([]byte("private")); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	if bytes.Equal(a, b) {
		t.Fatal("nonce reused")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("temporary files left behind")
	}
	if e := f.Create("admin", "long private phrase", []byte("replace")); e == nil {
		t.Fatal("existing vault overwritten")
	}
}
