package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestLayerRenameKeepsManualPositions(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, err := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		s.Devices = append(s.Devices, core.Device{ID: "layer-device", Name: "服务器", AssetTag: "LAYER-RENAME", ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		s.Networks = append(s.Networks, core.Network{ID: "layer-network", ProjectID: s.Projects[0].ID, Name: "分层测试", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"layer-device"}, DevicePositions: map[string]core.CanvasPoint{"layer-device": {X: 250, Y: 125}}})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	layers := core.DefaultNetworkLayers()
	layers[3].Name = "业务计算层"
	payload, _ := json.Marshal(map[string]any{"revision": s.Revision, "layers": layers, "deviceLayers": map[string]string{}})
	response := call(h, "PUT", "/api/networks/layer-network/layers", string(payload))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	s, _ = store.Snapshot()
	if !reflect.DeepEqual(s.Network("layer-network").DevicePositions, map[string]core.CanvasPoint{"layer-device": {X: 250, Y: 125}}) {
		t.Fatal("renaming a layer erased manual positions")
	}
	payload, _ = json.Marshal(map[string]any{"revision": s.Revision, "layers": layers, "deviceLayers": map[string]string{"layer-device": "access"}})
	response = call(h, "PUT", "/api/networks/layer-network/layers", string(payload))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	s, _ = store.Snapshot()
	if len(s.Network("layer-network").DevicePositions) != 0 {
		t.Fatal("changed layer kept old position")
	}
}

func TestMembershipUndoRestoresTagsAndRejectsStaleRevision(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, err := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"undo-one", "undo-two"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		for _, id := range []string{"undo-source", "undo-target"} {
			devices := []string{}
			if id == "undo-source" {
				devices = []string{"undo-one", "undo-two"}
			}
			s.Networks = append(s.Networks, core.Network{ID: id, ProjectID: s.Projects[0].ID, Name: id, Purpose: "business", Color: "#497cda", DeviceIDs: devices})
		}
		s.Networks[0].DevicePositions = map[string]core.CanvasPoint{"undo-one": {X: 30, Y: 40}}
		s.Links = append(s.Links, core.Link{ID: "undo-link", ProjectID: s.Projects[0].ID, SourceID: "undo-one", TargetID: "undo-two", NetworkIDs: []string{"undo-source"}, Kind: "logical", Status: "pending", Origin: "manual"})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := core.Clone(s)
	moved := call(h, "POST", "/api/network-members", fmt.Sprintf(`{"revision":%d,"deviceIds":["undo-one"],"sourceId":"undo-source","targetId":"undo-target","mode":"move"}`, s.Revision))
	if moved.Code != 200 {
		t.Fatal(moved.Body.String())
	}
	s, _ = store.Snapshot()
	payload, _ := json.Marshal(map[string]any{"revision": s.Revision, "projectId": s.Projects[0].ID, "networks": before.Networks, "links": before.Links})
	restored := call(h, "POST", "/api/topology/restore-membership", string(payload))
	if restored.Code != 200 {
		t.Fatal(restored.Body.String())
	}
	s, _ = store.Snapshot()
	if !reflect.DeepEqual(s.Networks, before.Networks) || !reflect.DeepEqual(s.Links, before.Links) || !reflect.DeepEqual(s.Devices, before.Devices) {
		t.Fatal("undo did not restore memberships/tags/positions exactly")
	}
	stale := call(h, "POST", "/api/topology/restore-membership", string(payload))
	if stale.Code != 409 {
		t.Fatal("stale undo was accepted")
	}
}

func TestDeviceDeleteCanExplicitlyDetachReferences(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, err := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"delete-one", "keep-two"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		s.Networks = append(s.Networks, core.Network{ID: "delete-network", ProjectID: s.Projects[0].ID, Name: "网络保留", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"delete-one", "keep-two"}, DevicePositions: map[string]core.CanvasPoint{"delete-one": {X: 30, Y: 40}}})
		s.Links = append(s.Links, core.Link{ID: "delete-link", ProjectID: s.Projects[0].ID, SourceID: "delete-one", TargetID: "keep-two", NetworkIDs: []string{"delete-network"}, Kind: "logical", Status: "pending", Origin: "manual"})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	response := call(h, "DELETE", "/api/devices/delete-one", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if response.Code == 200 {
		t.Fatal("references deleted without explicit confirmation")
	}
	response = call(h, "DELETE", "/api/devices/delete-one", fmt.Sprintf(`{"revision":%d,"detach":true}`, s.Revision))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Device("delete-one") != nil || s.Device("keep-two") == nil || s.Network("delete-network") == nil || len(s.Links) != 0 || !reflect.DeepEqual(s.Network("delete-network").DeviceIDs, []string{"keep-two"}) {
		t.Fatal("unexpected deletion impact")
	}
}
