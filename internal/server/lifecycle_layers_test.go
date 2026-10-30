package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestLayerAndDeleteNetworkAPIKeepAssetsAndLinks(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, e := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		for _, id := range []string{"a", "b"} {
			s.Devices = append(s.Devices, core.Device{ID: id, Name: id, AssetTag: id, ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"})
		}
		s.Networks = append(s.Networks, core.Network{ID: "net", ProjectID: s.Projects[0].ID, Name: "网络", Purpose: "business", Color: "#497cda", DeviceIDs: []string{"a", "b"}})
		s.Links = append(s.Links, core.Link{ID: "wire", ProjectID: s.Projects[0].ID, SourceID: "a", TargetID: "b", Kind: "physical", Status: "confirmed", Origin: "manual", SourcePort: "eth1", TargetPort: "eth2", NetworkIDs: []string{"net"}})
		return "fixture", nil
	})
	if e != nil {
		t.Fatal(e)
	}
	post := func(method, path string, body any) int {
		data, _ := json.Marshal(body)
		return call(h, method, path, string(data)).Code
	}
	if code := post("PUT", "/api/networks/net/layers", map[string]any{"revision": s.Revision, "layers": []core.NetworkLayer{{ID: "app", Name: "应用层"}}, "deviceLayers": map[string]string{"a": "app"}}); code != 200 {
		t.Fatal(code)
	}
	s, _ = store.Snapshot()
	if code := post("POST", "/api/networks/net/layer-members", map[string]any{"revision": s.Revision, "deviceIds": []string{"a", "b"}, "layerId": "app"}); code != 200 {
		t.Fatal(code)
	}
	s, _ = store.Snapshot()
	before := core.Clone(s)
	if code := post("POST", "/api/networks/net/layer-members", map[string]any{"revision": s.Revision, "deviceIds": []string{"a", "missing"}, "layerId": "app"}); code != 400 {
		t.Fatal(code)
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(after, before) {
		t.Fatal("invalid batch partially persisted")
	}
	if w := call(h, "DELETE", "/api/networks/net", fmt.Sprintf(`{"revision":%d}`, s.Revision)); w.Code != 400 {
		t.Fatal("detach must be explicit")
	}
	if code := post("DELETE", "/api/networks/net", map[string]any{"revision": s.Revision, "detach": true}); code != 200 {
		t.Fatal(code)
	}
	after, _ = store.Snapshot()
	if after.Network("net") != nil || !reflect.DeepEqual(after.Devices, before.Devices) || len(after.Links) != 1 || after.Links[0].SourcePort != "eth1" || len(after.Links[0].NetworkIDs) != 0 {
		t.Fatal("deletion removed assets or physical facts")
	}
}
