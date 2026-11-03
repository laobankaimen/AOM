package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestNetworkDiagramPositionPreservesBusinessDataWithoutAudit(t *testing.T) {
	handler, store := testAPI(t)
	s, _ := store.Snapshot()
	s, err := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		s.Devices = append(s.Devices, core.Device{ID: "layout-device", Name: "设备", AssetTag: "layout-device", ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		s.Networks = append(s.Networks, core.Network{ID: "layout-root", ProjectID: s.Projects[0].ID, Name: "根网", Purpose: "business", Color: "#497cda", DeviceIDs: []string{}}, core.Network{ID: "layout-net", ParentID: "layout-root", ProjectID: s.Projects[0].ID, Name: "子网", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"layout-device"}, DevicePositions: map[string]core.CanvasPoint{"layout-device": {X: 25, Y: 30}}})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeAudits, _ := store.Audits()
	original := core.Clone(s)
	payload := fmt.Sprintf(`{"revision":%d,"position":{"x":640.25,"y":230.75}}`, s.Revision)
	response := call(handler, "POST", "/api/networks/layout-net/layout-position", payload)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	s, _ = store.Snapshot()
	n := s.Network("layout-net")
	if n.Position == nil || n.Position.X != 640.25 || n.Position.Y != 230.75 {
		t.Fatal("position not saved")
	}
	if n.PositionSpace != "parent" {
		t.Fatal("child coordinates must be parent-local")
	}
	position := n.Position
	n.Position = nil
	n.PositionSpace = ""
	s.Revision = original.Revision
	if !reflect.DeepEqual(s, original) {
		t.Fatal("diagram move changed business data")
	}
	n.Position = position
	n.PositionSpace = "parent"
	audits, _ := store.Audits()
	if !reflect.DeepEqual(beforeAudits, audits) {
		t.Fatal("diagram move created audit")
	}
	if w := call(handler, "POST", "/api/networks/layout-net/layout-position", payload); w.Code != 409 {
		t.Fatal("stale position accepted")
	}
	current, _ := store.Snapshot()
	for _, point := range []string{`{"x":-1,"y":0}`, `{"x":0,"y":10001}`} {
		w := call(handler, "POST", "/api/networks/layout-net/layout-position", fmt.Sprintf(`{"revision":%d,"position":%s}`, current.Revision, point))
		if w.Code != 400 {
			t.Fatal("invalid coordinates accepted")
		}
	}
	image, _ := store.ExportImage()
	restored, err := core.OpenMemory(image, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	loaded, _ := restored.Snapshot()
	if !reflect.DeepEqual(current, loaded) {
		t.Fatal("layout position lost on reopen")
	}
	data, _ := json.Marshal(map[string]any{"revision": current.Revision, "projectId": current.Projects[0].ID, "networkIds": []string{"layout-net"}, "deviceIds": []string{"layout-device"}})
	if w := call(handler, "POST", "/api/topology/auto-layout", string(data)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	reset, _ := store.Snapshot()
	if reset.Network("layout-net").Position != nil {
		t.Fatal("auto layout did not reset network position")
	}
}

func TestGroupedDevicePositionBatchIsAtomicAndPreservesNetworkMembership(t *testing.T) {
	h, store := testAPI(t)
	state, _ := store.Snapshot()
	state, err := store.Apply(state.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"a", "b"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		s.Networks = append(s.Networks, core.Network{ID: "group-net", ProjectID: s.Projects[0].ID, Name: "批量网络", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"a", "b"}})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeAudits, _ := store.Audits()
	before := core.Clone(state)
	payload := map[string]any{"revision": state.Revision, "projectId": state.Projects[0].ID, "networkId": "group-net", "positions": []any{map[string]any{"deviceId": "a", "position": map[string]float64{"x": 80, "y": 50}}, map[string]any{"deviceId": "b", "position": map[string]float64{"x": 288, "y": 50}}}}
	data, _ := json.Marshal(payload)
	w := call(h, "POST", "/api/topology/device-positions", string(data))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	state, _ = store.Snapshot()
	if state.Revision != before.Revision+1 || len(state.Network("group-net").DevicePositions) != 2 {
		t.Fatal("batch positions not atomic")
	}
	if !reflect.DeepEqual(state.Devices, before.Devices) || !reflect.DeepEqual(state.Network("group-net").DeviceIDs, before.Network("group-net").DeviceIDs) {
		t.Fatal("moving group changed physical devices or membership")
	}
	afterAudits, _ := store.Audits()
	if !reflect.DeepEqual(beforeAudits, afterAudits) {
		t.Fatal("group graphic move generated audit")
	}
	payload["revision"] = state.Revision
	payload["positions"] = []any{map[string]any{"deviceId": "a", "position": map[string]float64{"x": 600, "y": 50}}, map[string]any{"deviceId": "missing", "position": map[string]float64{"x": 900, "y": 50}}}
	data, _ = json.Marshal(payload)
	w = call(h, "POST", "/api/topology/device-positions", string(data))
	if w.Code != 400 {
		t.Fatal("missing member accepted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(state, after) {
		t.Fatal("failed group move partially changed coordinates")
	}
	payload["positions"] = []any{map[string]any{"deviceId": "a", "position": map[string]float64{"x": 10001, "y": 50}}}
	data, _ = json.Marshal(payload)
	if w = call(h, "POST", "/api/topology/device-positions", string(data)); w.Code != 400 {
		t.Fatal("outside coordinates accepted")
	}
	payload["projectId"] = "other-project"
	data, _ = json.Marshal(payload)
	if w = call(h, "POST", "/api/topology/device-positions", string(data)); w.Code != 400 {
		t.Fatal("cross-project positions accepted")
	}
}
