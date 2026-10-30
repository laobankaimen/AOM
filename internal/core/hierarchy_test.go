package core

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectRoomModuleHierarchy(t *testing.T) {
	s := Seed()
	room := s.Sites[0]
	if room.ProjectID != s.Projects[0].ID {
		t.Fatal("room has no project")
	}
	for _, m := range s.Modules {
		if m.SiteID != room.ID {
			t.Fatal("point modules split across rooms")
		}
	}
	if s.Modules[0].ProductModel != "曙光数创 C2000（SG6600）" || s.Modules[1].Type != "double" || s.Modules[2].ProductModel != "曙光数创 C500（C500-40K）" {
		t.Fatal("wrong module templates")
	}
	if s.Model("model-c500") != nil {
		t.Fatal("micro module incorrectly seeded as server")
	}
	_, err := AddPlan(&s, Plan{ProjectName: "项目二", SiteName: room.Name, Modules: []ModulePlan{{Name: "模块", Type: "single", RackCount: 1, UHeight: 42}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Projects) != 2 || len(s.Sites) != 2 {
		t.Fatal("new project was not created")
	}
	_, err = AddPlan(&s, Plan{ProjectID: s.Projects[1].ID, SiteID: room.ID, Modules: []ModulePlan{{Name: "wrong", Type: "single", RackCount: 1, UHeight: 42}}})
	if err == nil {
		t.Fatal("room/project mismatch allowed")
	}
}
func TestInfraRowPlacementValidation(t *testing.T) {
	for _, tc := range []struct {
		name, side, placement string
		after                 int
		valid                 bool
	}{{"start", "L", "inline", 0, true}, {"end", "L", "inline", 5, true}, {"integrated", "L", "integrated", 1, true}, {"invalid right", "R", "inline", 0, false}, {"outside", "L", "inline", 6, false}, {"missing cabinet", "L", "integrated", 0, false}, {"invalid placement", "L", "unknown", 0, false}} {
		t.Run(tc.name, func(t *testing.T) {
			s := Seed()
			s.Infra = append(s.Infra, Infra{ID: ID("infra"), ModuleID: s.Modules[0].ID, Name: "测试设施", Kind: "ac", Side: tc.side, Placement: tc.placement, AfterRack: tc.after})
			err := Validate(&s)
			if (err == nil) != tc.valid {
				t.Fatalf("got %v", err)
			}
		})
	}
}
func TestHierarchyMigrationPreservesAssetsAndAudits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := store.Snapshot()
	s.SchemaVersion = 1
	s.Projects = nil
	s.Sites[0].Name = "点位 1 · 主机房"
	s.Sites[0].ProjectID = ""
	second := Site{ID: "old-site2", Name: "点位 2 · 综合机房"}
	s.Sites = append(s.Sites, second, Site{ID: "custom-room", Name: "自建机房"})
	s.Modules[0].Name = "单排微模块"
	s.Modules[1].Name = "双排微模块"
	s.Modules[2].Name = "综合微模块"
	s.Modules[2].SiteID = second.ID
	for i := range s.Modules {
		s.Modules[i].Point = ""
		s.Modules[i].ProductModel = ""
	}
	for i := range s.Infra {
		s.Infra[i].Side = ""
		s.Infra[i].Placement = ""
		s.Infra[i].AfterRack = 0
	}
	d := testDevice(s, "asset-preserved", 41)
	d.SN = "SN-keep"
	s.Devices = append(s.Devices, d)
	legacyModel := Model{ID: "model-c500", Name: "曙光创 C500", Role: "server", UHeight: 0}
	s.Models = append(s.Models, legacyModel)
	s.Devices = append(s.Devices, Device{ID: "legacy-c500-device", AssetTag: "legacy-c500", Name: "既有资产保留", ModelID: legacyModel.ID, ModuleID: s.Modules[2].ID, Status: "pending"})
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = saveState(tx, s); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	auditsBefore, _ := store.Audits()
	auditBefore, _ := store.Audit(auditsBefore[0].ID)
	store.Close()
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if after.SchemaVersion != 5 || after.Revision != s.Revision+1 || len(after.Sites) != 2 || len(after.Projects) != 1 {
		t.Fatal("wrong migration state")
	}
	if !reflect.DeepEqual(s.Devices, after.Devices) || !reflect.DeepEqual(s.Racks, after.Racks) {
		t.Fatal("migration changed assets or rack placements")
	}
	if after.Model("model-c500") == nil {
		t.Fatal("in-use legacy model deleted")
	}
	for _, m := range after.Modules {
		if m.SiteID != after.Sites[0].ID {
			t.Fatal("default rooms not merged")
		}
	}
	if after.Sites[1].ID != "custom-room" {
		t.Fatal("custom room changed")
	}
	auditAfter, _ := store.Audit(auditsBefore[0].ID)
	if !reflect.DeepEqual(auditBefore, auditAfter) {
		t.Fatal("historical audit modified")
	}
	entries, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "app-before-v5-*.db"))
	if len(entries) != 1 {
		t.Fatal("migration backup missing")
	}
	backup, err := sql.Open("sqlite", entries[0])
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err = backup.QueryRow("SELECT schema_version FROM meta").Scan(&version); err != nil || version != 1 {
		t.Fatal("backup not version1")
	}
	backup.Close()
	revision := after.Revision
	store.Close()
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	after, _ = store.Snapshot()
	if after.Revision != revision {
		t.Fatal("migration reran")
	}
	entries, _ = filepath.Glob(filepath.Join(filepath.Dir(path), "app-before-v5-*.db"))
	if len(entries) != 1 {
		t.Fatal("duplicate backup")
	}
}
func TestMigrationFailureRollsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := store.Snapshot()
	s.SchemaVersion = 1
	s.Projects = nil
	s.Models[0].UHeight = 80
	tx, _ := store.db.Begin()
	if err = saveState(tx, s); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	store.Close()
	store, err = Open(path)
	if err == nil {
		store.Close()
		t.Fatal("corrupt migration succeeded")
	}
	db, _ := sql.Open("sqlite", path)
	defer db.Close()
	var version int
	db.QueryRow("SELECT schema_version FROM meta").Scan(&version)
	if version != 1 {
		t.Fatal("failed migration updated version")
	}
	var payload string
	db.QueryRow("SELECT payload FROM models WHERE id=?", s.Models[0].ID).Scan(&payload)
	var m Model
	json.Unmarshal([]byte(payload), &m)
	if m.UHeight != 80 {
		t.Fatal("partial state committed")
	}
	files, _ := os.ReadDir(filepath.Dir(path))
	if len(files) < 2 {
		t.Fatal("no recovery backup")
	}
}
