package core

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func treeFixture() State {
	s := networkFixture()
	root := s.Networks[0]
	root.ID = "root"
	root.Name = "根网络"
	root.DeviceIDs = []string{}
	s.Networks[0].ParentID = root.ID
	s.Networks = append(s.Networks, root)
	other := s.Networks[0]
	other.ID = "other"
	other.Name = "另一子网"
	other.DeviceIDs = []string{"core"}
	s.Networks = append(s.Networks, other)
	return s
}
func TestNetworkHierarchyConstraints(t *testing.T) {
	s := treeFixture()
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		change func(*State)
	}{{"self", func(s *State) { s.Network("root").ParentID = "root" }}, {"descendant", func(s *State) { s.Network("root").ParentID = "business" }}, {"missing", func(s *State) { s.Network("business").ParentID = "unknown" }}, {"outside project", func(s *State) {
		s.Projects = append(s.Projects, Project{ID: "p2", Name: "另一项目"})
		s.Network("root").ProjectID = "p2"
	}}, {"bad coordinate", func(s *State) {
		s.Network("business").DevicePositions = map[string]CanvasPoint{"server": {X: -1, Y: 3}}
	}}, {"unrelated coordinate", func(s *State) {
		s.Network("business").DevicePositions = map[string]CanvasPoint{"unknown": {X: 4, Y: 3}}
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			c := Clone(s)
			tc.change(&c)
			if Validate(&c) == nil {
				t.Fatal("invalid network hierarchy accepted")
			}
		})
	}
	if ValidCanvasPoint(CanvasPoint{X: math.NaN()}) || ValidCanvasPoint(CanvasPoint{Y: math.Inf(1)}) {
		t.Fatal("non-finite position accepted")
	}
}
func TestMemberMoveKeepsAssetsAndPhysicalLinks(t *testing.T) {
	s := treeFixture()
	s.Links = []Link{physicalLink(s)}
	s.Network("business").DevicePositions = map[string]CanvasPoint{"server": {X: 40, Y: 80}}
	beforeDevices := append([]Device{}, s.Devices...)
	beforeLink := s.Links[0]
	_, e := MoveNetworkMember(&s, "server", "business", "other", "move")
	if e != nil {
		t.Fatal(e)
	}
	if Contains(s.Network("business").DeviceIDs, "server") || !Contains(s.Network("other").DeviceIDs, "server") {
		t.Fatal("wrong membership")
	}
	if !reflect.DeepEqual(beforeDevices, s.Devices) {
		t.Fatal("assets changed")
	}
	after := s.Links[0]
	if after.ID != beforeLink.ID || after.SourcePort != beforeLink.SourcePort || after.TargetPort != beforeLink.TargetPort || after.Status != beforeLink.Status || len(after.NetworkIDs) != 0 {
		t.Fatal("physical connection mutated or obsolete tag remained")
	}
	if len(s.Network("business").DevicePositions) != 0 {
		t.Fatal("position not cleaned")
	}
	_, e = MoveNetworkMember(&s, "server", "", "business", "add")
	if e != nil {
		t.Fatal(e)
	}
	if !Contains(s.Network("other").DeviceIDs, "server") {
		t.Fatal("add removed other membership")
	}
	_, e = MoveNetworkMember(&s, "server", "other", "", "remove")
	if e != nil {
		t.Fatal(e)
	}
	if !Contains(s.Network("business").DeviceIDs, "server") || s.Device("server") == nil {
		t.Fatal("remove deleted unrelated membership or device")
	}
}
func TestInvalidMemberMoveRollbackAndAudit(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	base, _ := store.Snapshot()
	s := treeFixture()
	s.Revision = base.Revision
	next, e := store.Apply(base.Revision, "fixture", func(v *State) (string, error) { *v = s; return "fixture", nil })
	if e != nil {
		t.Fatal(e)
	}
	_, e = store.Apply(next.Revision, "move", func(v *State) (string, error) { return MoveNetworkMember(v, "server", "business", "missing", "move") })
	if e == nil {
		t.Fatal("invalid target accepted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(after, next) {
		t.Fatal("failed move committed")
	}
	result, e := store.Apply(next.Revision, "move", func(v *State) (string, error) { return MoveNetworkMember(v, "server", "business", "other", "move") })
	if e != nil {
		t.Fatal(e)
	}
	if result.Revision != next.Revision+1 {
		t.Fatal("missing revision")
	}
	audits, _ := store.Audits()
	if len(audits) != 3 {
		t.Fatal("wrong audit count")
	}
}
func TestV3MigrationPreservesNetworkFacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	base, _ := store.Snapshot()
	s := networkFixture()
	s.SchemaVersion = 3
	s.Revision = base.Revision
	s.Links = []Link{physicalLink(s)}
	tx, _ := store.db.Begin()
	if e = saveState(tx, s); e != nil {
		t.Fatal(e)
	}
	tx.Commit()
	store.Close()
	store, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	after, _ := store.Snapshot()
	if after.SchemaVersion != 5 || !reflect.DeepEqual(after.Networks, s.Networks) || !reflect.DeepEqual(after.Links, s.Links) || !reflect.DeepEqual(after.Devices, s.Devices) {
		t.Fatal("v3 migration lost assets or network facts")
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "app-before-v5-*.db"))
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
}
