package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testDevice(s State, id string, start int) Device {
	return Device{ID: id, AssetTag: id, Name: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, RackID: s.Racks[0].ID, StartU: start, Status: "installed"}
}
func TestSeedPlan(t *testing.T) {
	s := Seed()
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	if len(s.Racks) != 17 || len(s.Modules) != 3 || len(s.Sites) != 1 || len(s.Projects) != 1 || len(s.Devices) != 0 || len(s.Infra) != 13 {
		t.Fatalf("unexpected seed: %d racks, %d modules, %d devices", len(s.Racks), len(s.Modules), len(s.Devices))
	}
	if s.Model("model-core").UHeight != 0 {
		t.Fatal("core height must remain unconfirmed")
	}
}
func TestPlacementBoundariesAndOverlap(t *testing.T) {
	for _, tt := range []struct {
		name  string
		start int
		valid bool
	}{{"bottom", 1, true}, {"top fits", 41, true}, {"overflows", 42, false}, {"zero", 0, false}, {"negative", -1, false}} {
		t.Run(tt.name, func(t *testing.T) {
			s := Seed()
			s.Devices = append(s.Devices, testDevice(s, "device-1", tt.start))
			err := Validate(&s)
			if (err == nil) != tt.valid {
				t.Fatalf("start %d: %v", tt.start, err)
			}
		})
	}
	s := Seed()
	s.Devices = []Device{testDevice(s, "one", 1), testDevice(s, "two", 2)}
	if Validate(&s) == nil {
		t.Fatal("overlap accepted")
	}
	s.Devices[1].StartU = 3
	if err := Validate(&s); err != nil {
		t.Fatal(err)
	}
}
func TestUnknownHeightAndCrossModuleMove(t *testing.T) {
	s := Seed()
	d := testDevice(s, "one", 1)
	d.ModelID = "model-core"
	s.Devices = append(s.Devices, d)
	if Validate(&s) == nil {
		t.Fatal("unknown height accepted")
	}
	s.Devices[0].ModelID = "model-r722"
	target := s.Racks[len(s.Racks)-1]
	if e := Place(&s, d.ID, target.ID, 41); e != nil {
		t.Fatal(e)
	}
	if s.Devices[0].ModuleID != target.ModuleID {
		t.Fatal("module must follow rack")
	}
	if e := Place(&s, d.ID, "", 0); e != nil {
		t.Fatal(e)
	}
	if s.Devices[0].Status != "pending" || s.Devices[0].StartU != 0 {
		t.Fatal("unmount left stale placement")
	}
}
func TestAutoPlaceFragmentationAndCapacity(t *testing.T) {
	s := Seed()
	s.Devices = []Device{testDevice(s, "first", 2)}
	d := testDevice(s, "next", 0)
	d.RackID = ""
	reason := AutoPlace(&s, &d)
	if reason != "" || d.StartU != 4 {
		t.Fatalf("must skip 1U gap: %+v, %s", d, reason)
	}
	s = Seed()
	module := s.Modules[0].ID
	for _, rack := range s.Racks {
		if rack.ModuleID == module {
			for u := 1; u < 42; u += 2 {
				v := testDevice(s, fmt.Sprintf("%s-%d", rack.ID, u), u)
				v.RackID = rack.ID
				s.Devices = append(s.Devices, v)
			}
		}
	}
	d = testDevice(s, "overflow", 0)
	d.RackID = ""
	d.Status = "pending"
	if AutoPlace(&s, &d) == "" || d.RackID != "" {
		t.Fatal("capacity overflow borrowed another module")
	}
}
func TestCSVExpansionPreviewIsReadOnly(t *testing.T) {
	s := Seed()
	s.Revision = 10
	csv := "\ufeff型号,数量,名称,IP,SN,备注\nKunTai R722 K2,3,\"计算,服务器\",,,\"多行\n备注\"\nRZ18001-1,1,核心,,,待确认\n"
	p, e := PreviewCSV(s, s.Modules[0].ID, csv)
	if e != nil {
		t.Fatal(e)
	}
	if p.Total != 4 || p.Placed != 3 || p.Pending != 1 || p.Revision != 10 || len(s.Devices) != 0 {
		t.Fatalf("wrong preview: %+v", p)
	}
	for i := 0; i < 3; i++ {
		if p.Rows[i].Device.StartU != 1+i*2 {
			t.Fatal("not continuously allocated")
		}
	}
	if !strings.Contains(p.Rows[0].Device.Note, "\n") {
		t.Fatal("quoted multiline lost")
	}
}
func TestCSVInvalidInputs(t *testing.T) {
	for _, csv := range []string{
		"model,quantity\nKunTai R722 K2,-1", "model,quantity\nKunTai R722 K2,1.5", "model,quantity\nKunTai R722 K2,2001", "model,quantity\nunknown,1", "model,quantity,ip\nKunTai R722 K2,2,10.0.0.1", "model,quantity,sn\nKunTai R722 K2,2,SN1", "model,quantity\nKunTai R722 K2,1,extra", "model,name\nKunTai R722 K2,x", "model,quantity,quantity\nKunTai R722 K2,1,1", "model,quantity", "model,quantity,ip\nKunTai R722 K2,1,badip",
	} {
		t.Run(csv, func(t *testing.T) {
			s := Seed()
			if _, e := PreviewCSV(s, s.Modules[0].ID, csv); e == nil {
				t.Fatal("invalid CSV accepted")
			}
		})
	}
}
func TestCSVSerialUniquenessAndAssetSequence(t *testing.T) {
	s := Seed()
	csv := "model,quantity,sn,asset_prefix\nKunTai R722 K2,1,SN-A,NEW"
	p, e := PreviewCSV(s, s.Modules[0].ID, csv)
	if e != nil {
		t.Fatal(e)
	}
	s.Devices = append(s.Devices, p.Rows[0].Device)
	if _, e = PreviewCSV(s, s.Modules[0].ID, csv); e == nil {
		t.Fatal("duplicate SN accepted")
	}
	p, e = PreviewCSV(s, s.Modules[0].ID, "model,quantity,asset_prefix\nKunTai R722 K2,1,NEW")
	if e != nil {
		t.Fatal(e)
	}
	if p.Rows[0].Device.AssetTag != "NEW-0002" {
		t.Fatal("asset sequence collides")
	}
}
func TestModelChangesRevalidatePlacement(t *testing.T) {
	s := Seed()
	s.Devices = []Device{testDevice(s, "one", 41)}
	s.Model("model-r722").UHeight = 3
	if Validate(&s) == nil {
		t.Fatal("height increase overflow accepted")
	}
	s.Model("model-r722").UHeight = 0
	if Validate(&s) == nil {
		t.Fatal("occupied model changed to unknown")
	}
}
func TestPlanValidation(t *testing.T) {
	for _, p := range []Plan{{SiteName: "", Modules: []ModulePlan{{Name: "A", Type: "single", RackCount: 4, UHeight: 42}}}, {SiteName: "new", Modules: []ModulePlan{{Name: "A", Type: "double", RackLeft: 0, RackRight: 4, UHeight: 42}}}, {SiteName: "new", Modules: []ModulePlan{{Name: "A", Type: "single", RackCount: 4, UHeight: 48}}}, {SiteName: "new", Modules: []ModulePlan{{Name: "A", Type: "single", RackCount: 4, UHeight: 42}}, Infra: InfraPlan{Power: -1}}} {
		s := Seed()
		if _, e := AddPlan(&s, p); e == nil {
			t.Fatal("invalid plan accepted")
		}
	}
}
func TestExportCSVFormulaSafety(t *testing.T) {
	s := Seed()
	d := testDevice(s, "one", 1)
	d.Name = "=HYPERLINK(\"evil\")"
	d.Note = "  +123"
	s.Devices = append(s.Devices, d)
	out := ExportCSV(s)
	if !strings.Contains(out, "'=HYPERLINK") || !strings.Contains(out, "'  +123") {
		t.Fatal("spreadsheet formula not escaped")
	}
}
func TestStoreAtomicAuditPersistenceAndConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	s, e := store.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	initial := s.Revision
	next, e := store.Apply(initial, "test.create", func(s *State) (string, error) {
		s.Devices = append(s.Devices, testDevice(*s, "one", 1))
		return "create one", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if next.Revision != initial+1 {
		t.Fatal("revision not incremented")
	}
	if _, e = store.Apply(initial, "stale", func(s *State) (string, error) { return "stale", nil }); !errors.Is(e, ErrConflict) {
		t.Fatalf("expected conflict: %v", e)
	}
	if _, e = store.Apply(next.Revision, "invalid", func(s *State) (string, error) {
		s.Devices = append(s.Devices, testDevice(*s, "two", 1))
		return "overlap", nil
	}); e == nil {
		t.Fatal("invalid state committed")
	}
	audits, e := store.Audits()
	if e != nil {
		t.Fatal(e)
	}
	if len(audits) != 2 {
		t.Fatalf("invalid mutation created audit: %d", len(audits))
	}
	snapshot, e := store.Audit(audits[0].ID)
	if e != nil || len(snapshot.Before) == 0 || len(snapshot.After) == 0 {
		t.Fatalf("missing snapshots: %v", e)
	}
	if e = store.Close(); e != nil {
		t.Fatal(e)
	}
	store, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	persisted, e := store.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	if len(persisted.Devices) != 1 || persisted.Revision != next.Revision {
		t.Fatal("restart lost data or invalid write partially committed")
	}
}
func TestConcurrentWritersOnlyOneWins(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Apply(s.Revision, "test", func(s *State) (string, error) { return "concurrent", nil })
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for e := range results {
		if e == nil {
			wins++
		} else if errors.Is(e, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins %d conflicts %d", wins, conflicts)
	}
}
