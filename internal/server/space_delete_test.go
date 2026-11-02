package server

import (
	"aom/internal/core"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSpaceDeletionAPIOptionsAndRevision(t *testing.T) {
	for _, mode := range []string{"migrate", "cascade"} {
		t.Run(mode, func(t *testing.T) {
			h, store := testAPI(t)
			s, _ := store.Snapshot()
			source, target := s.Modules[0], s.Modules[1]
			rack := core.OrderedRacks(s, source.ID, "L")[0]
			s, err := store.Apply(s.Revision, "fixture", func(v *core.State) (string, error) {
				v.Devices = append(v.Devices, core.Device{ID: "occupied", Name: "在架设备", AssetTag: "occupied", ModelID: "model-r722", ModuleID: source.ID, RackID: rack.ID, StartU: 1, Status: "installed"}, core.Device{ID: "waiting", Name: "待上架设备", AssetTag: "waiting", ModelID: "model-r722", ModuleID: source.ID, Status: "pending"})
				return "fixture", nil
			})
			if err != nil {
				t.Fatal(err)
			}
			send := func(path string, body map[string]any, status int) {
				t.Helper()
				data, _ := json.Marshal(body)
				w := call(h, "DELETE", path, string(data))
				if w.Code != status {
					t.Fatalf("expected %d, got %d: %s", status, w.Code, w.Body.String())
				}
			}
			// Uninstalling through the API removes the rack and its layout markers but retains assets.
			send("/api/racks/"+rack.ID, map[string]any{"revision": s.Revision, "mode": "uninstall", "removeFacilities": true}, 200)
			s, _ = store.Snapshot()
			if s.Rack(rack.ID) != nil || s.Device("occupied").RackID != "" || s.Device("occupied").Status != "pending" {
				t.Fatal("rack uninstall did not preserve pending assets")
			}
			before := core.Clone(s)
			body := map[string]any{"revision": s.Revision - 1, "mode": mode, "targetModuleId": target.ID, "confirmName": source.Name}
			send("/api/modules/"+source.ID, body, 409)
			after, _ := store.Snapshot()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("stale deletion changed data")
			}
			body["revision"] = s.Revision
			if mode == "cascade" {
				body["confirmName"] = ""
				send("/api/modules/"+source.ID, body, 400)
				body["confirmName"] = source.Name
			}
			send("/api/modules/"+source.ID, body, 200)
			after, _ = store.Snapshot()
			if after.Revision != s.Revision+1 {
				t.Fatal("deletion did not commit once")
			}
			for _, m := range after.Modules {
				if m.ID == source.ID {
					t.Fatal("module retained")
				}
			}
			for _, id := range []string{"occupied", "waiting"} {
				d := after.Device(id)
				if mode == "cascade" {
					if d != nil {
						t.Fatal("device retained")
					}
				} else if d == nil || d.ModuleID != target.ID || d.RackID != "" {
					t.Fatal("device not migrated")
				}
			}
		})
	}
}
