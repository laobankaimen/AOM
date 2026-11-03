package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestNetworkRoomScopeAndOptionalLayerSettingsPersist(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	s, err := store.Apply(s.Revision, "fixture", func(s *core.State) (string, error) {
		s.Networks = append(s.Networks, core.Network{ID: "scope", ProjectID: s.Projects[0].ID, SiteID: s.Sites[0].ID, Name: "机房网络", Purpose: "business", Color: "#497cda", DeviceIDs: []string{}})
		return "fixture", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	n := *s.Network("scope")
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "network": n})
	if w := call(h, "PUT", "/api/networks/scope", string(body)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	before := core.Clone(s)
	n.SiteID = "missing-room"
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "network": n})
	if w := call(h, "PUT", "/api/networks/scope", string(body)); w.Code != 400 {
		t.Fatal("invalid room accepted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid room partially saved")
	}
	for _, visible := range []bool{true, false, true} {
		body, _ = json.Marshal(map[string]any{"revision": s.Revision, "layersVisible": visible, "layers": []core.NetworkLayer{{ID: "app", Name: "自定义应用"}}, "deviceLayers": map[string]string{}})
		w := call(h, "PUT", "/api/networks/scope/layers", string(body))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		s, _ = store.Snapshot()
		n = *s.Network("scope")
		if n.LayersVisible == nil || *n.LayersVisible != visible || n.Layers[0].Name != "自定义应用" || n.SiteID != before.Sites[0].ID {
			t.Fatal("visibility erased custom layers or room")
		}
	}
	stale := call(h, "PUT", "/api/networks/scope/layers", fmt.Sprintf(`{"revision":%d,"layersVisible":false}`, before.Revision))
	if stale.Code != 409 {
		t.Fatal("stale layer toggle accepted")
	}
	image, _ := store.ExportImage()
	restored, err := core.OpenMemory(image, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	loaded, _ := restored.Snapshot()
	if !reflect.DeepEqual(s, loaded) {
		t.Fatal("scope or optional layers lost on reopen")
	}
}
