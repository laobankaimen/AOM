package server

import (
	"aom/internal/core"
	"fmt"
	"net/http"
	"strings"
)

func (a *API) initialize(w http.ResponseWriter, r *http.Request) {
	var req core.InitRequest
	if !decode(w, r, &req) {
		return
	}
	if strings.HasSuffix(r.URL.Path, "preview") {
		state, e := a.store.Snapshot()
		if e != nil {
			internal(w, e)
			return
		}
		if state.Revision != req.Revision {
			failure(w, core.ErrConflict)
			return
		}
		preview, e := core.PrepareInitialization(state, req)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, preview)
		return
	}
	state, e := a.store.Apply(req.Revision, "workspace.initialize", func(s *core.State) (string, error) {
		p, e := core.PrepareInitialization(*s, req)
		if e != nil {
			return "", e
		}
		*s = p.State
		return fmt.Sprintf("初始化%d模块、%d设备、%d网络、%d链路", p.NewModules, p.Devices, p.Networks, p.Links), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 201, state)
}
