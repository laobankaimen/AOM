package core

import (
	"aom/internal/vault"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestVisualChangesPersistWithoutAuditsAndBusinessChangesAreCompact(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, _ := s.Snapshot()
	state, err = s.Apply(state.Revision, "module.position", func(next *State) (string, error) {
		rotation := 90
		return SetRoomTransform(next, next.Modules[0].ID, &CanvasPoint{X: 100, Y: 200}, &rotation)
	})
	if err != nil {
		t.Fatal(err)
	}
	audits, _ := s.Audits()
	if len(audits) != 1 {
		t.Fatal("diagram changed audit history")
	}
	if state.Modules[0].RoomPosition == nil || state.Modules[0].RoomRotation != 90 {
		t.Fatal("diagram edit not persisted")
	}
	state, err = s.Apply(state.Revision, "project.update", func(next *State) (string, error) { next.Projects[0].Name = "修改项目"; return "改名", nil })
	if err != nil {
		t.Fatal(err)
	}
	audits, _ = s.Audits()
	if len(audits) != 2 {
		t.Fatal("business edit missing")
	}
	entry, _ := s.Audit(audits[0].ID)
	var before, after map[string]json.RawMessage
	json.Unmarshal(entry.Before, &before)
	json.Unmarshal(entry.After, &after)
	if len(after["projects"]) == 0 || len(after["devices"]) > 0 || len(after["modules"]) > 0 {
		t.Fatalf("unrelated objects included: %s", entry.After)
	}
	if !bytes.Contains(entry.After, []byte("changed-records-v1")) {
		t.Fatal("missing format")
	}
	_, err = s.Apply(state.Revision, "project.update", func(*State) (string, error) { return "未修改", nil })
	if err != nil {
		t.Fatal(err)
	}
	nextAudits, _ := s.Audits()
	if len(nextAudits) != 2 {
		t.Fatal("unchanged save creates audit")
	}
}
func TestAuditDeltaRecordsCreateUpdateDeleteButNotVisualFields(t *testing.T) {
	before := networkFixture()
	after := Clone(before)
	after.Networks[0].Color = "#112233"
	after.Networks[0].DevicePositions = map[string]CanvasPoint{"server": {X: 100, Y: 200}}
	after.Devices[0].TopologyPosition = &CanvasPoint{X: 200, Y: 300}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	_, _, changed, err := auditDelta(a, b)
	if err != nil || changed {
		t.Fatal("visual-only delta", err)
	}
	after.Devices[0].IP = "10.0.0.8"
	after.Devices = after.Devices[:len(after.Devices)-1]
	after.Devices = append(after.Devices, Device{ID: "new", Name: "新增设备"})
	b, _ = json.Marshal(after)
	old, new, changed, err := auditDelta(a, b)
	if err != nil || !changed {
		t.Fatal(err)
	}
	var x, y State
	json.Unmarshal(old, &x)
	json.Unmarshal(new, &y)
	if len(x.Devices) != 2 || len(y.Devices) != 2 || x.Device("server") == nil || y.Device("server") != nil || y.Device("new") == nil || y.Device("core").IP != "10.0.0.8" {
		t.Fatal("create/update/delete not preserved")
	}
	if bytes.Contains(new, []byte("topologyPosition")) || bytes.Contains(new, []byte("networks")) {
		t.Fatal("visual/unrelated data retained")
	}
	full := len(a) + len(b)
	compact := len(old) + len(new)
	if compact >= full/3 {
		t.Fatalf("snapshot not reduced enough: %d -> %d", full, compact)
	}
	t.Logf("single edit sample: %d -> %d bytes", full, compact)
}
func legacyAudits(t *testing.T, s *Store) {
	t.Helper()
	before, _ := s.Snapshot()
	after := Clone(before)
	after.Modules[0].RoomPosition = &CanvasPoint{X: 100, Y: 200}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	for _, entry := range []struct {
		action   string
		old, new []byte
	}{{"module.position", a, b}, {"project.update", a, func() []byte { after.Projects[0].Name = "历史项目名称"; v, _ := json.Marshal(after); return v }()}, {"unknown-format", []byte(`{"unknown":true}`), []byte(`{"unknown":false}`)}} {
		_, err := s.db.Exec("INSERT INTO audits(time,action,summary,revision,before_json,after_json) VALUES('2026-10-30T00:00:00Z',?,'历史记录',1,?,?)", entry.action, string(entry.old), string(entry.new))
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestAuditCompactionPreservesBusinessHistoryIDsAndCurrentData(t *testing.T) {
	s, err := OpenMemory(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	legacyAudits(t, s)
	before, _ := s.Snapshot()
	entries, _ := s.Audits()
	oldBusiness, _ := s.Audit(entries[1].ID)
	unknown, _ := s.Audit(entries[0].ID)
	result, err := s.CompactAudits(before.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed != 1 || result.Compacted != 1 || result.BytesAfter >= result.BytesBefore/2 {
		t.Fatalf("unexpected compaction: %+v", result)
	}
	after, _ := s.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("compaction changed current state")
	}
	business, _ := s.Audit(oldBusiness.ID)
	if business.ID != oldBusiness.ID || business.Time != oldBusiness.Time || business.Action != oldBusiness.Action || business.Summary != oldBusiness.Summary {
		t.Fatal("audit metadata lost")
	}
	var old, new State
	json.Unmarshal(business.Before, &old)
	json.Unmarshal(business.After, &new)
	if len(old.Projects) != 1 || new.Projects[0].Name != "历史项目名称" {
		t.Fatal("business history lost")
	}
	preserved, _ := s.Audit(unknown.ID)
	if !reflect.DeepEqual(unknown, preserved) {
		t.Fatal("unknown audit modified")
	}
	second, err := s.CompactAudits(before.Revision)
	if err != nil || second.Removed != 0 || second.Compacted != 0 {
		t.Fatal("compaction not idempotent", err, second)
	}
	if _, err = s.CompactAudits(before.Revision - 1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale revision accepted")
	}
}
func TestAuditCompactionEncryptedFailureRollsBack(t *testing.T) {
	s, err := OpenMemory(nil, func([]byte) error { return errors.New("disk full") })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	legacyAudits(t, s)
	before, _ := s.ExportImage()
	state, _ := s.Snapshot()
	if _, err = s.CompactAudits(state.Revision); !errors.Is(err, ErrPersistence) {
		t.Fatal(err)
	}
	after, _ := s.ExportImage()
	if !bytes.Equal(before, after) {
		t.Fatal("failed persistence changed audits")
	}
}
func TestCompactionShrinksVaultAndReopensWithStateAndCriticalHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.vault")
	f := vault.New(path)
	defer f.Close()
	s, err := OpenMemory(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyAudits(t, s)
	before, _ := s.Snapshot()
	image, _ := s.ExportImage()
	if err = f.Create("admin", "audit-test-password-long", image); err != nil {
		t.Fatal(err)
	}
	sizeBefore, _ := os.Stat(path)
	s.Close()
	s, err = OpenMemory(image, f.Save)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.CompactAudits(before.Revision); err != nil {
		t.Fatal(err)
	}
	sizeAfter, _ := os.Stat(path)
	if sizeAfter.Size() >= sizeBefore.Size() {
		t.Fatal("encrypted disk file did not shrink")
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("历史项目名称")) {
		t.Fatal("audit plaintext leaked")
	}
	decrypted, err := f.Open("admin", "audit-test-password-long")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := OpenMemory(decrypted, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	state, _ := restored.Snapshot()
	if !reflect.DeepEqual(state, before) {
		t.Fatal("data changed on compaction")
	}
	entries, _ := restored.Audits()
	if len(entries) != 3 {
		t.Fatal("critical history lost")
	}
	t.Logf("vault size: %d -> %d bytes", sizeBefore.Size(), sizeAfter.Size())
}
