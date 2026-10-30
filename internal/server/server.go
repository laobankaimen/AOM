package server

import (
	"aom/internal/core"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"modernc.org/sqlite"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type API struct{ store *core.Store }

func newApplication(store *core.Store, web fs.FS) http.Handler {
	a := &API{store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"status": "ok", "version": "0.7.0", "mode": "local"})
	})
	mux.HandleFunc("GET /api/state", a.state)
	mux.HandleFunc("POST /api/networks", a.network)
	mux.HandleFunc("POST /api/networks/{id}/move", a.networkMove)
	mux.HandleFunc("POST /api/networks/{id}/layout-position", a.networkLayoutPosition)
	mux.HandleFunc("POST /api/networks/{id}/position", a.networkPosition)
	mux.HandleFunc("POST /api/network-members", a.networkMember)
	mux.HandleFunc("PUT /api/networks/{id}", a.network)
	mux.HandleFunc("PUT /api/networks/{id}/layers", a.networkLayers)
	mux.HandleFunc("POST /api/networks/{id}/layer-members", a.layerMembers)
	mux.HandleFunc("DELETE /api/networks/{id}", a.network)
	mux.HandleFunc("POST /api/links", a.link)
	mux.HandleFunc("POST /api/links/batch", a.batchLinks)
	mux.HandleFunc("POST /api/topology/auto-layout", a.autoLayout)
	mux.HandleFunc("POST /api/topology/device-positions", a.devicePositions)
	mux.HandleFunc("PUT /api/links/{id}", a.link)
	mux.HandleFunc("DELETE /api/links/{id}", a.link)
	mux.HandleFunc("POST /api/topology/preview", a.suggest)
	mux.HandleFunc("POST /api/topology/commit", a.suggest)
	mux.HandleFunc("POST /api/plans", a.plan)
	mux.HandleFunc("POST /api/initialize/preview", a.initialize)
	mux.HandleFunc("POST /api/initialize/commit", a.initialize)
	mux.HandleFunc("POST /api/templates", a.templates)
	mux.HandleFunc("PUT /api/templates/{id}", a.templates)
	mux.HandleFunc("DELETE /api/templates/{id}", a.templates)
	mux.HandleFunc("PUT /api/projects/{id}", a.project)
	mux.HandleFunc("PUT /api/sites/{id}", a.site)
	mux.HandleFunc("PUT /api/modules/{id}", a.module)
	mux.HandleFunc("DELETE /api/modules/{id}", a.moduleDelete)
	mux.HandleFunc("POST /api/modules/{id}/racks", a.rackCreate)
	mux.HandleFunc("DELETE /api/racks/{id}", a.rackDelete)
	mux.HandleFunc("POST /api/modules/{id}/rack-order", a.rackOrder)
	mux.HandleFunc("POST /api/modules/{id}/rack-insert", a.rackInsert)
	mux.HandleFunc("POST /api/modules/{id}/room-position", a.roomPosition)
	mux.HandleFunc("POST /api/infrastructure/{id}/placement", a.infraPlacement)
	mux.HandleFunc("PUT /api/racks/{id}", a.rackDetails)
	mux.HandleFunc("POST /api/infrastructure", a.infrastructure)
	mux.HandleFunc("PUT /api/infrastructure/{id}", a.infrastructure)
	mux.HandleFunc("DELETE /api/infrastructure/{id}", a.infrastructure)
	mux.HandleFunc("POST /api/models", a.model)
	mux.HandleFunc("PUT /api/models/{id}", a.model)
	mux.HandleFunc("DELETE /api/models/{id}", a.model)
	mux.HandleFunc("POST /api/devices", a.device)
	mux.HandleFunc("PUT /api/devices/{id}", a.device)
	mux.HandleFunc("DELETE /api/devices/{id}", a.device)
	mux.HandleFunc("POST /api/devices/{id}/placement", a.placement)
	mux.HandleFunc("POST /api/devices/{id}/topology-position", a.devicePosition)
	mux.HandleFunc("POST /api/import/preview", a.importCSV)
	mux.HandleFunc("POST /api/import/commit", a.importCSV)
	mux.HandleFunc("GET /api/export/devices.csv", a.exportCSV)
	mux.HandleFunc("GET /api/export/plan.json", a.exportPlan)
	mux.HandleFunc("GET /api/audits", a.audits)
	mux.HandleFunc("POST /api/audits/compact", a.compactAudits)
	mux.HandleFunc("GET /api/audits/{id}", a.audit)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 404, map[string]string{"message": "API不存在或请求方法不支持"})
	})
	files := http.FileServer(http.FS(web))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			reply(w, 405, map[string]string{"message": "请求方法不支持"})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(web, path); err != nil {
			if strings.Contains(path, ".") {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/"
		}
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		// Same-origin mutations protect the local service from third-party websites.
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					reply(w, 403, map[string]string{"message": "仅允许同源页面提交变更"})
					return
				}
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				reply(w, 415, map[string]string{"message": "请求须为application/json"})
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
func reply(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 3*1024*1024)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(v); err != nil {
		reply(w, 400, map[string]string{"message": "请求JSON无效或超过3MiB限制"})
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		reply(w, 400, map[string]string{"message": "请求包含多余内容"})
		return false
	}
	return true
}
func failure(w http.ResponseWriter, err error) {
	if errors.Is(err, core.ErrPersistence) {
		reply(w, 503, map[string]string{"message": err.Error()})
		return
	}
	var storageError *sqlite.Error
	if errors.As(err, &storageError) {
		internal(w, err)
		return
	}
	code := 400
	if errors.Is(err, core.ErrConflict) {
		code = 409
	}
	reply(w, code, map[string]string{"message": err.Error()})
}
func internal(w http.ResponseWriter, err error) {
	log.Printf("request error: %v", err)
	reply(w, 500, map[string]string{"message": "存储操作失败，请检查后台日志"})
}
func (a *API) state(w http.ResponseWriter, r *http.Request) {
	s, e := a.store.Snapshot()
	if e != nil {
		internal(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) plan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int       `json:"revision"`
		Plan     core.Plan `json:"plan"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "plan.create", func(s *core.State) (string, error) { return core.AddPlan(s, req.Plan) })
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 201, s)
}
func (a *API) model(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int        `json:"revision"`
		Model    core.Model `json:"model"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "model."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		m := req.Model
		m.Name = strings.TrimSpace(m.Name)
		if r.Method == "POST" {
			m.ID = core.ID("model")
			s.Models = append(s.Models, m)
			return "新增型号：" + m.Name, nil
		}
		existing := s.Model(r.PathValue("id"))
		if existing == nil {
			return "", fmt.Errorf("型号不存在")
		}
		name := existing.Name
		if r.Method == "DELETE" {
			for _, d := range s.Devices {
				if d.ModelID == existing.ID {
					return "", fmt.Errorf("型号已被设备引用，不能删除")
				}
			}
			for i, v := range s.Models {
				if v.ID == existing.ID {
					s.Models = append(s.Models[:i], s.Models[i+1:]...)
					break
				}
			}
			return "删除型号：" + name, nil
		}
		m.ID = existing.ID
		*existing = m
		return "修改型号：" + m.Name, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) device(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int         `json:"revision"`
		Device    core.Device `json:"device"`
		AutoPlace bool        `json:"autoPlace"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "device."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		d := req.Device
		d.Name = strings.TrimSpace(d.Name)
		d.AssetTag = strings.TrimSpace(d.AssetTag)
		d.SN = strings.TrimSpace(d.SN)
		d.IP = strings.TrimSpace(d.IP)
		if r.Method == "POST" {
			d.ID = core.ID("dev")
			d.Source = "manual"
			if d.Status == "" {
				d.Status = "pending"
			}
			if req.AutoPlace {
				d.RackID = ""
				d.StartU = 0
				d.Status = "pending"
				core.AutoPlace(s, &d)
			}
			s.Devices = append(s.Devices, d)
			return "新增设备：" + d.AssetTag, nil
		}
		existing := s.Device(r.PathValue("id"))
		if existing == nil {
			return "", fmt.Errorf("设备不存在")
		}
		tag := existing.AssetTag
		if r.Method == "DELETE" {
			for _, l := range s.Links {
				if l.SourceID == existing.ID || l.TargetID == existing.ID {
					return "", fmt.Errorf("设备仍有链路，请先删除或调整连接")
				}
			}
			for _, n := range s.Networks {
				if core.Contains(n.DeviceIDs, existing.ID) {
					return "", fmt.Errorf("设备仍属于网络分区，请先移除网络成员关系")
				}
			}
			for i, v := range s.Devices {
				if v.ID == existing.ID {
					s.Devices = append(s.Devices[:i], s.Devices[i+1:]...)
					break
				}
			}
			return "删除设备：" + tag, nil
		}
		d.ID = existing.ID
		d.Source = existing.Source
		d.TopologyPosition = existing.TopologyPosition
		*existing = d
		return "修改设备：" + d.AssetTag, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) placement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		RackID   string `json:"rackId"`
		StartU   int    `json:"startU"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "device.place", func(s *core.State) (string, error) {
		if err := core.Place(s, r.PathValue("id"), req.RackID, req.StartU); err != nil {
			return "", err
		}
		d := s.Device(r.PathValue("id"))
		where := "待上架区"
		if rack := s.Rack(d.RackID); rack != nil {
			where = fmt.Sprintf("%s / U%d", rack.Name, d.StartU)
		}
		return "设备移位：" + d.AssetTag + " → " + where, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) importCSV(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		ModuleID string `json:"moduleId"`
		Content  string `json:"content"`
	}
	if !decode(w, r, &req) {
		return
	}
	if strings.HasSuffix(r.URL.Path, "preview") {
		s, e := a.store.Snapshot()
		if e != nil {
			internal(w, e)
			return
		}
		if s.Revision != req.Revision {
			failure(w, core.ErrConflict)
			return
		}
		p, e := core.PreviewCSV(s, req.ModuleID, req.Content)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, p)
		return
	}
	s, e := a.store.Apply(req.Revision, "device.import", func(s *core.State) (string, error) {
		p, e := core.PreviewCSV(*s, req.ModuleID, req.Content)
		if e != nil {
			return "", e
		}
		for _, row := range p.Rows {
			s.Devices = append(s.Devices, row.Device)
		}
		return fmt.Sprintf("CSV导入%d台：已上架%d台、待上架%d台", p.Total, p.Placed, p.Pending), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 201, s)
}
func (a *API) exportCSV(w http.ResponseWriter, r *http.Request) {
	s, e := a.store.Snapshot()
	if e != nil {
		internal(w, e)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="aom-devices.csv"`)
	_, _ = io.WriteString(w, core.ExportCSV(s))
}
func (a *API) exportPlan(w http.ResponseWriter, r *http.Request) {
	s, e := a.store.Snapshot()
	if e != nil {
		internal(w, e)
		return
	}
	siteID := r.URL.Query().Get("siteId")
	p := core.Plan{Modules: []core.ModulePlan{}}
	for _, site := range s.Sites {
		if site.ID == siteID {
			p.SiteName = site.Name
			for _, project := range s.Projects {
				if project.ID == site.ProjectID {
					p.ProjectName = project.Name
				}
			}
		}
	}
	if p.SiteName == "" {
		reply(w, 404, map[string]string{"message": "机房不存在"})
		return
	}
	for _, m := range s.Modules {
		if m.SiteID == siteID {
			mp := core.ModulePlan{RoomRotation: m.RoomRotation, RoomPosition: m.RoomPosition, RackNames: map[string][]string{"L": {}, "R": {}}, ProductModel: m.ProductModel, Point: m.Point, Layout: []core.InfraSlot{}, Name: m.Name, Type: m.Type, RackLeft: m.RackLeft, RackRight: m.RackRight, UHeight: 42}
			if m.Type == "single" {
				mp.RackCount = m.RackLeft
			}
			for _, side := range []string{"L", "R"} {
				for _, rack := range core.OrderedRacks(s, m.ID, side) {
					mp.RackNames[side] = append(mp.RackNames[side], rack.Name)
				}
			}
			infra := core.InfraPlan{}
			for _, slot := range s.Infra {
				if slot.ModuleID == m.ID {
					mp.Layout = append(mp.Layout, core.InfraSlot{Kind: slot.Kind, Name: slot.Name, Side: slot.Side, AfterRack: slot.AfterRack, Placement: slot.Placement, Order: slot.Order})
					switch slot.Kind {
					case "power":
						infra.Power++
					case "ac":
						infra.AC++
					case "env":
						infra.Env++
					}
				}
			}
			if len(p.Modules) == 0 {
				p.Infra = infra
			}
			if infra != p.Infra {
				mp.Infra = &infra
			}
			p.Modules = append(p.Modules, mp)
		}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="aom-plan.json"`)
	reply(w, 200, p)
}
func (a *API) audits(w http.ResponseWriter, r *http.Request) {
	v, e := a.store.Audits()
	if e != nil {
		internal(w, e)
		return
	}
	reply(w, 200, v)
}
func (a *API) audit(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil {
		reply(w, 404, map[string]string{"message": "快照不存在"})
		return
	}
	v, e := a.store.Audit(id)
	if errors.Is(e, sql.ErrNoRows) {
		reply(w, 404, map[string]string{"message": "快照不存在"})
		return
	}
	if e != nil {
		internal(w, e)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="audit-%d.json"`, id))
	reply(w, 200, v)
}

func (a *API) compactAudits(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int `json:"revision"`
	}
	if !decode(w, r, &req) {
		return
	}
	result, e := a.store.CompactAudits(req.Revision)
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, result)
}
