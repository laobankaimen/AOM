package core

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func networkFixture() State {
	s := Seed()
	m := s.Modules[0].ID
	for _, entry := range []struct{ id, model string }{{"core", "model-core"}, {"access", "model-25g"}, {"server", "model-r722"}} {
		s.Devices = append(s.Devices, Device{ID: entry.id, Name: entry.id, AssetTag: entry.id, ModelID: entry.model, ModuleID: m, Status: "pending"})
	}
	s.Networks = []Network{{ID: "business", ProjectID: s.Projects[0].ID, Name: "业务网", Purpose: "business", VLAN: 100, CIDR: "10.0.0.0/24", Gateway: "10.0.0.1", Color: "#497cda", DeviceIDs: []string{"core", "access", "server"}}}
	return s
}
func physicalLink(s State) Link {
	return Link{ID: "link-1", ProjectID: s.Projects[0].ID, SourceID: "access", TargetID: "server", SourcePort: "Ethernet1/1", TargetPort: "eth0", NetworkIDs: []string{"business"}, Kind: "physical", Status: "confirmed", Origin: "manual", Medium: "光纤", SpeedGbps: 25}
}
func TestNetworksAndPhysicalLinks(t *testing.T) {
	s := networkFixture()
	s.Links = []Link{physicalLink(s)}
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		change func(*State)
	}{
		{"VLAN too high", func(s *State) { s.Networks[0].VLAN = 4095 }},
		{"negative VLAN", func(s *State) { s.Networks[0].VLAN = -1 }},
		{"invalid CIDR", func(s *State) { s.Networks[0].CIDR = "10.0.0.3/24" }},
		{"outside gateway", func(s *State) { s.Networks[0].Gateway = "10.1.0.1" }},
		{"no subnet for gateway", func(s *State) { s.Networks[0].CIDR = "" }},
		{"duplicate member", func(s *State) { s.Networks[0].DeviceIDs = append(s.Networks[0].DeviceIDs, "server") }},
		{"unknown member", func(s *State) { s.Networks[0].DeviceIDs = append(s.Networks[0].DeviceIDs, "missing") }},
		{"self link", func(s *State) { s.Links[0].TargetID = "access" }},
		{"unknown endpoint", func(s *State) { s.Links[0].TargetID = "missing" }},
		{"unconfirmed port", func(s *State) { s.Links[0].TargetPort = "" }},
		{"negative rate", func(s *State) { s.Links[0].SpeedGbps = -1 }},
		{"bad network membership", func(s *State) { s.Networks[0].DeviceIDs = []string{"core", "access"} }},
		{"missing network", func(s *State) { s.Links[0].NetworkIDs = []string{"missing"} }},
		{"duplicate network", func(s *State) { s.Links[0].NetworkIDs = []string{"business", "business"} }},
		{"duplicate port", func(s *State) {
			l := s.Links[0]
			l.ID = "other-link"
			l.TargetID = "core"
			l.TargetPort = "p2"
			l.SourcePort = " ethernet1/1 "
			l.Status = "pending"
			s.Links = append(s.Links, l)
		}},
		{"duplicate global ID", func(s *State) { s.Networks[0].ID = "core" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := Clone(s)
			tc.change(&copy)
			if Validate(&copy) == nil {
				t.Fatal("invalid network accepted")
			}
		})
	}
}
func TestMultinetworkAndParallelPhysicalLinks(t *testing.T) {
	s := networkFixture()
	n := s.Networks[0]
	n.ID = "management"
	n.Name = "管理网"
	n.Purpose = "management"
	n.CIDR = "2001:db8::/64"
	n.Gateway = "2001:db8::1"
	s.Networks = append(s.Networks, n)
	l := physicalLink(s)
	l.NetworkIDs = append(l.NetworkIDs, n.ID)
	s.Links = append(s.Links, l)
	l.ID = "second-link"
	l.SourcePort = "Ethernet1/2"
	l.TargetPort = "eth1"
	s.Links = append(s.Links, l)
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
}
func TestCrossProjectNetworkAndMoveRejected(t *testing.T) {
	s := networkFixture()
	s.Links = []Link{physicalLink(s)}
	_, err := AddPlan(&s, Plan{ProjectName: "other", SiteName: "other room", Modules: []ModulePlan{{Name: "m", Type: "single", RackCount: 1, UHeight: 42}}})
	if err != nil {
		t.Fatal(err)
	}
	other := s.Modules[len(s.Modules)-1]
	s.Device("server").ModuleID = other.ID
	if Validate(&s) == nil {
		t.Fatal("cross project member/link allowed")
	}
	s.Networks[0].DeviceIDs = []string{"core", "access"}
	s.Links[0].NetworkIDs = nil
	if Validate(&s) == nil {
		t.Fatal("unassigned cross project link allowed")
	}
}
func TestInferenceReadOnlyPendingAndIdempotent(t *testing.T) {
	s := networkFixture()
	original := Clone(s)
	links, e := SuggestLinks(s, "business")
	if e != nil {
		t.Fatal(e)
	}
	if len(links) != 2 {
		t.Fatalf("expected core-access-server, got %d", len(links))
	}
	if !reflect.DeepEqual(s, original) {
		t.Fatal("preview mutated state")
	}
	for _, l := range links {
		if l.Kind != "logical" || l.Status != "pending" || l.Origin != "inferred" || l.SourcePort != "" || l.TargetPort != "" || l.SpeedGbps != 0 {
			t.Fatal("inference fabricated facts")
		}
	}
	s.Links = append(s.Links, links...)
	if e = Validate(&s); e != nil {
		t.Fatal(e)
	}
	again, e := SuggestLinks(s, "business")
	if e != nil || len(again) != 0 {
		t.Fatal("inference duplicated existing links")
	}
	s.Links = nil
	s.Networks[0].DeviceIDs = []string{"server"}
	links, e = SuggestLinks(s, "business")
	if e != nil || len(links) != 0 {
		t.Fatal("inference invented upstream")
	}
}
func TestNetworkPersistenceAndV2Migration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	seed, _ := store.Snapshot()
	s := networkFixture()
	s.Revision = seed.Revision
	s.Links = []Link{physicalLink(s)}
	_, e = store.Apply(seed.Revision, "network.test", func(next *State) (string, error) { *next = s; return "network persistence", nil })
	if e != nil {
		t.Fatal(e)
	}
	store.Close()
	store, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(after.Links, s.Links) || !reflect.DeepEqual(after.Networks, s.Networks) {
		t.Fatal("network data lost on restart")
	}
	store.Close()
	// A true v2 file has neither network table. Migration must preserve the room tree.
	legacy := filepath.Join(t.TempDir(), "app.db")
	store, e = Open(legacy)
	if e != nil {
		t.Fatal(e)
	}
	old, _ := store.Snapshot()
	old.SchemaVersion = 2
	tx, _ := store.db.Begin()
	if e = saveState(tx, old); e != nil {
		t.Fatal(e)
	}
	tx.Commit()
	store.db.Exec("DROP TABLE links")
	store.db.Exec("DROP TABLE networks")
	store.Close()
	store, e = Open(legacy)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	migrated, _ := store.Snapshot()
	if migrated.SchemaVersion != 5 || migrated.Revision != old.Revision+1 {
		t.Fatal("migration metadata invalid")
	}
	if !reflect.DeepEqual(old.Sites, migrated.Sites) || !reflect.DeepEqual(old.Modules, migrated.Modules) {
		t.Fatal("v2 topology migration changed layout")
	}
	if migrated.Networks == nil || migrated.Links == nil {
		t.Fatal("empty arrays missing")
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(legacy), "app-before-v5-*.db"))
	if len(backups) != 1 {
		t.Fatal("missing backup")
	}
	db, _ := sql.Open("sqlite", backups[0])
	defer db.Close()
	var v int
	db.QueryRow("SELECT schema_version FROM meta").Scan(&v)
	if v != 2 {
		t.Fatal("incorrect backup version")
	}
}
func TestReferenceDeletionIsAtomic(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	initial, _ := store.Snapshot()
	s := networkFixture()
	s.Revision = initial.Revision
	s.Links = []Link{physicalLink(s)}
	state, e := store.Apply(s.Revision, "seed", func(next *State) (string, error) { *next = s; return "seed", nil })
	if e != nil {
		t.Fatal(e)
	}
	_, e = store.Apply(state.Revision, "delete", func(next *State) (string, error) { next.Networks = nil; return "delete network", nil })
	if e == nil || !strings.Contains(e.Error(), "网络归属") {
		t.Fatalf("referenced network deleted: %v", e)
	}
	after, _ := store.Snapshot()
	if after.Revision != state.Revision || len(after.Networks) != 1 {
		t.Fatal("partial reference deletion committed")
	}
}

func TestNetworkValidationExplainsTheSpecificField(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*State)
	}{
		{"duplicate", "已存在网络「业务网」", func(s *State) { n := s.Networks[0]; n.ID = "other-network"; s.Networks = append(s.Networks, n) }},
		{"empty", "请填写网络名称", func(s *State) { s.Networks[0].Name = "  " }},
		{"project", "所属项目不存在", func(s *State) { s.Networks[0].ProjectID = "unknown" }},
		{"id", "内部编号为空或重复", func(s *State) { s.Networks[0].ID = "core" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := networkFixture()
			tc.change(&s)
			err := Validate(&s)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %s, got %v", tc.want, err)
			}
		})
	}
}
