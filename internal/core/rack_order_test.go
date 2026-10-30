package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestReorderPreservesCabinetContentsAndIntegratedFacilities(t *testing.T) {
	s := Seed()
	m := s.Modules[0]
	left := OrderedRacks(s, m.ID, "L")
	original := left[0]
	s.Devices = append(s.Devices, testDevice(s, "server-keep", 1))
	device := s.Devices[0]
	attached := ""
	for _, i := range s.Infra {
		if i.ModuleID == m.ID && i.Placement == "integrated" && i.AfterRack == 1 {
			attached = i.ID
		}
	}
	order := []string{}
	for _, r := range left[1:] {
		order = append(order, r.ID)
	}
	order = append(order, original.ID)
	if _, e := ReorderRacks(&s, m.ID, order, nil); e != nil {
		t.Fatal(e)
	}
	if s.Rack(original.ID).Name != original.Name || s.Rack(original.ID).Index != 5 || !reflect.DeepEqual(s.Devices[0], device) {
		t.Fatal("reorder changed identity or devices")
	}
	for _, i := range s.Infra {
		if i.ID == attached && i.AfterRack != 5 {
			t.Fatal("integrated facility did not follow cabinet")
		}
	}
}
func TestReorderAcrossRows(t *testing.T) {
	s := Seed()
	m := s.Modules[1]
	left, right := OrderedRacks(s, m.ID, "L"), OrderedRacks(s, m.ID, "R")
	lids, rids := []string{}, []string{left[0].ID}
	for _, r := range left[1:] {
		lids = append(lids, r.ID)
	}
	for _, r := range right {
		rids = append(rids, r.ID)
	}
	s.Infra = append(s.Infra, Infra{ID: "tail-power", ModuleID: m.ID, Kind: "power", Name: "列尾供电", Side: "L", AfterRack: 4, Placement: "inline"})
	if _, err := ReorderRacks(&s, m.ID, lids, rids); err != nil {
		t.Fatal(err)
	}
	if s.Modules[1].RackLeft != 3 || s.Modules[1].RackRight != 5 || s.Rack(left[0].ID).Side != "R" {
		t.Fatal("counts or positions stale")
	}
	for _, i := range s.Infra {
		if i.ID == "tail-power" && i.AfterRack != 3 {
			t.Fatal("tail facility invalid")
		}
	}
}
func TestBadReorderTransactionIsAtomic(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	m := s.Modules[0]
	valid := []string{}
	for _, r := range OrderedRacks(s, m.ID, "L") {
		valid = append(valid, r.ID)
	}
	for _, order := range [][]string{valid[:4], {valid[0], valid[0], valid[2], valid[3], valid[4]}, {"foreign", valid[1], valid[2], valid[3], valid[4]}} {
		_, e = store.Apply(s.Revision, "rack.reorder", func(next *State) (string, error) { return ReorderRacks(next, m.ID, order, nil) })
		if e == nil {
			t.Fatal("bad permutation accepted")
		}
		after, _ := store.Snapshot()
		if !reflect.DeepEqual(s, after) {
			t.Fatal("failed reorder changed state")
		}
	}
	after, e := store.Apply(s.Revision, "rack.reorder", func(next *State) (string, error) {
		return ReorderRacks(next, m.ID, []string{valid[1], valid[0], valid[2], valid[3], valid[4]}, nil)
	})
	if e != nil {
		t.Fatal(e)
	}
	audits, _ := store.Audits()
	if len(audits) != 2 || after.Revision != s.Revision+1 {
		t.Fatal("bad audit count")
	}
}
func TestCabinetPositionCollisionRejected(t *testing.T) {
	s := Seed()
	s.Racks[1].Index = s.Racks[0].Index
	if Validate(&s) == nil {
		t.Fatal("overlapping cabinets accepted")
	}
}

func TestInsertRackUsesFullMixedRowAndKeepsOtherObjectsInOrder(t *testing.T) {
	initial := Seed()
	module := initial.Modules[0]
	racks := OrderedRacks(initial, module.ID, "L")
	kept := []Infra{}
	for _, v := range initial.Infra {
		if v.ModuleID != module.ID {
			kept = append(kept, v)
		}
	}
	initial.Infra = append(kept, Infra{ID: "insert-power", ModuleID: module.ID, Name: "供电", Kind: "power", Side: "L", AfterRack: 2, Placement: "inline"}, Infra{ID: "insert-monitor", ModuleID: module.ID, Name: "监控", Kind: "env", Side: "L", AfterRack: 2, Order: 1, Placement: "inline"}, Infra{ID: "insert-ac", ModuleID: module.ID, Name: "空调", Kind: "ac", Side: "L", AfterRack: 4, Placement: "inline"}, Infra{ID: "attached", ModuleID: module.ID, Name: "柜内环境", Kind: "env", Side: "L", AfterRack: 1, Placement: "integrated"})
	initial.Devices = append(initial.Devices, testDevice(initial, "insert-device", 1))
	for _, rack := range racks {
		for _, before := range append(OrderedRowItems(initial, module.ID, "L"), "") {
			s := Clone(initial)
			old := OrderedRowItems(s, module.ID, "L")
			want := append([]string{}, old...)
			if before != rack.ID {
				want = RemoveID(want, rack.ID)
				at := len(want)
				for i, id := range want {
					if id == before {
						at = i
						break
					}
				}
				want = append(want, "")
				copy(want[at+1:], want[at:])
				want[at] = rack.ID
			}
			if _, err := InsertRack(&s, module.ID, rack.ID, "L", before); err != nil {
				t.Fatal(err)
			}
			got := OrderedRowItems(s, module.ID, "L")
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("insert %s before %s: want %v got %v", rack.Name, before, want, got)
			}
			if !reflect.DeepEqual(RemoveID(got, rack.ID), RemoveID(old, rack.ID)) {
				t.Fatal("nonmoving objects were reordered")
			}
			if !reflect.DeepEqual(s.Devices, initial.Devices) || !reflect.DeepEqual(s.Links, initial.Links) {
				t.Fatal("assets changed")
			}
			for _, i := range s.Infra {
				if i.ID == "attached" && (i.Side != s.Rack(racks[0].ID).Side || i.AfterRack != s.Rack(racks[0].ID).Index) {
					t.Fatal("integrated facility lost its rack")
				}
			}
			for _, r := range racks {
				if s.Rack(r.ID).Name != r.Name {
					t.Fatal("rack identity changed")
				}
			}
		}
	}
}
func TestInsertRackAcrossRowsKeepsFacilityOrderAndRejectsInvalidAnchor(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "insert.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	m := s.Modules[1]
	rack := OrderedRacks(s, m.ID, "L")[0]
	target := OrderedRowItems(s, m.ID, "R")[0]
	beforeL, beforeR := OrderedRowItems(s, m.ID, "L"), OrderedRowItems(s, m.ID, "R")
	next, e := store.Apply(s.Revision, "rack.reorder", func(v *State) (string, error) { return InsertRack(v, m.ID, rack.ID, "R", target) })
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(OrderedRowItems(next, m.ID, "L"), RemoveID(beforeL, rack.ID)) {
		t.Fatal("source row order changed")
	}
	if !reflect.DeepEqual(OrderedRowItems(next, m.ID, "R"), append([]string{rack.ID}, beforeR...)) {
		t.Fatal("target was swapped instead of inserted")
	}
	for _, invalid := range []struct{ rack, side, before string }{{rack.ID, "L", target}, {rack.ID, "X", ""}, {"missing", "R", ""}, {rack.ID, "R", "missing"}} {
		_, err := store.Apply(next.Revision, "rack.reorder", func(v *State) (string, error) { return InsertRack(v, m.ID, invalid.rack, invalid.side, invalid.before) })
		if err == nil {
			t.Fatal("invalid anchor accepted")
		}
		current, _ := store.Snapshot()
		if !reflect.DeepEqual(current, next) {
			t.Fatal("invalid insertion was partially persisted")
		}
	}
}
