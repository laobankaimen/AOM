package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestNetworkAPIWorkflow(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	_, e := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, v := range []struct{ id, model string }{{"switch", "model-25g"}, {"server", "model-r722"}} {
			s.Devices = append(s.Devices, core.Device{ID: v.id, AssetTag: v.id, Name: v.id, ModelID: v.model, ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s, _ = store.Snapshot()
	n := core.Network{ProjectID: s.Projects[0].ID, Name: "业务网", Purpose: "business", Color: "#497cda", VLAN: 100, DeviceIDs: []string{"switch", "server"}}
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "network": n})
	w := call(h, "POST", "/api/networks", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	n = s.Networks[0]
	payload := fmt.Sprintf(`{"revision":%d,"networkId":%q}`, s.Revision, n.ID)
	w = call(h, "POST", "/api/topology/preview", payload)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var preview struct {
		Links []core.Link `json:"links"`
	}
	json.Unmarshal(w.Body.Bytes(), &preview)
	if len(preview.Links) != 1 || preview.Links[0].Status != "pending" {
		t.Fatal("incorrect preview")
	}
	s, _ = store.Snapshot()
	if len(s.Links) != 0 {
		t.Fatal("preview persisted")
	}
	w = call(h, "POST", "/api/topology/commit", payload)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(h, "POST", "/api/topology/commit", payload)
	if w.Code != 409 {
		t.Fatal("duplicate commit accepted")
	}
	s, _ = store.Snapshot()
	l := s.Links[0]
	l.Kind = "physical"
	l.Status = "confirmed"
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "link": l})
	w = call(h, "PUT", "/api/links/"+l.ID, string(body))
	if w.Code != 400 {
		t.Fatal("confirmed without ports")
	}
	l.SourcePort = "Ethernet1"
	l.TargetPort = "eth0"
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "link": l})
	w = call(h, "PUT", "/api/links/"+l.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	w = call(h, "DELETE", "/api/devices/server", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 400 {
		t.Fatal("linked device deleted")
	}
	w = call(h, "DELETE", "/api/networks/"+n.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 400 {
		t.Fatal("referenced network deleted")
	}
	n.DeviceIDs = []string{"switch"}
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "network": n})
	w = call(h, "PUT", "/api/networks/"+n.ID, string(body))
	if w.Code != 400 {
		t.Fatal("linked member removed")
	}
	w = call(h, "DELETE", "/api/links/"+l.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	for _, deviceID := range []string{"switch", "server"} {
		b, _ := json.Marshal(map[string]any{"revision": s.Revision, "deviceId": deviceID, "sourceId": n.ID, "mode": "remove"})
		response := call(h, "POST", "/api/network-members", string(b))
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		s, _ = store.Snapshot()
	}
	w = call(h, "DELETE", "/api/networks/"+n.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	w = call(h, "DELETE", "/api/devices/server", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}

func TestNetworkStructureAndPositionAPI(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	_, e := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		s.Devices = append(s.Devices, core.Device{ID: "d1", Name: "d1", AssetTag: "d1", ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		s.Networks = append(s.Networks, core.Network{ID: "root", ProjectID: s.Projects[0].ID, Name: "根网", Purpose: "business", Color: "#497cda", DeviceIDs: []string{}}, core.Network{ID: "child", ParentID: "root", ProjectID: s.Projects[0].ID, Name: "子网", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"d1"}})
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s, _ = store.Snapshot()
	w := call(h, "POST", "/api/networks/child/position", fmt.Sprintf(`{"revision":%d,"deviceId":"d1","position":{"x":250,"y":300}}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Networks[1].DevicePositions["d1"].X != 250 {
		t.Fatal("position not persisted")
	}
	w = call(h, "POST", "/api/networks/root/move", fmt.Sprintf(`{"revision":%d,"parentId":"child"}`, s.Revision))
	if w.Code != 400 {
		t.Fatal("cycle allowed")
	}
	w = call(h, "DELETE", "/api/networks/root", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 400 {
		t.Fatal("parent deleted")
	}
	w = call(h, "POST", "/api/network-members", fmt.Sprintf(`{"revision":%d,"deviceId":"d1","sourceId":"child","targetId":"root","mode":"move"}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if len(s.Network("child").DeviceIDs) != 0 || len(s.Network("child").DevicePositions) != 0 {
		t.Fatal("stale member or coordinate")
	}
	w = call(h, "POST", "/api/networks/child/move", fmt.Sprintf(`{"revision":%d,"parentId":""}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	w = call(h, "DELETE", "/api/networks/child", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}

func TestBatchNetworkMembershipAndRedundantLinksAreAtomic(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, e := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"up-a", "up-b", "server-a", "server-b"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		s.Networks = append(s.Networks, core.Network{ID: "batch-net", ProjectID: s.Projects[0].ID, Name: "批量网络", Purpose: "business", Color: "#497cda", DeviceIDs: []string{}})
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	post := func(path string, payload map[string]any) int {
		body, _ := json.Marshal(payload)
		return call(h, "POST", path, string(body)).Code
	}
	// One invalid member rolls back the whole selection.
	revision := s.Revision
	if code := post("/api/network-members", map[string]any{"revision": revision, "deviceIds": []string{"up-a", "missing"}, "targetId": "batch-net", "mode": "add"}); code == 200 {
		t.Fatal("invalid selection accepted")
	}
	s, _ = store.Snapshot()
	if s.Revision != revision || len(s.Network("batch-net").DeviceIDs) != 0 {
		t.Fatal("partial membership committed")
	}
	ids := []string{"up-a", "up-b", "server-a", "server-b"}
	if code := post("/api/network-members", map[string]any{"revision": revision, "deviceIds": ids, "targetId": "batch-net", "mode": "add"}); code != 200 {
		t.Fatal(code)
	}
	s, _ = store.Snapshot()
	if s.Revision != revision+1 || len(s.Network("batch-net").DeviceIDs) != 4 {
		t.Fatal("batch must commit once")
	}
	payload := map[string]any{"revision": s.Revision, "networkId": "batch-net", "sources": []string{"up-a", "up-b"}, "targets": []string{"server-a", "server-b"}}
	if code := post("/api/links/batch", payload); code != 200 {
		t.Fatal(code)
	}
	s, _ = store.Snapshot()
	if len(s.Links) != 4 {
		t.Fatalf("expected four redundant relations: %d", len(s.Links))
	}
	for _, l := range s.Links {
		if l.Kind != "logical" || l.Status != "pending" || l.SourcePort != "" || l.TargetPort != "" {
			t.Fatal("batch fabricated physical connection")
		}
	}
	revision = s.Revision
	payload["revision"] = revision
	if code := post("/api/links/batch", payload); code == 200 {
		t.Fatal("duplicate batch accepted")
	}
	s, _ = store.Snapshot()
	if s.Revision != revision || len(s.Links) != 4 {
		t.Fatal("duplicate changed state")
	}
	payload["sources"] = []string{"up-a"}
	payload["targets"] = []string{"up-b", "up-a"}
	if code := post("/api/links/batch", payload); code == 200 {
		t.Fatal("self connection accepted")
	}
	s, _ = store.Snapshot()
	if s.Revision != revision || len(s.Links) != 4 {
		t.Fatal("partial links committed")
	}
	payload["targets"] = []string{"missing"}
	if code := post("/api/links/batch", payload); code == 200 {
		t.Fatal("non-member accepted")
	}
}

func TestAutoLayoutOnlyClearsRequestedDiagramCoordinates(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, e := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"one", "two", "unassigned"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending", TopologyPosition: &core.CanvasPoint{X: 2400, Y: 600}})
		}
		for _, id := range []string{"net-a", "net-b"} {
			s.Networks = append(s.Networks, core.Network{ID: id, ProjectID: s.Projects[0].ID, Name: id, Purpose: "business", Color: "#497cda", DeviceIDs: []string{"one", "two"}, DevicePositions: map[string]core.CanvasPoint{"one": {X: 2400, Y: 600}, "two": {X: 900, Y: 450}}})
		}
		s.Links = append(s.Links, core.Link{ID: "kept-link", ProjectID: s.Projects[0].ID, SourceID: "one", TargetID: "two", Kind: "logical", Status: "pending", Origin: "manual", NetworkIDs: []string{"net-a"}})
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	before := s
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "projectId": s.Projects[0].ID, "networkIds": []string{"net-a"}, "deviceIds": []string{"one", "unassigned"}})
	w := call(h, "POST", "/api/topology/auto-layout", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Revision != before.Revision+1 {
		t.Fatal("not a single change")
	}
	if _, ok := s.Network("net-a").DevicePositions["one"]; ok {
		t.Fatal("requested coordinate preserved")
	}
	if s.Network("net-a").DevicePositions["two"].X != 900 || s.Network("net-b").DevicePositions["one"].X != 2400 {
		t.Fatal("changed unrelated positions")
	}
	if s.Device("unassigned").TopologyPosition != nil || s.Device("one").TopologyPosition == nil {
		t.Fatal("incorrect unassigned position reset")
	}
	if !reflect.DeepEqual(before.Links, s.Links) || !reflect.DeepEqual(before.Networks[0].DeviceIDs, s.Networks[0].DeviceIDs) {
		t.Fatal("auto layout changed connectivity")
	}
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "projectId": "other-project", "networkIds": []string{"net-a"}, "deviceIds": []string{"two"}})
	if w = call(h, "POST", "/api/topology/auto-layout", string(body)); w.Code == 200 {
		t.Fatal("cross-project reset allowed")
	}
	after, _ := store.Snapshot()
	if after.Revision != s.Revision || !reflect.DeepEqual(after, s) {
		t.Fatal("failed reset partially changed data")
	}
	if w = call(h, "POST", "/api/topology/auto-layout", fmt.Sprintf(`{"revision":%d,"projectId":%q,"networkIds":["net-a"],"deviceIds":["two"]}`, before.Revision, s.Projects[0].ID)); w.Code != 409 {
		t.Fatal("stale revision accepted")
	}
}
