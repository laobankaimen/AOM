package server

import (
	"aom/internal/core"
	"encoding/json"
	"fmt"
	"testing"
)

func TestTemplateAPIAndInstanceIsolation(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	module := s.Modules[0]
	body, _ := json.Marshal(map[string]any{"revision": s.Revision, "moduleId": module.ID, "template": map[string]string{"name": "现场模板"}})
	w := call(h, "POST", "/api/templates", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	saved := s.Templates[len(s.Templates)-1]
	saved.Plan.Name = "改后的默认名"
	saved.Plan.RackNames["L"][0] = "新柜名"
	body, _ = json.Marshal(map[string]any{"revision": s.Revision, "template": saved})
	w = call(h, "PUT", "/api/templates/"+saved.ID, string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if core.OrderedRacks(s, module.ID, "L")[0].Name == "新柜名" || s.Modules[0].Name != module.Name {
		t.Fatal("template edit modified instance")
	}
	w = call(h, "DELETE", "/api/templates/"+saved.ID, fmt.Sprintf(`{"revision":%d}`, s.Revision))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s, _ = store.Snapshot()
	if s.Template(saved.ID) != nil || len(s.Modules) != 3 {
		t.Fatal("template deletion affected instances")
	}
}
func TestInitializationAPIReadOnlyPreviewAndDuplicateCommit(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	req := core.InitRequest{Revision: s.Revision, ProjectID: s.Projects[0].ID, SiteName: "初始化验收", Modules: []core.InitModule{{TemplateID: s.Templates[0].ID, Name: "模块一", Content: "型号,数量,资产前缀,网络\nRZ2805-48YS8CQ,1,NET,业务网\nKunTai R722 K2,2,SERV,业务网"}}, Infer: true}
	body, _ := json.Marshal(req)
	w := call(h, "POST", "/api/initialize/preview", string(body))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	unchanged, _ := store.Snapshot()
	if unchanged.Revision != s.Revision || len(unchanged.Devices) != 0 {
		t.Fatal("preview wrote data")
	}
	w = call(h, "POST", "/api/initialize/commit", string(body))
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	after, _ := store.Snapshot()
	if len(after.Devices) != 3 || len(after.Networks) != 1 || len(after.Links) != 2 {
		t.Fatal("incomplete batch")
	}
	w = call(h, "POST", "/api/initialize/commit", string(body))
	if w.Code != 409 {
		t.Fatal("duplicate commit accepted")
	}
	audits, _ := store.Audits()
	if len(audits) != 2 {
		t.Fatal("batch not single audited transaction")
	}
}

func TestUnifiedInitializationAPI(t *testing.T) {
	h, store := testAPI(t)
	before, _ := store.Snapshot()
	req := core.InitRequest{Revision: before.Revision, ProjectName: "总表API项目", SiteName: "总表API机房", DefaultTemplateID: before.Templates[0].ID, UnifiedCSV: "模块,型号,数量,资产编号,网络\n甲,RZ2805-48YS8CQ,1,API-SW,业务网\n乙,KunTai R722 K2,1,API-SRV,业务网\n", NetworksCSV: "网络名称,VLAN,网段,网关\n业务网,101,10.1.0.0/24,10.1.0.1\n", LinksCSV: "source,target,source_port,target_port,status\nAPI-SW,API-SRV,p1,eth0,confirmed\n"}
	b, _ := json.Marshal(req)
	w := call(h, "POST", "/api/initialize/preview", string(b))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var preview core.InitPreview
	if e := json.Unmarshal(w.Body.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	if preview.NewModules != 2 || preview.Devices != 2 || preview.Links != 1 {
		t.Fatal("bad preview")
	}
	w = call(h, "POST", "/api/initialize/commit", string(b))
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	after, _ := store.Snapshot()
	if after.Revision != before.Revision+1 || len(after.Projects) != len(before.Projects)+1 {
		t.Fatal("incomplete commit")
	}
	w = call(h, "POST", "/api/initialize/commit", string(b))
	if w.Code != 409 {
		t.Fatal("stale duplicate accepted")
	}
}
