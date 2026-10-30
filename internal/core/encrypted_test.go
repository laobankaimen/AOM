package core

import (
	"aom/internal/vault"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEncryptedStorePersistsAtomicSnapshotAndAudit(t *testing.T) {
	dir := t.TempDir()
	f := vault.New(filepath.Join(dir, "app.vault"))
	defer f.Close()
	s, e := OpenMemory(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	image, e := s.ExportImage()
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Create("admin", "long secret password", image); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = OpenMemory(image, f.Save)
	clear(image)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := s.Snapshot()
	after, e := s.Apply(before.Revision, "private.audit", func(s *State) (string, error) {
		d := testDevice(*s, "encrypted-device", 1)
		d.SN = "SECRET-SERIAL-9384"
		s.Devices = append(s.Devices, d)
		return "sensitive audit", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	raw, _ := os.ReadFile(filepath.Join(dir, "app.vault"))
	for _, secret := range []string{"SQLite format", "SECRET-SERIAL-9384", "sensitive audit", "encrypted-device"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatal("plaintext in vault")
		}
	}
	image, e = f.Open("admin", "long secret password")
	if e != nil {
		t.Fatal(e)
	}
	s, e = OpenMemory(image, f.Save)
	clear(image)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	restored, e := s.Snapshot()
	if e != nil || !reflect.DeepEqual(after, restored) {
		t.Fatal("snapshot serialization lost transaction", e)
	}
	audits, e := s.Audits()
	if e != nil || len(audits) != 2 {
		t.Fatal("audit not in encrypted snapshot", e)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("plaintext database files created")
	}
}
func TestEncryptedSaveFailureRollsBackStateAndAudit(t *testing.T) {
	s, e := OpenMemory(nil, func([]byte) error { return errors.New("disk full") })
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	before, _ := s.Snapshot()
	_, e = s.Apply(before.Revision, "failed", func(s *State) (string, error) {
		s.Devices = append(s.Devices, testDevice(*s, "not-committed", 1))
		return "fail", nil
	})
	if !errors.Is(e, ErrPersistence) {
		t.Fatal("persistence error missing", e)
	}
	after, _ := s.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unpersisted write stayed in memory")
	}
	audits, _ := s.Audits()
	if len(audits) != 1 {
		t.Fatal("failed audit persisted")
	}
}

func TestEncryptedInvalidRestoreFailsWithoutHanging(t *testing.T) {
	s, e := OpenMemory([]byte(`{"format":"aom-encrypted-state-1","state":{},"audits":[]}`), nil)
	if e == nil {
		s.Close()
		t.Fatal("invalid image accepted")
	}
}
