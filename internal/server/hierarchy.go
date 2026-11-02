package server

import (
	"aom/internal/core"
	"fmt"
	"net/http"
	"strings"
)

func (a *API) project(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "project.update", func(s *core.State) (string, error) {
		name := strings.TrimSpace(req.Name)
		for _, p := range s.Projects {
			if p.ID != r.PathValue("id") && p.Name == name {
				return "", fmt.Errorf("项目名称已存在")
			}
		}
		for i := range s.Projects {
			if s.Projects[i].ID == r.PathValue("id") {
				s.Projects[i].Name = name
				return "更新项目：" + name, nil
			}
		}
		return "", fmt.Errorf("项目不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) site(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int    `json:"revision"`
		Name      string `json:"name"`
		ProjectID string `json:"projectId"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "site.update", func(s *core.State) (string, error) {
		name := strings.TrimSpace(req.Name)
		for _, site := range s.Sites {
			if site.ID != r.PathValue("id") && site.ProjectID == req.ProjectID && site.Name == name {
				return "", fmt.Errorf("项目内机房名称已存在")
			}
		}
		for i := range s.Sites {
			if s.Sites[i].ID == r.PathValue("id") {
				s.Sites[i].Name = name
				s.Sites[i].ProjectID = req.ProjectID
				return "更新机房：" + name, nil
			}
		}
		return "", fmt.Errorf("机房不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) module(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision     int    `json:"revision"`
		Name         string `json:"name"`
		SiteID       string `json:"siteId"`
		ProductModel string `json:"productModel"`
		Point        string `json:"point"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "module.update", func(s *core.State) (string, error) {
		name := strings.TrimSpace(req.Name)
		for _, m := range s.Modules {
			if m.ID != r.PathValue("id") && m.SiteID == req.SiteID && m.Name == name {
				return "", fmt.Errorf("机房内微模块名称已存在")
			}
		}
		for i := range s.Modules {
			if s.Modules[i].ID == r.PathValue("id") {
				m := &s.Modules[i]
				m.Name = name
				if m.SiteID != req.SiteID {
					m.RoomPosition = nil
				}
				m.SiteID = req.SiteID
				m.ProductModel = strings.TrimSpace(req.ProductModel)
				m.Point = strings.TrimSpace(req.Point)
				return "更新微模块：" + name, nil
			}
		}
		return "", fmt.Errorf("微模块不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) infrastructure(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int        `json:"revision"`
		Infra    core.Infra `json:"infra"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "infra."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		v := req.Infra
		v.Name = strings.TrimSpace(v.Name)
		if r.Method == "POST" {
			count := 0
			for _, i := range s.Infra {
				if i.ModuleID == v.ModuleID {
					count++
				}
			}
			if count >= 48 {
				return "", fmt.Errorf("每模块基础设施最多48项")
			}
			v.ID = core.ID("infra")
			s.Infra = append(s.Infra, v)
			return "新增基础设施：" + v.Name, nil
		}
		for i, old := range s.Infra {
			if old.ID == r.PathValue("id") {
				if r.Method == "DELETE" {
					s.Infra = append(s.Infra[:i], s.Infra[i+1:]...)
					return "删除基础设施：" + old.Name, nil
				}
				v.ID = old.ID
				v.ModuleID = old.ModuleID
				v.Order = old.Order
				s.Infra[i] = v
				return "调整基础设施：" + v.Name, nil
			}
		}
		return "", fmt.Errorf("基础设施不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

func (a *API) rackOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int      `json:"revision"`
		Left     []string `json:"left"`
		Right    []string `json:"right"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "rack.reorder", func(s *core.State) (string, error) {
		return core.ReorderRacks(s, r.PathValue("id"), req.Left, req.Right)
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) rackDetails(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "rack.update", func(s *core.State) (string, error) {
		rack := s.Rack(r.PathValue("id"))
		if rack == nil {
			return "", fmt.Errorf("机柜不存在")
		}
		name := strings.TrimSpace(req.Name)
		for _, other := range s.Racks {
			if other.ID != rack.ID && other.ModuleID == rack.ModuleID && other.Name == name {
				return "", fmt.Errorf("本模块机柜名称已存在")
			}
		}
		rack.Name = name
		return "修改机柜名称：" + name, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

func (a *API) roomPosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		Rotation *int              `json:"rotation"`
		Position *core.CanvasPoint `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "module.position", func(s *core.State) (string, error) {
		return core.SetRoomTransform(s, r.PathValue("id"), req.Position, req.Rotation)
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) infraPlacement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int    `json:"revision"`
		Side      string `json:"side"`
		AfterRack int    `json:"afterRack"`
		BeforeID  string `json:"beforeId"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "infra.move", func(s *core.State) (string, error) {
		return core.MoveInlineInfrastructure(s, r.PathValue("id"), req.Side, req.AfterRack, req.BeforeID)
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

func (a *API) rackInsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		RackID   string `json:"rackId"`
		Side     string `json:"side"`
		BeforeID string `json:"beforeId"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "rack.reorder", func(s *core.State) (string, error) {
		return core.InsertRack(s, r.PathValue("id"), req.RackID, req.Side, req.BeforeID)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}

func (a *API) rackCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		Side     string `json:"side"`
		Name     string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "rack.create", func(s *core.State) (string, error) { return core.AddRack(s, r.PathValue("id"), req.Side, req.Name) })
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
func (a *API) rackDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int `json:"revision"`
		core.SpaceDeleteOptions
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "rack.delete", func(s *core.State) (string, error) {
		return core.DeleteRackWithContents(s, r.PathValue("id"), req.SpaceDeleteOptions)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
func (a *API) moduleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int `json:"revision"`
		core.SpaceDeleteOptions
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "module.delete", func(s *core.State) (string, error) {
		return core.DeleteModuleWithContents(s, r.PathValue("id"), req.SpaceDeleteOptions)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}

func (a *API) roomEntrance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int                `json:"revision"`
		Entrance *core.RoomEntrance `json:"entrance"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "site.entrance", func(s *core.State) (string, error) {
		for i := range s.Sites {
			if s.Sites[i].ID == r.PathValue("id") {
				s.Sites[i].Entrance = req.Entrance
				return "设置机房入口：" + s.Sites[i].Name, nil
			}
		}
		return "", fmt.Errorf("机房不存在")
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
