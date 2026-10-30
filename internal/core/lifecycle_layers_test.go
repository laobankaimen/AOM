package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestRackAndModuleLifecycleGuardsAndStableMixedOrder(t *testing.T) {
	s := Seed()
	m := s.Modules[0]
	before := Clone(s)
	if _, e := AddRack(&s, m.ID, "L", "扩容柜"); e != nil {
		t.Fatal(e)
	}
	added := OrderedRacks(s, m.ID, "L")
	last := added[len(added)-1]
	if last.Name != "扩容柜" || s.Modules[0].RackLeft != 6 {
		t.Fatal("rack not added")
	}
	if _, e := DeleteRack(&s, last.ID); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.Racks, before.Racks) {
		t.Fatal("existing racks changed")
	}
	r := OrderedRacks(s, m.ID, "L")[0]
	if _, e := DeleteRack(&s, r.ID); e == nil {
		t.Fatal("integrated facility rack deleted")
	}
	s.Devices = append(s.Devices, testDevice(s, "occupied", 1))
	if _, e := DeleteModule(&s, m.ID); e == nil {
		t.Fatal("occupied module deleted")
	}
	if _, e := DeleteRack(&s, r.ID); e == nil {
		t.Fatal("occupied rack deleted")
	}
	s.Devices = nil
	if _, e := DeleteModule(&s, m.ID); e != nil {
		t.Fatal(e)
	}
	for _, r := range s.Racks {
		if r.ModuleID == m.ID {
			t.Fatal("rack orphaned")
		}
	}
	for _, i := range s.Infra {
		if i.ModuleID == m.ID {
			t.Fatal("facility orphaned")
		}
	}
	if len(s.Sites) != len(before.Sites) {
		t.Fatal("room deleted")
	}
}
func TestRackRemovalPreservesAdjacentFacilityOrder(t *testing.T) {
	s := Seed()
	m := s.Modules[0]
	racks := OrderedRacks(s, m.ID, "L")
	s.Infra = append(s.Infra, Infra{ID: "facility-before", ModuleID: m.ID, Side: "L", Kind: "power", Name: "前供电", Placement: "inline", AfterRack: 2, Order: 9}, Infra{ID: "facility-after", ModuleID: m.ID, Side: "L", Kind: "ac", Name: "后空调", Placement: "inline", AfterRack: 3, Order: 0})
	order := OrderedRowItems(s, m.ID, "L")
	if _, e := DeleteRack(&s, racks[2].ID); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(OrderedRowItems(s, m.ID, "L"), RemoveID(order, racks[2].ID)) {
		t.Fatal("facility order changed")
	}
}
func TestLifecycleTransactionsRejectOccupiedAndKeepState(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	s, e = store.Apply(s.Revision, "fixture", func(s *State) (string, error) {
		s.Devices = append(s.Devices, testDevice(*s, "kept", 1))
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	_, e = store.Apply(s.Revision, "module.delete", func(v *State) (string, error) { return DeleteModule(v, v.Modules[0].ID) })
	if e == nil {
		t.Fatal("occupied deleted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(s, after) {
		t.Fatal("failed deletion changed data")
	}
}
func TestCustomNetworkLayersValidationAndMemberMoveCleanup(t *testing.T) {
	s := networkFixture()
	n := &s.Networks[0]
	n.Layers = []NetworkLayer{{"edge", "边界"}, {"app", "应用"}}
	n.DeviceLayers = map[string]string{"server": "app"}
	if e := Validate(&s); e != nil {
		t.Fatal(e)
	}
	n.DeviceLayers["missing"] = "app"
	if Validate(&s) == nil {
		t.Fatal("unknown device accepted")
	}
	delete(n.DeviceLayers, "missing")
	n.DeviceLayers["server"] = "missing"
	if Validate(&s) == nil {
		t.Fatal("unknown layer accepted")
	}
	n.DeviceLayers["server"] = "app"
	s.Networks = append(s.Networks, Network{ID: "other", ProjectID: s.Projects[0].ID, Name: "其他网络", Purpose: "business", Color: "#497cda", DeviceIDs: []string{}})
	if _, e := MoveNetworkMember(&s, "server", n.ID, "other", "move"); e != nil {
		t.Fatal(e)
	}
	if _, ok := s.Network("business").DeviceLayers["server"]; ok {
		t.Fatal("old layer assignment retained")
	}
}
