package server

import (
	"aom/internal/core"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
)

func (a *API) deviceRoles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		Roles    []core.DeviceRole `json:"roles"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, err := a.store.Apply(req.Revision, "role.update", func(s *core.State) (string, error) {
		// Built-in entries are always retained by RoleDefinitions; referenced custom roles may not disappear.
		s.DeviceRoles = req.Roles
		return "更新设备角色", core.ValidateDeviceRoles(s)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, s)
}
func (a *API) importModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int    `json:"revision"`
		Content  string `json:"content"`
	}
	if !decode(w, r, &req) {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/preview") {
		s, err := a.store.Snapshot()
		if err != nil {
			internal(w, err)
			return
		}
		if s.Revision != req.Revision {
			failure(w, core.ErrConflict)
			return
		}
		preview, err := core.PreviewModels(s, req.Content)
		if err != nil {
			failure(w, err)
			return
		}
		reply(w, 200, preview)
		return
	}
	s, err := a.store.Apply(req.Revision, "model.import", func(s *core.State) (string, error) {
		preview, err := core.PreviewModels(*s, req.Content)
		if err != nil {
			return "", err
		}
		for _, row := range preview.Rows {
			if !row.Existing {
				s.Models = append(s.Models, row.Model)
			}
		}
		return fmt.Sprintf("导入型号：新增%d个，跳过%d个相同定义", preview.Added, preview.Skipped), nil
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, s)
}
func (a *API) exportModels(w http.ResponseWriter, r *http.Request) {
	s, err := a.store.Snapshot()
	if err != nil {
		internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="models.csv"`)
	_, _ = w.Write([]byte("\xef\xbb\xbf"))
	out := csv.NewWriter(w)
	_ = out.Write([]string{"型号", "厂商", "设备角色", "U高", "端口说明"})
	names := map[string]string{}
	for _, role := range s.RoleDefinitions() {
		names[role.ID] = role.Name
	}
	for _, m := range s.Models {
		row := []string{m.Name, m.Vendor, names[m.Role], fmt.Sprint(m.UHeight), m.Ports}
		for i, value := range row {
			trim := strings.TrimLeft(value, " \t\r\n")
			if trim != "" && strings.ContainsAny(trim[:1], "=+-@") {
				row[i] = "'" + value
			}
		}
		_ = out.Write(row)
	}
	out.Flush()
}
