package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestModelImportRolesAndAtomicConflicts(t *testing.T) {
	h, store := testAPI(t)
	state, _ := store.Snapshot()
	send := func(path string, revision int, content string) *core.State {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"revision": revision, "content": content})
		w := call(h, "POST", path, string(body))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var s core.State
		_ = json.Unmarshal(w.Body.Bytes(), &s)
		return &s
	}
	roles := core.DefaultDeviceRoles()
	roles[0].Name = "计算节点"
	roles = append(roles, core.DeviceRole{ID: "custom-firewall", Name: "边界防火墙", Color: "#aabbcc", BaseRole: "access"})
	body, _ := json.Marshal(map[string]any{"revision": state.Revision, "roles": roles})
	w := call(h, "PUT", "/api/device-roles", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &state)
	content := "型号,厂商,设备角色,U高,端口说明\nFW-TEST,实验厂商,边界防火墙,1,8口\nFW-TEST,实验厂商,边界防火墙,1,8口\nNODE-TEST,,计算节点,2,\n"
	before := state.Revision
	body, _ = json.Marshal(map[string]any{"revision": before, "content": content})
	preview := call(h, "POST", "/api/models/import/preview", string(body))
	if preview.Code != 200 {
		t.Fatal(preview.Body.String())
	}
	var p core.ModelImportPreview
	_ = json.Unmarshal(preview.Body.Bytes(), &p)
	if p.Added != 2 || p.Skipped != 1 {
		t.Fatalf("preview %+v", p)
	}
	check, _ := store.Snapshot()
	if check.Revision != before || len(check.Models) != len(state.Models) {
		t.Fatal("preview wrote state")
	}
	state = *send("/api/models/import/commit", before, content)
	if state.RoleBase("custom-firewall") != "access" {
		t.Fatal("custom topology classification lost")
	}
	same := call(h, "POST", "/api/models/import/commit", string(body))
	if same.Code != 409 {
		t.Fatal("stale preview accepted")
	}
	exported := call(h, "GET", "/api/export/models.csv", "")
	if exported.Code != 200 || !strings.Contains(exported.Body.String(), "边界防火墙") {
		t.Fatal("role name not exported")
	}
	roundtrip, err := core.PreviewModels(state, exported.Body.String())
	if err != nil || roundtrip.Added != 0 {
		t.Fatalf("roundtrip: %v %+v", err, roundtrip)
	}
	conflict := "model,role,u_height\nMust-not-be-added,server,1\nFW-TEST,server,42\n"
	body, _ = json.Marshal(map[string]any{"revision": state.Revision, "content": conflict})
	bad := call(h, "POST", "/api/models/import/commit", string(body))
	if bad.Code != 400 {
		t.Fatalf("conflict accepted: %d", bad.Code)
	}
	after, _ := store.Snapshot()
	if after.Revision != state.Revision || len(after.Models) != len(state.Models) {
		t.Fatal("partial import committed")
	}
	body, _ = json.Marshal(map[string]any{"revision": state.Revision, "roles": roles[:len(roles)-1]})
	bad = call(h, "PUT", "/api/device-roles", string(body))
	if bad.Code != 400 {
		t.Fatal("referenced role deleted")
	}
	for _, csv := range []string{"model,role,u_height\nBad,不存在,1", "model,role,u_height\nBad,server,1.5", "model,role,u_height\nBad,server,43"} {
		body, _ = json.Marshal(map[string]any{"revision": state.Revision, "content": csv})
		if bad := call(h, "POST", "/api/models/import/preview", string(body)); bad.Code != 400 {
			t.Fatalf("invalid input accepted %s", csv)
		}
	}
}
func TestCreateModelWithInlineCustomRole(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	body := fmt.Sprintf(`{"revision":%d,"model":{"name":"inline-test","role":"__new","uHeight":1},"roleDefinition":{"name":"授时设备","color":"#123456","baseRole":"infra"}}`, s.Revision)
	w := call(h, "POST", "/api/models", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	m := s.Models[len(s.Models)-1]
	if m.Role == "__new" || s.ResolveRole("授时设备") != m.Role {
		t.Fatal("inline role not saved")
	}
	roles := s.RoleDefinitions()
	roles[len(roles)-1].Name = roles[0].Name
	payload, _ := json.Marshal(map[string]any{"revision": s.Revision, "roles": roles})
	w = call(h, "PUT", "/api/device-roles", string(payload))
	if w.Code != 400 {
		t.Fatal("duplicate role name allowed")
	}
}
