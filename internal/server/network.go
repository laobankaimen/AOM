package server

import (
	"aom/internal/core"
	"fmt"
	"net/http"
	"strings"
)

func (a *API) network(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int          `json:"revision"`
		Network  core.Network `json:"network"`
		Detach   bool         `json:"detach"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "network."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		n := req.Network
		n.Name = strings.TrimSpace(n.Name)
		n.CIDR = strings.TrimSpace(n.CIDR)
		n.Gateway = strings.TrimSpace(n.Gateway)
		if n.DeviceIDs == nil {
			n.DeviceIDs = []string{}
		}
		if r.Method == "POST" {
			n.ID = core.ID("network")
			s.Networks = append(s.Networks, n)
			return "创建网络分区：" + n.Name, nil
		}
		for i, old := range s.Networks {
			if old.ID != r.PathValue("id") {
				continue
			}
			if r.Method == "DELETE" {
				for _, child := range s.Networks {
					if child.ParentID == old.ID {
						return "", fmt.Errorf("网络仍包含子网络，请先移动子网络")
					}
				}
				if !req.Detach && len(old.DeviceIDs) > 0 {
					return "", fmt.Errorf("网络仍有成员设备，请先移出成员")
				}
				for _, l := range s.Links {
					if !req.Detach && core.Contains(l.NetworkIDs, old.ID) {
						return "", fmt.Errorf("该分区被链路引用，请先调整链路归属")
					}
				}
				for j := range s.Links {
					s.Links[j].NetworkIDs = core.RemoveID(s.Links[j].NetworkIDs, old.ID)
				}
				s.Networks = append(s.Networks[:i], s.Networks[i+1:]...)
				return "删除网络分区：" + old.Name, nil
			}
			n.ID = old.ID
			n.ProjectID = old.ProjectID
			n.Layers = old.Layers
			n.DeviceLayers = old.DeviceLayers
			for id := range n.DeviceLayers {
				if !core.Contains(n.DeviceIDs, id) {
					delete(n.DeviceLayers, id)
				}
			}
			n.Position = old.Position
			n.DevicePositions = old.DevicePositions
			for id := range n.DevicePositions {
				if !core.Contains(n.DeviceIDs, id) {
					delete(n.DevicePositions, id)
				}
			}
			s.Networks[i] = n
			return "修改网络分区：" + n.Name, nil
		}
		return "", fmt.Errorf("网络分区不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) link(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int       `json:"revision"`
		Link     core.Link `json:"link"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "link."+strings.ToLower(r.Method), func(s *core.State) (string, error) {
		l := req.Link
		l.SourcePort = strings.TrimSpace(l.SourcePort)
		l.TargetPort = strings.TrimSpace(l.TargetPort)
		l.Medium = strings.TrimSpace(l.Medium)
		if l.NetworkIDs == nil {
			l.NetworkIDs = []string{}
		}
		if r.Method == "POST" {
			l.ID = core.ID("link")
			l.Origin = "manual"
			s.Links = append(s.Links, l)
			return "新增设备链路", nil
		}
		for i, old := range s.Links {
			if old.ID != r.PathValue("id") {
				continue
			}
			if r.Method == "DELETE" {
				s.Links = append(s.Links[:i], s.Links[i+1:]...)
				return "删除设备链路", nil
			}
			l.ID = old.ID
			l.ProjectID = old.ProjectID
			l.Origin = old.Origin
			s.Links[i] = l
			return "修改设备链路", nil
		}
		return "", fmt.Errorf("链路不存在")
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) suggest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int    `json:"revision"`
		NetworkID string `json:"networkId"`
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
		links, e := core.SuggestLinks(s, req.NetworkID)
		if e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, map[string]any{"links": links, "revision": s.Revision})
		return
	}
	s, e := a.store.Apply(req.Revision, "link.infer", func(s *core.State) (string, error) {
		links, e := core.SuggestLinks(*s, req.NetworkID)
		if e != nil {
			return "", e
		}
		if len(links) == 0 {
			return "", fmt.Errorf("没有可新增的推演链路")
		}
		s.Links = append(s.Links, links...)
		return fmt.Sprintf("记录%d条待确认逻辑链路", len(links)), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

func (a *API) networkMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		ParentID string            `json:"parentId"`
		Position *core.CanvasPoint `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "network.move", func(s *core.State) (string, error) {
		n := s.Network(r.PathValue("id"))
		if n == nil {
			return "", fmt.Errorf("网络不存在")
		}
		n.ParentID = req.ParentID
		n.Position = req.Position
		return "移动子网络：" + n.Name, nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) networkMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int      `json:"revision"`
		DeviceID  string   `json:"deviceId"`
		DeviceIDs []string `json:"deviceIds"`
		SourceID  string   `json:"sourceId"`
		TargetID  string   `json:"targetId"`
		Mode      string   `json:"mode"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "network.member", func(s *core.State) (string, error) {
		ids := req.DeviceIDs
		if len(ids) == 0 {
			ids = []string{req.DeviceID}
		}
		if len(ids) > 2000 {
			return "", fmt.Errorf("一次最多调整2000台设备")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				return "", fmt.Errorf("设备选择重复，请刷新后重新选择")
			}
			seen[id] = true
			if _, err := core.MoveNetworkMember(s, id, req.SourceID, req.TargetID, req.Mode); err != nil {
				return "", err
			}
		}
		return fmt.Sprintf("调整%d台设备的网络归属", len(ids)), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) networkPosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		DeviceID string            `json:"deviceId"`
		Position *core.CanvasPoint `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "topology.position", func(s *core.State) (string, error) {
		n := s.Network(r.PathValue("id"))
		if n == nil || !core.Contains(n.DeviceIDs, req.DeviceID) {
			return "", fmt.Errorf("设备不属于该网络")
		}
		if n.DevicePositions == nil {
			n.DevicePositions = map[string]core.CanvasPoint{}
		}
		if req.Position == nil {
			delete(n.DevicePositions, req.DeviceID)
		} else {
			n.DevicePositions[req.DeviceID] = *req.Position
		}
		return "调整拓扑设备位置", nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}
func (a *API) devicePosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		Position *core.CanvasPoint `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "topology.position", func(s *core.State) (string, error) {
		d := s.Device(r.PathValue("id"))
		if d == nil {
			return "", fmt.Errorf("设备不存在")
		}
		d.TopologyPosition = req.Position
		return "调整未分区设备的图位置", nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

// Batch links record a proposed relationship, never fabricated physical ports.
func (a *API) batchLinks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int      `json:"revision"`
		NetworkID string   `json:"networkId"`
		Sources   []string `json:"sources"`
		Targets   []string `json:"targets"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "link.batch", func(s *core.State) (string, error) {
		n := s.Network(req.NetworkID)
		if n == nil {
			return "", fmt.Errorf("请先选择要建立连接的网络")
		}
		if len(req.Sources) == 0 || len(req.Targets) == 0 || len(req.Sources) > 8 || len(req.Targets) > 2000 || len(req.Sources)*len(req.Targets) > 2000 {
			return "", fmt.Errorf("请选择1–8台上联设备和下联设备，一次最多建立2000条关系")
		}
		for _, ids := range [][]string{req.Sources, req.Targets} {
			seen := map[string]bool{}
			for _, id := range ids {
				if seen[id] || !core.Contains(n.DeviceIDs, id) {
					return "", fmt.Errorf("请选择当前网络中的设备，且不能重复选择")
				}
				seen[id] = true
			}
		}
		count := 0
		for _, from := range req.Sources {
			for _, to := range req.Targets {
				if from == to {
					return "", fmt.Errorf("同一台设备不能同时作为上联和下联，请调整选择")
				}
				exists := false
				for _, l := range s.Links {
					if (l.SourceID == from && l.TargetID == to || l.SourceID == to && l.TargetID == from) && core.Contains(l.NetworkIDs, n.ID) {
						exists = true
						break
					}
				}
				if exists {
					continue
				}
				s.Links = append(s.Links, core.Link{ID: core.ID("link"), ProjectID: n.ProjectID, SourceID: from, TargetID: to, NetworkIDs: []string{n.ID}, Kind: "logical", Status: "pending", Origin: "manual", Note: "批量创建，实际连接与端口待确认"})
				count++
			}
		}
		if count == 0 {
			return "", fmt.Errorf("所选设备之间的关系已经存在，无需重复创建")
		}
		return fmt.Sprintf("批量建立%d条待确认连接：%s", count, n.Name), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

// Discard saved diagram coordinates only for the explicitly visible scope.
func (a *API) autoLayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision   int      `json:"revision"`
		ProjectID  string   `json:"projectId"`
		NetworkIDs []string `json:"networkIds"`
		DeviceIDs  []string `json:"deviceIds"`
	}
	if !decode(w, r, &req) {
		return
	}
	s, e := a.store.Apply(req.Revision, "topology.auto-layout", func(s *core.State) (string, error) {
		if len(req.DeviceIDs) == 0 || len(req.DeviceIDs) > 2000 || len(req.NetworkIDs) > 1000 {
			return "", fmt.Errorf("请选择1–2000台可见设备进行自动排列")
		}
		for _, id := range req.DeviceIDs {
			if s.Device(id) == nil || s.DeviceProject(id) != req.ProjectID {
				return "", fmt.Errorf("自动排列只能调整当前项目的设备")
			}
		}
		for _, id := range req.NetworkIDs {
			n := s.Network(id)
			if n == nil || n.ProjectID != req.ProjectID {
				return "", fmt.Errorf("自动排列的网络不属于当前项目")
			}
		}
		for _, networkID := range req.NetworkIDs {
			s.Network(networkID).Position = nil
		}
		for _, id := range req.DeviceIDs {
			for _, networkID := range req.NetworkIDs {
				delete(s.Network(networkID).DevicePositions, id)
			}
			assigned := false
			for _, n := range s.Networks {
				if core.Contains(n.DeviceIDs, id) {
					assigned = true
					break
				}
			}
			if !assigned {
				s.Device(id).TopologyPosition = nil
			}
		}
		return fmt.Sprintf("自动排列%d台设备；保留网络归属、链路和机位", len(req.DeviceIDs)), nil
	})
	if e != nil {
		failure(w, e)
		return
	}
	reply(w, 200, s)
}

func (a *API) networkLayoutPosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int               `json:"revision"`
		Position *core.CanvasPoint `json:"position"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "topology.network-position", func(s *core.State) (string, error) {
		network := s.Network(r.PathValue("id"))
		if network == nil {
			return "", fmt.Errorf("网络不存在，请刷新后重试")
		}
		if req.Position != nil && !core.ValidCanvasPoint(*req.Position) {
			return "", fmt.Errorf("网络图位置须在0–10000范围内")
		}
		network.Position = req.Position
		return "调整网络图位置：" + network.Name, nil
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}

func (a *API) devicePositions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int    `json:"revision"`
		ProjectID string `json:"projectId"`
		NetworkID string `json:"networkId"`
		Positions []struct {
			DeviceID string           `json:"deviceId"`
			Position core.CanvasPoint `json:"position"`
		} `json:"positions"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "topology.position", func(s *core.State) (string, error) {
		if len(req.Positions) < 1 || len(req.Positions) > 2000 {
			return "", fmt.Errorf("一次最多移动2000台设备，请展开后分批移动")
		}
		n := s.Network(req.NetworkID)
		if req.NetworkID != "" && (n == nil || n.ProjectID != req.ProjectID) {
			return "", fmt.Errorf("请选择当前项目中的网络")
		}
		seen := map[string]bool{}
		for _, p := range req.Positions {
			if seen[p.DeviceID] || s.Device(p.DeviceID) == nil || s.DeviceProject(p.DeviceID) != req.ProjectID || !core.ValidCanvasPoint(p.Position) {
				return "", fmt.Errorf("设备或图位置无效，请刷新后重试")
			}
			seen[p.DeviceID] = true
			if n != nil && !core.Contains(n.DeviceIDs, p.DeviceID) {
				return "", fmt.Errorf("设备不属于目标网络")
			}
			if n == nil {
				for _, network := range s.Networks {
					if core.Contains(network.DeviceIDs, p.DeviceID) {
						return "", fmt.Errorf("请选择设备所属的网络")
					}
				}
			}
		}
		for _, p := range req.Positions {
			if n != nil {
				if n.DevicePositions == nil {
					n.DevicePositions = map[string]core.CanvasPoint{}
				}
				n.DevicePositions[p.DeviceID] = p.Position
			} else {
				point := p.Position
				s.Device(p.DeviceID).TopologyPosition = &point
			}
		}
		return fmt.Sprintf("调整%d台设备的图位置", len(req.Positions)), nil
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}

func (a *API) networkLayers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision     int                 `json:"revision"`
		Layers       []core.NetworkLayer `json:"layers"`
		DeviceLayers map[string]string   `json:"deviceLayers"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "network.layers", func(s *core.State) (string, error) {
		n := s.Network(r.PathValue("id"))
		if n == nil {
			return "", fmt.Errorf("网络不存在")
		}
		oldLayers := append([]core.NetworkLayer(nil), core.NetworkLayers(n)...)
		oldAssignments := n.DeviceLayers
		effective := func(layers []core.NetworkLayer, assignments map[string]string, deviceID string) (string, int) {
			id := assignments[deviceID]
			if id == "" {
				role := ""
				if d := s.Device(deviceID); d != nil {
					if m := s.Model(d.ModelID); m != nil {
						role = m.Role
					}
				}
				id = map[string]string{"core": "core", "aggregation": "aggregation", "access": "access", "management": "access", "server": "compute", "storage": "compute"}[role]
			}
			for i, layer := range layers {
				if layer.ID == id {
					return id, i
				}
			}
			if len(layers) == 0 {
				return "", -1
			}
			return layers[len(layers)-1].ID, len(layers) - 1
		}
		n.Layers = req.Layers
		n.DeviceLayers = req.DeviceLayers
		for _, deviceID := range n.DeviceIDs {
			beforeID, beforeIndex := effective(oldLayers, oldAssignments, deviceID)
			afterID, afterIndex := effective(core.NetworkLayers(n), n.DeviceLayers, deviceID)
			if beforeID != afterID || beforeIndex != afterIndex {
				delete(n.DevicePositions, deviceID)
			}
		}
		return "设置网络显示层：" + n.Name, core.ValidateNetworkLayers(n)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
func (a *API) layerMembers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int      `json:"revision"`
		DeviceIDs []string `json:"deviceIds"`
		LayerID   string   `json:"layerId"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "network.layer-members", func(s *core.State) (string, error) {
		n := s.Network(r.PathValue("id"))
		if n == nil {
			return "", fmt.Errorf("网络不存在")
		}
		if len(req.DeviceIDs) < 1 || len(req.DeviceIDs) > 2000 {
			return "", fmt.Errorf("一次分配1–2000台设备")
		}
		if n.DeviceLayers == nil {
			n.DeviceLayers = map[string]string{}
		}
		for _, id := range req.DeviceIDs {
			if !core.Contains(n.DeviceIDs, id) {
				return "", fmt.Errorf("设备不属于当前网络")
			}
			if req.LayerID == "" {
				delete(n.DeviceLayers, id)
			} else {
				n.DeviceLayers[id] = req.LayerID
			}
			delete(n.DevicePositions, id)
		}
		return "调整设备显示层：" + n.Name, core.ValidateNetworkLayers(n)
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}

func (a *API) arrangedTopology(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int    `json:"revision"`
		ProjectID string `json:"projectId"`
		Groups    []struct {
			NetworkID string            `json:"networkId"`
			Position  *core.CanvasPoint `json:"position"`
			Positions []struct {
				DeviceID string           `json:"deviceId"`
				Position core.CanvasPoint `json:"position"`
			} `json:"positions"`
		} `json:"groups"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "topology.auto-layout", func(s *core.State) (string, error) {
		seen := map[string]bool{}
		count := 0
		if len(req.Groups) == 0 || len(req.Groups) > 1001 {
			return "", fmt.Errorf("请选择有效的拓扑范围")
		}
		for _, g := range req.Groups {
			n := s.Network(g.NetworkID)
			if g.NetworkID != "" && (n == nil || n.ProjectID != req.ProjectID || g.Position == nil || !core.ValidCanvasPoint(*g.Position)) {
				return "", fmt.Errorf("网络位置无效")
			}
			for _, p := range g.Positions {
				count++
				if count > 10000 || seen[p.DeviceID] || s.Device(p.DeviceID) == nil || s.DeviceProject(p.DeviceID) != req.ProjectID || !core.ValidCanvasPoint(p.Position) {
					return "", fmt.Errorf("设备位置无效")
				}
				seen[p.DeviceID] = true
				if n != nil && !core.Contains(n.DeviceIDs, p.DeviceID) {
					return "", fmt.Errorf("设备不属于当前网络")
				}
				if n == nil {
					for _, network := range s.Networks {
						if core.Contains(network.DeviceIDs, p.DeviceID) {
							return "", fmt.Errorf("设备已有网络归属")
						}
					}
				}
			}
		}
		for _, g := range req.Groups {
			n := s.Network(g.NetworkID)
			if n != nil {
				n.Position = g.Position
				if n.DevicePositions == nil {
					n.DevicePositions = map[string]core.CanvasPoint{}
				}
			}
			for _, p := range g.Positions {
				if n != nil {
					n.DevicePositions[p.DeviceID] = p.Position
				} else {
					point := p.Position
					s.Device(p.DeviceID).TopologyPosition = &point
				}
			}
		}
		return fmt.Sprintf("自动整理%d台设备的图位置", count), nil
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
