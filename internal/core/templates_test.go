package core

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTemplateCaptureContainsOnlyLayout(t *testing.T) {
	s := Seed()
	m := s.Modules[0]
	s.Devices = append(s.Devices, testDevice(s, "private-device", 1))
	s.Devices[0].IP = "10.0.0.1"
	s.Devices[0].SN = "secret-serial"
	left := OrderedRacks(s, m.ID, "L")
	order := []string{left[1].ID, left[0].ID}
	for _, r := range left[2:] {
		order = append(order, r.ID)
	}
	if _, e := ReorderRacks(&s, m.ID, order, nil); e != nil {
		t.Fatal(e)
	}
	p, e := CaptureModulePlan(s, m.ID)
	if e != nil {
		t.Fatal(e)
	}
	if p.RackNames["L"][0] != left[1].Name {
		t.Fatal("order lost")
	}
	b, _ := json.Marshal(p)
	for _, secret := range []string{"private-device", "10.0.0.1", "secret-serial", m.ID, left[0].ID} {
		if strings.Contains(string(b), secret) {
			t.Fatal("template leaked asset or internal ID")
		}
	}
	before := Clone(s)
	copy := CopyModulePlan(p)
	copy.Name = "复用微模块"
	_, e = AddPlan(&s, Plan{SiteID: m.SiteID, Modules: []ModulePlan{copy}})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.Devices, before.Devices) {
		t.Fatal("template cloned devices")
	}
	newModule := s.Modules[len(s.Modules)-1]
	if OrderedRacks(s, newModule.ID, "L")[0].Name != p.RackNames["L"][0] {
		t.Fatal("reused order changed")
	}
	copy.RackNames["L"][0] = "modified"
	if p.RackNames["L"][0] == "modified" {
		t.Fatal("copy aliases source")
	}
}
func TestTemplateValidation(t *testing.T) {
	for _, change := range []func(*ModulePlan){func(p *ModulePlan) { p.UHeight = 48 }, func(p *ModulePlan) { p.RackCount = 0 }, func(p *ModulePlan) { p.Type = "other" }, func(p *ModulePlan) { p.RackNames = map[string][]string{"L": {"same", "same", "c", "d", "e"}} }, func(p *ModulePlan) {
		p.Layout = []InfraSlot{{Name: "空调", Kind: "ac", Side: "R", Placement: "inline", AfterRack: 0}}
	}, func(p *ModulePlan) {
		p.Layout = []InfraSlot{{Name: "供电", Kind: "power", Side: "L", Placement: "integrated", AfterRack: 0}}
	}} {
		p := CopyModulePlan(DefaultTemplates()[0].Plan)
		change(&p)
		if ValidateModulePlan(p) == nil {
			t.Fatal("invalid template accepted")
		}
	}
}
func TestTemplateRevisionAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	old, _ := store.Snapshot()
	old.SchemaVersion = 4
	old.Templates = nil
	tx, _ := store.db.Begin()
	if e = saveState(tx, old); e != nil {
		t.Fatal(e)
	}
	tx.Commit()
	store.db.Exec("DROP TABLE templates")
	store.Close()
	store, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	if len(s.Templates) != 3 || s.SchemaVersion != 5 || s.Revision != old.Revision+1 {
		t.Fatal("template migration failed")
	}
	if !reflect.DeepEqual(s.Modules, old.Modules) || !reflect.DeepEqual(s.Devices, old.Devices) {
		t.Fatal("migration changed assets")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "app-before-v5-*.db"))
	if len(files) != 1 {
		t.Fatal("missing migration backup")
	}
}
