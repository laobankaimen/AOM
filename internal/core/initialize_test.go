package core

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func initFixture(s State) InitRequest {
	return InitRequest{Revision: s.Revision, ProjectID: s.Projects[0].ID, SiteName: "批量初始化机房", Modules: []InitModule{{TemplateID: s.Templates[0].ID, Name: "计算模块", Content: "\ufeff型号,数量,名称,资产前缀,IP,SN,网络\nRZ2805-48YS8CQ,1,接入,SW,,,业务网\nKunTai R722 K2,3,计算,SRV,,,业务网;管理网\nRZ18001-1,1,核心,CORE,,,业务网\n"}}, Infer: true}
}
func TestInitializationPreviewAndInference(t *testing.T) {
	s := Seed()
	before := Clone(s)
	req := initFixture(s)
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("preview mutated input")
	}
	if p.NewModules != 1 || p.Devices != 5 || p.Placed != 4 || p.Pending != 1 || p.Networks != 2 || p.Links != 4 {
		t.Fatalf("unexpected totals: %+v", p)
	}
	for _, l := range p.State.Links {
		if l.Status != "pending" || l.SourcePort != "" || l.Origin != "inferred" {
			t.Fatal("inference invented facts")
		}
	}
	if len(p.Warnings) < 3 {
		t.Fatal("missing pending warnings")
	}
	if p.State.Modules[len(p.State.Modules)-1].ProductModel != s.Templates[0].Plan.ProductModel {
		t.Fatal("template not applied")
	}
}
func TestInitializationKnownLinks(t *testing.T) {
	s := Seed()
	req := initFixture(s)
	req.LinksCSV = "源资产编号,目标资产编号,源端口,目标端口,类型,速率,介质,网络,确认状态\nSW-0001,SRV-0001,Eth1,eth0,物理,25,光纤,业务网,已确认\n"
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	l := p.State.Links[0]
	if l.Status != "confirmed" || l.SpeedGbps != 25 || l.Kind != "physical" || l.SourcePort != "Eth1" {
		t.Fatal("known connection not preserved")
	}
	for _, kind := range []string{"NaN", "+Inf", "-1"} {
		bad := req
		bad.LinksCSV = strings.Replace(req.LinksCSV, ",25,", ","+kind+",", 1)
		if _, e = PrepareInitialization(s, bad); e == nil {
			t.Fatal("invalid speed accepted")
		}
	}
}
func TestInitializationInvalidBatchAtomic(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	req := initFixture(s)
	req.Modules = append(req.Modules, InitModule{TemplateID: s.Templates[0].ID, Name: "错误模块", Content: "model,quantity\nmissing-model,1"})
	_, e = store.Apply(s.Revision, "initialize", func(next *State) (string, error) {
		p, e := PrepareInitialization(*next, req)
		if e != nil {
			return "", e
		}
		*next = p.State
		return "initialize", nil
	})
	if e == nil {
		t.Fatal("invalid batch accepted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(s, after) {
		t.Fatal("partially created room or devices")
	}
	audits, _ := store.Audits()
	if len(audits) != 1 {
		t.Fatal("failed batch created audit")
	}
}
func TestInitializeExistingModuleAndConflicts(t *testing.T) {
	s := Seed()
	req := initFixture(s)
	req.SiteID = s.Sites[0].ID
	req.Modules[0].ModuleID = s.Modules[0].ID
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	if p.NewModules != 0 || len(p.State.Modules) != len(s.Modules) {
		t.Fatal("existing module duplicated")
	}
	if !reflect.DeepEqual(p.State.Racks, s.Racks) {
		t.Fatal("existing rack positions changed")
	}
	req.Modules = append(req.Modules, req.Modules[0])
	if _, e = PrepareInitialization(s, req); e == nil {
		t.Fatal("duplicate target accepted")
	}
	bad := initFixture(s)
	bad.LinksCSV = "source,target\nNOT-THERE,SRV-0001"
	if _, e = PrepareInitialization(s, bad); e == nil {
		t.Fatal("unresolved endpoint accepted")
	}
	bad = initFixture(s)
	bad.Modules[0].TemplateID = "missing"
	if _, e = PrepareInitialization(s, bad); e == nil {
		t.Fatal("unknown template accepted")
	}
}
