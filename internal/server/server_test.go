package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testAPI(t *testing.T) (http.Handler, *core.Store) {
	t.Helper()
	s, e := core.Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return newApplication(s, fstest.MapFS{"index.html": {Data: []byte("<html>AOM</html>")}}), s
}
func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
func TestAPIRoutesAndBoundary(t *testing.T) {
	h, _ := testAPI(t)
	for _, x := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/api/health", "", 200}, {"GET", "/api/state", "", 200}, {"GET", "/api/unknown", "", 404}, {"GET", "/arbitrary-page", "", 200}, {"GET", "/missing.js", "", 404}, {"GET", "/api/audits/9999", "", 404}, {"POST", "/api/plans", "bad", 400}, {"POST", "/api/plans", "{} {}", 400}} {
		w := call(h, x.method, x.path, x.body)
		if w.Code != x.status {
			t.Fatalf("%s got %d %s", x.path, w.Code, w.Body.String())
		}
	}
	req := httptest.NewRequest("POST", "/api/plans", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("cross-origin write allowed")
	}
}
func TestCSVImportAndAuditAPI(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	payload := map[string]any{"revision": s.Revision, "moduleId": s.Modules[0].ID, "content": "model,quantity\nKunTai R722 K2,3\nRZ18001-1,1"}
	b, _ := json.Marshal(payload)
	preview := call(h, "POST", "/api/import/preview", string(b))
	if preview.Code != 200 {
		t.Fatal(preview.Body.String())
	}
	unchanged, _ := store.Snapshot()
	if len(unchanged.Devices) != 0 {
		t.Fatal("preview wrote data")
	}
	committed := call(h, "POST", "/api/import/commit", string(b))
	if committed.Code != 201 {
		t.Fatal(committed.Body.String())
	}
	after, _ := store.Snapshot()
	if len(after.Devices) != 4 {
		t.Fatal("wrong import count")
	}
	if retry := call(h, "POST", "/api/import/commit", string(b)); retry.Code != 409 {
		t.Fatal("duplicate submit not rejected")
	}
	export := call(h, "GET", "/api/export/devices.csv", "")
	if export.Code != 200 || !strings.Contains(export.Body.String(), "AOM-0001") {
		t.Fatal("CSV export missing")
	}
	audit := call(h, "GET", "/api/audits/2", "")
	if audit.Code != 200 || !strings.Contains(audit.Body.String(), "before") || !strings.Contains(audit.Body.String(), "after") {
		t.Fatal("audit incomplete")
	}
}
func TestDeviceCRUDAndConflictRollback(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	d := core.Device{AssetTag: "manual-1", Name: "manual", ModelID: "model-r722", ModuleID: s.Modules[0].ID, Status: "pending"}
	payload, _ := json.Marshal(map[string]any{"revision": s.Revision, "device": d, "autoPlace": true})
	w := call(h, "POST", "/api/devices", string(payload))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	d = s.Devices[0]
	if d.StartU != 1 {
		t.Fatal("auto place failed")
	}
	w = call(h, "POST", "/api/devices/"+d.ID+"/placement", fmt.Sprintf(`{"revision":%d,"rackId":%q,"startU":42}`, s.Revision, d.RackID))
	if w.Code != 400 {
		t.Fatal("overflow accepted")
	}
	s, _ = store.Snapshot()
	if s.Devices[0].StartU != 1 {
		t.Fatal("bad placement persisted")
	}
	w = call(h, "DELETE", "/api/models/model-r722", fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 400 {
		t.Fatal("referenced model deleted")
	}
	w = call(h, "DELETE", "/api/devices/"+d.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if len(s.Devices) != 0 {
		t.Fatal("device not removed")
	}
}

func TestExportPlanPreservesPerModuleInfrastructure(t *testing.T) {
	h, store := testAPI(t)
	state, _ := store.Snapshot()
	siteID := state.Sites[0].ID
	plan := core.Plan{SiteID: siteID, Modules: []core.ModulePlan{{Name: "无基础设施模块", Type: "single", RackCount: 1, UHeight: 42}}, Infra: core.InfraPlan{}}
	_, err := store.Apply(state.Revision, "plan.create", func(s *core.State) (string, error) { return core.AddPlan(s, plan) })
	if err != nil {
		t.Fatal(err)
	}
	w := call(h, "GET", "/api/export/plan.json?siteId="+siteID, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var exported core.Plan
	if err = json.Unmarshal(w.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Modules[len(exported.Modules)-1].Infra == nil || *exported.Modules[len(exported.Modules)-1].Infra != (core.InfraPlan{}) {
		t.Fatal("per-module infrastructure was lost")
	}
	exported.SiteName = "复制机房"
	copy := core.Seed()
	exported.ProjectID = copy.Projects[0].ID
	if _, err = core.AddPlan(&copy, exported); err != nil {
		t.Fatal(err)
	}
	last := copy.Modules[len(copy.Modules)-1]
	for _, slot := range copy.Infra {
		if slot.ModuleID == last.ID {
			t.Fatal("extra infrastructure generated on roundtrip")
		}
	}
}

func TestInfrastructureCRUDAndLayoutExport(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	m := s.Modules[1]
	item := core.Infra{ModuleID: m.ID, Name: "右列空调", Kind: "ac", Side: "R", AfterRack: 2, Placement: "inline"}
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "infra": item})
	w := call(h, "POST", "/api/infrastructure", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	created := s.Infra[len(s.Infra)-1]
	created.AfterRack = 3
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "infra": created})
	w = call(h, "PUT", "/api/infrastructure/"+created.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	bad := created
	bad.AfterRack = 99
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "infra": bad})
	w = call(h, "PUT", "/api/infrastructure/"+created.ID, string(body))
	if w.Code != 400 {
		t.Fatal("out-of-row location accepted")
	}
	w = call(h, "GET", "/api/export/plan.json?siteId="+m.SiteID, "")
	var plan core.Plan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, slot := range plan.Modules[1].Layout {
		if slot.Name == created.Name && slot.Side == "R" && slot.AfterRack == 3 {
			found = true
		}
	}
	if !found {
		t.Fatal("export lost physical layout")
	}
	w = call(h, "DELETE", "/api/infrastructure/"+created.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}
func TestRenameProjectAndReparentModule(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	w := call(h, "PUT", "/api/projects/"+s.Projects[0].ID, fmt.Sprintf(`{"revision":%d,"name":"正式项目"}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Projects[0].Name != "正式项目" {
		t.Fatal("project rename failed")
	}
	_, err := store.Apply(s.Revision, "plan", func(s *core.State) (string, error) {
		return core.AddPlan(s, core.Plan{ProjectName: "新项目", SiteName: "二号机房", Modules: []core.ModulePlan{{Name: "新模块", Type: "single", RackCount: 1, UHeight: 42}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	s, _ = store.Snapshot()
	m := s.Modules[0]
	target := s.Sites[len(s.Sites)-1]
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "name": m.Name, "siteId": target.ID, "productModel": m.ProductModel, "point": m.Point})
	w = call(h, "PUT", "/api/modules/"+m.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Modules[0].SiteID != target.ID {
		t.Fatal("module reparent failed")
	}
}

func TestRackReorderExportAndRename(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	m := s.Modules[0]
	ordered := core.OrderedRacks(s, m.ID, "L")
	ids := []string{ordered[1].ID, ordered[0].ID}
	for _, r := range ordered[2:] {
		ids = append(ids, r.ID)
	}
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "left": ids, "right": []string{}})
	w := call(h, "POST", "/api/modules/"+m.ID+"/rack-order", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	stale := call(h, "POST", "/api/modules/"+m.ID+"/rack-order", string(body))
	if stale.Code != 409 {
		t.Fatal("stale reorder accepted")
	}
	s, _ = store.Snapshot()
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "name": "业务机柜"})
	w = call(h, "PUT", "/api/racks/"+ordered[1].ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(h, "GET", "/api/export/plan.json?siteId="+m.SiteID, "")
	var plan core.Plan
	json.Unmarshal(w.Body.Bytes(), &plan)
	if plan.Modules[0].RackNames["L"][0] != "业务机柜" || plan.Modules[0].RackNames["L"][1] != ordered[0].Name {
		t.Fatal("export lost cabinet order")
	}
	plan.ProjectID = s.Projects[0].ID
	plan.SiteName = "复用布局"
	copy := core.Seed()
	if _, e := core.AddPlan(&copy, plan); e != nil {
		t.Fatal(e)
	}
	newModule := copy.Modules[len(copy.Modules)-3]
	if core.OrderedRacks(copy, newModule.ID, "L")[0].Name != "业务机柜" {
		t.Fatal("import lost cabinet names")
	}
}

func TestRoomLayoutAndInfraPlacementAPI(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	m := s.Modules[1]
	payload := fmt.Sprintf(`{"revision":%d,"position":{"x":350,"y":200}}`, s.Revision)
	w := call(h, "POST", "/api/modules/"+m.ID+"/room-position", payload)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(h, "POST", "/api/modules/"+m.ID+"/room-position", payload)
	if w.Code != 409 {
		t.Fatal("stale position accepted")
	}
	s, _ = store.Snapshot()
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "name": m.Name + "重命名", "siteId": m.SiteID, "productModel": m.ProductModel, "point": m.Point})
	w = call(h, "PUT", "/api/modules/"+m.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Modules[1].RoomPosition == nil || s.Modules[1].RoomPosition.X != 350 {
		t.Fatal("metadata edit lost coordinates")
	}
	var infra core.Infra
	for _, i := range s.Infra {
		if i.ModuleID == m.ID && i.Kind == "ac" && i.Side == "L" {
			infra = i
			break
		}
	}
	w = call(h, "POST", "/api/infrastructure/"+infra.ID+"/placement", fmt.Sprintf(`{"revision":%d,"side":"R","afterRack":3}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	for _, i := range s.Infra {
		if i.ID == infra.ID && (i.Side != "R" || i.AfterRack != 3 || i.Name != infra.Name || i.Kind != infra.Kind) {
			t.Fatal("facility move altered metadata")
		}
	}
	w = call(h, "GET", "/api/export/plan.json?siteId="+m.SiteID, "")
	var plan core.Plan
	json.Unmarshal(w.Body.Bytes(), &plan)
	if plan.Modules[1].RoomPosition == nil || plan.Modules[1].RoomPosition.Y != 200 {
		t.Fatal("plan lost coordinates")
	}
	_, e := store.Apply(s.Revision, "new room", func(s *core.State) (string, error) {
		return core.AddPlan(s, core.Plan{ProjectID: s.Projects[0].ID, SiteName: "其他机房", Modules: []core.ModulePlan{{Name: "接收模块", Type: "single", RackCount: 1, UHeight: 42}}})
	})
	if e != nil {
		t.Fatal(e)
	}
	s, _ = store.Snapshot()
	target := s.Sites[len(s.Sites)-1]
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "name": m.Name, "siteId": target.ID, "productModel": m.ProductModel, "point": m.Point})
	w = call(h, "PUT", "/api/modules/"+m.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Modules[1].RoomPosition != nil {
		t.Fatal("new room kept old room coordinates")
	}
}

func TestRoomRotationAPIAndPlanRoundtrip(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	m := s.Modules[0]
	payload := fmt.Sprintf(`{"revision":%d,"position":{"x":200.5,"y":100.25},"rotation":90}`, s.Revision)
	w := call(h, "POST", "/api/modules/"+m.ID+"/room-position", payload)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Modules[0].RoomRotation != 90 || s.Modules[0].RoomPosition.X != 200.5 {
		t.Fatal("transform lost")
	}
	w = call(h, "POST", "/api/modules/"+m.ID+"/room-position", fmt.Sprintf(`{"revision":%d,"position":{"x":250,"y":150}}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Modules[0].RoomRotation != 90 {
		t.Fatal("move without rotation reset angle")
	}
	w = call(h, "GET", "/api/export/plan.json?siteId="+m.SiteID, "")
	var plan core.Plan
	if e := json.Unmarshal(w.Body.Bytes(), &plan); e != nil {
		t.Fatal(e)
	}
	if plan.Modules[0].RoomRotation != 90 {
		t.Fatal("plan lost orientation")
	}
	plan.SiteName = "旋转复用"
	plan.SiteID = ""
	plan.ProjectID = s.Projects[0].ID
	if _, e := core.AddPlan(&s, plan); e != nil {
		t.Fatal(e)
	}
	if s.Modules[len(s.Modules)-len(plan.Modules)].RoomRotation != 90 {
		t.Fatal("import lost orientation")
	}
}
