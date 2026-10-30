package server

import (
	"aom/internal/core"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (a *API) templates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int                 `json:"revision"`
		Template core.ModuleTemplate `json:"template"`
		ModuleID string              `json:"moduleId"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "template."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		t := req.Template
		t.Name = strings.TrimSpace(t.Name)
		t.Description = strings.TrimSpace(t.Description)
		if r.Method == "DELETE" {
			for i, old := range s.Templates {
				if old.ID == r.PathValue("id") {
					s.Templates = append(s.Templates[:i], s.Templates[i+1:]...)
					return "删除布局模板：" + old.Name, nil
				}
			}
			return "", fmt.Errorf("模板不存在")
		}
		if req.ModuleID != "" {
			p, e := core.CaptureModulePlan(*s, req.ModuleID)
			if e != nil {
				return "", e
			}
			t.Plan = p
		}
		t.Plan.RoomPosition = nil
		t.Plan.RoomRotation = 0
		t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if r.Method == "POST" {
			t.ID = core.ID("template")
			s.Templates = append(s.Templates, t)
			return "保存布局模板：" + t.Name, nil
		}
		old := s.Template(r.PathValue("id"))
		if old == nil {
			return "", fmt.Errorf("模板不存在")
		}
		t.ID = old.ID
		*old = t
		return "修改布局模板：" + t.Name, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
