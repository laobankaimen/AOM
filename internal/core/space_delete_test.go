package core

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func deletionFixture() State {
	s := Seed()
	source := s.Modules[0].ID
	target := s.Modules[1].ID
	s.Devices = append(s.Devices, Device{ID: "installed", Name: "已上架", AssetTag: "installed", ModelID: "model-r722", ModuleID: source, RackID: OrderedRacks(s, source, "L")[0].ID, StartU: 1, Status: "installed"}, Device{ID: "pending", Name: "待上架", AssetTag: "pending", ModelID: "model-r722", ModuleID: source, Status: "pending"}, Device{ID: "outside", Name: "外部设备", AssetTag: "outside", ModelID: "model-r722", ModuleID: target, Status: "pending"})
	s.Networks = []Network{{ID: "network", ProjectID: s.Projects[0].ID, Name: "业务网", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"installed", "pending", "outside"}, DevicePositions: map[string]CanvasPoint{"installed": {X: 1, Y: 2}, "pending": {X: 3, Y: 4}}, DeviceLayers: map[string]string{"installed": "compute", "pending": "compute"}}}
	s.Links = []Link{{ID: "inside-link", ProjectID: s.Projects[0].ID, SourceID: "installed", TargetID: "pending", Kind: "logical", Status: "pending", Origin: "manual", NetworkIDs: []string{"network"}}, {ID: "cross-link", ProjectID: s.Projects[0].ID, SourceID: "installed", TargetID: "outside", Kind: "physical", SourcePort: "eth1", TargetPort: "eth2", Status: "confirmed", Origin: "manual", NetworkIDs: []string{"network"}}}
	return s
}
func TestModuleDeleteMigrateRetainsAssetsConnectionsAndPendingDevices(t *testing.T) {
	s := deletionFixture()
	before := Clone(s)
	source, target := s.Modules[0], s.Modules[1]
	if _, err := DeleteModuleWithContents(&s, source.ID, SpaceDeleteOptions{Mode: "migrate", TargetModuleID: target.ID}); err != nil {
		t.Fatal(err)
	}
	if moduleByID(&s, source.ID) != nil {
		t.Fatal("module retained")
	}
	if len(s.Devices) != 3 || !reflect.DeepEqual(before.Links, s.Links) || !reflect.DeepEqual(before.Networks, s.Networks) {
		t.Fatal("migration lost assets or relationships")
	}
	for _, id := range []string{"installed", "pending"} {
		d := s.Device(id)
		if d.ModuleID != target.ID || d.RackID != "" || d.StartU != 0 || d.Status != "pending" {
			t.Fatal("device not migrated to pending")
		}
	}
	if !reflect.DeepEqual(before.Device("outside"), s.Device("outside")) {
		t.Fatal("outside asset changed")
	}
}
func TestModuleCascadeDeletesOnlyScopeAndCleansNetworkReferences(t *testing.T) {
	s := deletionFixture()
	before := Clone(s)
	source := s.Modules[0]
	if _, err := DeleteModuleWithContents(&s, source.ID, SpaceDeleteOptions{Mode: "cascade", ConfirmName: source.Name}); err != nil {
		t.Fatal(err)
	}
	if len(s.Devices) != 1 || s.Devices[0].ID != "outside" || len(s.Links) != 0 {
		t.Fatal("wrong cascade scope")
	}
	if len(s.Networks) != 1 || !reflect.DeepEqual(s.Networks[0].DeviceIDs, []string{"outside"}) || len(s.Networks[0].DevicePositions) != 0 || len(s.Networks[0].DeviceLayers) != 0 {
		t.Fatal("network references not cleaned")
	}
	if !reflect.DeepEqual(s.Device("outside"), before.Device("outside")) || !reflect.DeepEqual(s.Models, before.Models) || !reflect.DeepEqual(s.Templates, before.Templates) {
		t.Fatal("unrelated objects changed")
	}
}
func TestRackRemovalKeepsDevicesAndRemovesOnlyAttachedMarkers(t *testing.T) {
	s := deletionFixture()
	before := Clone(s)
	id := s.Device("installed").RackID
	if _, err := DeleteRackWithContents(&s, id, SpaceDeleteOptions{Mode: "uninstall", RemoveFacilities: true}); err != nil {
		t.Fatal(err)
	}
	if s.Rack(id) != nil || s.Device("installed").RackID != "" || s.Device("installed").ModuleID != before.Device("installed").ModuleID {
		t.Fatal("rack or device not updated")
	}
	if !reflect.DeepEqual(s.Links, before.Links) || !reflect.DeepEqual(s.Networks, before.Networks) || !reflect.DeepEqual(s.Device("pending"), before.Device("pending")) {
		t.Fatal("related data lost")
	}
}
func TestDeleteLastRackInDoubleRowConvertsToSingleAndPreservesOtherRow(t *testing.T) {
	s := Seed()
	m := s.Modules[1]
	for len(OrderedRacks(s, m.ID, "L")) > 1 {
		row := OrderedRacks(s, m.ID, "L")
		if _, e := DeleteRackWithContents(&s, row[len(row)-1].ID, SpaceDeleteOptions{Mode: "uninstall", RemoveFacilities: true}); e != nil {
			t.Fatal(e)
		}
	}
	right := OrderedRacks(s, m.ID, "R")
	id := OrderedRacks(s, m.ID, "L")[0].ID
	if _, e := DeleteRackWithContents(&s, id, SpaceDeleteOptions{Mode: "uninstall", RemoveFacilities: true}); e != nil {
		t.Fatal(e)
	}
	module := moduleByID(&s, m.ID)
	if module.Type != "single" || module.RackRight != 0 || module.RackLeft != len(right) {
		t.Fatal("double row not normalized")
	}
	for _, r := range right {
		if s.Rack(r.ID).Side != "L" || s.Rack(r.ID).Name != r.Name {
			t.Fatal("surviving rack identity changed")
		}
	}
}
func TestDestructiveDeleteRequiresExplicitConfirmationAndAtomicPersistence(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	old, _ := store.Snapshot()
	s, e := store.Apply(old.Revision, "fixture", func(v *State) (string, error) {
		fixture := deletionFixture()
		fixture.Revision = v.Revision
		*v = fixture
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	for _, option := range []SpaceDeleteOptions{{Mode: "cascade"}, {Mode: "cascade", ConfirmName: "wrong"}, {Mode: "migrate", TargetModuleID: s.Modules[0].ID}, {Mode: "migrate", TargetModuleID: "unknown"}, {Mode: "other"}} {
		if _, e = store.Apply(s.Revision, "module.delete", func(v *State) (string, error) { return DeleteModuleWithContents(v, s.Modules[0].ID, option) }); e == nil {
			t.Fatal("invalid delete accepted")
		}
		after, _ := store.Snapshot()
		if !reflect.DeepEqual(after, s) {
			t.Fatal("failed operation changed data")
		}
	}
	image, _ := store.ExportImage()
	encrypted, e := OpenMemory(image, func([]byte) error { return errors.New("disk full") })
	if e != nil {
		t.Fatal(e)
	}
	defer encrypted.Close()
	if _, e = encrypted.Apply(s.Revision, "module.delete", func(v *State) (string, error) {
		return DeleteModuleWithContents(v, s.Modules[0].ID, SpaceDeleteOptions{Mode: "cascade", ConfirmName: s.Modules[0].Name})
	}); !errors.Is(e, ErrPersistence) {
		t.Fatal(e)
	}
	after, _ := encrypted.Snapshot()
	if !reflect.DeepEqual(after, s) {
		t.Fatal("encryption failure did not roll back")
	}
}
