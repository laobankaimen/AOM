package core

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoomPositionChangesOnlyVisualLayout(t *testing.T) {
	s := networkFixture()
	s.Links = []Link{physicalLink(s)}
	before := Clone(s)
	p := &CanvasPoint{X: 350, Y: 240}
	if _, e := SetRoomPosition(&s, s.Modules[0].ID, p); e != nil {
		t.Fatal(e)
	}
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before.Devices, s.Devices) || !reflect.DeepEqual(before.Racks, s.Racks) || !reflect.DeepEqual(before.Links, s.Links) || !reflect.DeepEqual(before.Infra, s.Infra) {
		t.Fatal("visual move changed assets")
	}
	if !reflect.DeepEqual(s.Modules[0].RoomPosition, p) {
		t.Fatal("position not stored")
	}
	if _, e := SetRoomPosition(&s, s.Modules[0].ID, nil); e != nil {
		t.Fatal(e)
	}
	if s.Modules[0].RoomPosition != nil {
		t.Fatal("reset failed")
	}
}
func TestInvalidModuleCoordinates(t *testing.T) {
	for _, point := range []CanvasPoint{{X: -1}, {Y: 10001}, {X: math.NaN()}, {Y: math.Inf(1)}} {
		s := Seed()
		if _, e := SetRoomPosition(&s, s.Modules[0].ID, &point); e == nil {
			t.Fatal("invalid position accepted")
		}
		s.Modules[0].RoomPosition = &point
		if Validate(&s) == nil {
			t.Fatal("validation allowed invalid coordinates")
		}
	}
}
func TestInfraMoveWithinAndBetweenRowsPreservesMeaning(t *testing.T) {
	s := Seed()
	m := s.Modules[1]
	before := Clone(s)
	var id string
	for _, i := range s.Infra {
		if i.ModuleID == m.ID && i.Kind == "ac" && i.Side == "L" {
			id = i.ID
			break
		}
	}
	s.Infra = append(s.Infra, Infra{ID: "second-inline", ModuleID: m.ID, Name: "右列供电", Kind: "power", Side: "R", AfterRack: 3, Placement: "inline"})
	if _, e := MoveInlineInfrastructure(&s, id, "R", 3, "second-inline"); e != nil {
		t.Fatal(e)
	}
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	var moved, other Infra
	for _, i := range s.Infra {
		if i.ID == id {
			moved = i
		}
		if i.ID == "second-inline" {
			other = i
		}
	}
	if moved.Side != "R" || moved.AfterRack != 3 || moved.Order >= other.Order || moved.Kind != "ac" || moved.Placement != "inline" {
		t.Fatal("wrong move/order")
	}
	if !reflect.DeepEqual(before.Racks, s.Racks) || !reflect.DeepEqual(before.Devices, s.Devices) || !reflect.DeepEqual(before.Modules, s.Modules) {
		t.Fatal("facility move changed cabinets")
	}
	if _, e := MoveInlineInfrastructure(&s, "second-inline", "R", 3, id); e != nil {
		t.Fatal(e)
	}
	for _, i := range s.Infra {
		if i.ID == id {
			moved = i
		}
		if i.ID == "second-inline" {
			other = i
		}
	}
	if other.Order >= moved.Order {
		t.Fatal("same gap ordering not changed")
	}
	p, e := CaptureModulePlan(s, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, slot := range p.Layout {
		if slot.Name == moved.Name && slot.Side == "R" && slot.AfterRack == 3 && slot.Order == moved.Order {
			found = true
		}
	}
	if !found {
		t.Fatal("template lost facility position/order")
	}
}
func TestInvalidInfraMoveRollsBack(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	module := s.Modules[0]
	id, integrated := "", ""
	for _, i := range s.Infra {
		if i.ModuleID == module.ID {
			if i.Placement == "inline" {
				id = i.ID
			} else {
				integrated = i.ID
			}
		}
	}
	for _, tc := range []struct {
		id, side string
		after    int
		before   string
	}{{id, "R", 0, ""}, {id, "L", 99, ""}, {id, "L", 1, "missing"}, {integrated, "L", 1, ""}, {id, "L", 1, id}} {
		_, e := store.Apply(s.Revision, "infra.move", func(s *State) (string, error) {
			return MoveInlineInfrastructure(s, tc.id, tc.side, tc.after, tc.before)
		})
		if e == nil {
			t.Fatal("bad move accepted")
		}
		after, _ := store.Snapshot()
		if !reflect.DeepEqual(after, s) {
			t.Fatal("failed move committed")
		}
	}
	audits, _ := store.Audits()
	if len(audits) != 1 {
		t.Fatal("invalid moves created audits")
	}
}
func TestRoomAndInfrastructurePositionPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := store.Snapshot()
	moduleID := s.Modules[0].ID
	_, e = store.Apply(s.Revision, "module.position", func(s *State) (string, error) { return SetRoomPosition(s, moduleID, &CanvasPoint{X: 100, Y: 250}) })
	if e != nil {
		t.Fatal(e)
	}
	s, _ = store.Snapshot()
	var id string
	for _, i := range s.Infra {
		if i.ModuleID == moduleID && i.Placement == "inline" {
			id = i.ID
			break
		}
	}
	_, e = store.Apply(s.Revision, "infra.move", func(s *State) (string, error) { return MoveInlineInfrastructure(s, id, "L", 4, "") })
	if e != nil {
		t.Fatal(e)
	}
	before, _ := store.Snapshot()
	store.Close()
	store, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("positions lost on restart")
	}
	audits, _ := store.Audits()
	if len(audits) != 2 || audits[0].Action != "infra.move" {
		t.Fatal("physical facilities must be audited, diagram movement must not")
	}
}

func TestRoomRotationPreservesAssetsAndValidatesAngles(t *testing.T) {
	s := networkFixture()
	s.Links = []Link{physicalLink(s)}
	before := Clone(s)
	for _, angle := range []int{90, 180, 270, 0} {
		if _, e := SetRoomTransform(&s, s.Modules[0].ID, &CanvasPoint{X: 200.5, Y: 100.25}, &angle); e != nil {
			t.Fatal(e)
		}
		if e := Validate(&s); e != nil {
			t.Fatal(e)
		}
		if s.Modules[0].RoomRotation != angle {
			t.Fatal("rotation not stored")
		}
	}
	if !reflect.DeepEqual(before.Devices, s.Devices) || !reflect.DeepEqual(before.Racks, s.Racks) || !reflect.DeepEqual(before.Links, s.Links) {
		t.Fatal("rotation changed physical assets")
	}
	for _, bad := range []int{-90, 45, 360} {
		if _, e := SetRoomTransform(&s, s.Modules[0].ID, &CanvasPoint{}, &bad); e == nil {
			t.Fatal("invalid angle accepted")
		}
	}
}
