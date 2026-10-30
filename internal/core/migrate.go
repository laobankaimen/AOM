package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Migrate only the two recognizable initial rooms. User-created rooms and IDs
// remain intact. Device/rack IDs and all historical audit snapshots are preserved.
func UpgradeHierarchy(s *State) {
	s.SchemaVersion = 2
	s.Projects = []Project{{ID: "project-default", Name: "默认项目"}}
	for i := range s.Sites {
		s.Sites[i].ProjectID = "project-default"
	}
	primary, secondary := "", ""
	for _, site := range s.Sites {
		if site.Name == "点位 1 · 主机房" {
			primary = site.ID
		}
		if site.Name == "点位 2 · 综合机房" {
			secondary = site.ID
		}
	}
	if primary != "" && secondary != "" {
		name := "主机房"
		for _, site := range s.Sites {
			if site.Name == name && site.ID != primary {
				name = "点位1与点位2机房"
			}
		}
		for i := range s.Sites {
			if s.Sites[i].ID == primary {
				s.Sites[i].Name = name
			}
		}
		for i := range s.Modules {
			m := &s.Modules[i]
			if m.SiteID == primary {
				m.Point = "点位1"
				m.ProductModel = "曙光数创 C2000（SG6600）"
				m.Name = "点位1 · " + m.Name
			}
			if m.SiteID == secondary {
				m.SiteID = primary
				m.Point = "点位2"
				m.ProductModel = "曙光数创 C500（C500-40K）"
				m.Name = "点位2 · " + m.Name
			}
		}
		kept := []Site{}
		for _, site := range s.Sites {
			if site.ID != secondary {
				kept = append(kept, site)
			}
		}
		s.Sites = kept
	}
	for i := range s.Infra {
		v := &s.Infra[i]
		v.Side = "L"
		v.Placement = "inline"
		v.AfterRack = 0
		for _, m := range s.Modules {
			if m.ID != v.ModuleID {
				continue
			}
			if v.Kind == "ac" {
				v.AfterRack = m.RackLeft / 2
			}
			if v.Kind == "env" || strings.Contains(m.ProductModel, "C500") {
				v.Placement = "integrated"
				v.AfterRack = 1
			}
		}
	}
	// Remove the old module-as-server preset only when unused; never discard assets.
	keptModels := []Model{}
	for _, m := range s.Models {
		used := false
		for _, d := range s.Devices {
			if d.ModelID == m.ID {
				used = true
				break
			}
		}
		if m.ID != "model-c500" || used {
			keptModels = append(keptModels, m)
		}
	}
	s.Models = keptModels
}

func (s *Store) migrateHierarchy(path string) error {
	// SQLite produces a consistent standalone backup, including committed data.
	backup := filepath.Join(filepath.Dir(path), "app-before-v5-"+time.Now().UTC().Format("20060102T150405.000000000")+".db")
	if _, err := s.db.Exec("VACUUM INTO ?", backup); err != nil {
		return fmt.Errorf("升级前备份失败：%w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old, err := loadState(tx)
	if err != nil {
		return err
	}
	if old.SchemaVersion < 1 || old.SchemaVersion > 4 {
		return fmt.Errorf("升级版本冲突")
	}
	next := Clone(old)
	if old.SchemaVersion == 1 {
		UpgradeHierarchy(&next)
	}
	next.SchemaVersion = 5
	next.Templates = DefaultTemplates()
	if old.SchemaVersion < 3 {
		next.Networks = []Network{}
		next.Links = []Link{}
	}
	next.Revision++
	if err = Validate(&next); err != nil {
		return fmt.Errorf("升级校验失败：%w", err)
	}
	if err = saveState(tx, next); err != nil {
		return err
	}
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(next)
	_, err = tx.Exec("INSERT INTO audits(time,action,summary,revision,before_json,after_json) VALUES(?,?,?,?,?,?)", time.Now().UTC().Format(time.RFC3339), "schema.upgrade", "升级空间与网络数据结构，保留资产、机位和历史审计", next.Revision, string(before), string(after))
	if err != nil {
		return err
	}
	return tx.Commit()
}
