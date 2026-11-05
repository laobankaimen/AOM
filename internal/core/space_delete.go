package core

import "fmt"

type SpaceDeleteOptions struct {
	Mode             string `json:"mode"`
	TargetModuleID   string `json:"targetModuleId"`
	ConfirmName      string `json:"confirmName"`
	RemoveFacilities bool   `json:"removeFacilities"`
}

func moduleByID(s *State, id string) *Module {
	for i := range s.Modules {
		if s.Modules[i].ID == id {
			return &s.Modules[i]
		}
	}
	return nil
}
func moduleProject(s *State, id string) string {
	m := moduleByID(s, id)
	if m == nil {
		return ""
	}
	for _, site := range s.Sites {
		if site.ID == m.SiteID {
			return site.ProjectID
		}
	}
	return ""
}
func removeDeviceContents(s *State, ids map[string]bool) {
	devices := s.Devices[:0]
	for _, d := range s.Devices {
		if !ids[d.ID] {
			devices = append(devices, d)
		}
	}
	s.Devices = devices
	links := s.Links[:0]
	for _, l := range s.Links {
		if !ids[l.SourceID] && !ids[l.TargetID] {
			links = append(links, l)
		}
	}
	s.Links = links
	for i := range s.Networks {
		n := &s.Networks[i]
		members := n.DeviceIDs[:0]
		for _, id := range n.DeviceIDs {
			if !ids[id] {
				members = append(members, id)
			}
		}
		n.DeviceIDs = members
		for id := range ids {
			delete(n.DevicePositions, id)
			delete(n.DeviceLayers, id)
		}
	}
}
func DeleteModuleWithContents(s *State, id string, o SpaceDeleteOptions) (string, error) {
	if o.Mode == "" || o.Mode == "empty" {
		return DeleteModule(s, id)
	}
	m := moduleByID(s, id)
	if m == nil {
		return "", fmt.Errorf("微模块不存在，请刷新后重试")
	}
	name := m.Name
	ids := map[string]bool{}
	for _, d := range s.Devices {
		if d.ModuleID == id {
			ids[d.ID] = true
		}
	}
	switch o.Mode {
	case "migrate":
		target := moduleByID(s, o.TargetModuleID)
		if target == nil || target.ID == id || moduleProject(s, target.ID) != moduleProject(s, id) {
			return "", fmt.Errorf("请选择同一项目中的其他微模块接收设备")
		}
		for i := range s.Devices {
			d := &s.Devices[i]
			if ids[d.ID] {
				d.ModuleID = target.ID
				d.RackID = ""
				d.StartU = 0
				if d.Status != "maintenance" {
					d.Status = "pending"
				}
			}
		}
	case "cascade":
		if o.ConfirmName != name {
			return "", fmt.Errorf("请确认删除「%s」及其设备、关联连接后再提交", name)
		}
		removeDeviceContents(s, ids)
	default:
		return "", fmt.Errorf("请选择迁移设备或连同内容删除")
	}
	if _, err := DeleteModule(s, id); err != nil {
		return "", err
	}
	if o.Mode == "migrate" {
		return fmt.Sprintf("删除微模块：%s；%d台设备迁移至目标机房共享待上架区，保留网络和连接", name, len(ids)), nil
	}
	return fmt.Sprintf("删除微模块：%s；同时删除%d台设备及其关联连接，保留其他资产和网络", name, len(ids)), nil
}
func DeleteRackWithContents(s *State, id string, o SpaceDeleteOptions) (string, error) {
	if o.Mode == "" {
		return DeleteRack(s, id)
	}
	r := s.Rack(id)
	if r == nil {
		return "", fmt.Errorf("机柜不存在，请刷新后重试")
	}
	rack := *r
	m := moduleByID(s, rack.ModuleID)
	if m == nil {
		return "", fmt.Errorf("微模块不存在")
	}
	if m.RackLeft+m.RackRight <= 1 {
		return "", fmt.Errorf("这是微模块的最后一个机柜，请在当前确认框选择删除整个微模块，无需逐项清理")
	}
	if o.Mode != "uninstall" && o.Mode != "cascade" {
		return "", fmt.Errorf("请选择保留设备待上架或连同设备删除")
	}
	ids := map[string]bool{}
	for _, d := range s.Devices {
		if d.RackID == id {
			ids[d.ID] = true
		}
	}
	if o.Mode == "cascade" && o.ConfirmName != rack.Name {
		return "", fmt.Errorf("请确认删除机柜「%s」及柜内设备", rack.Name)
	}
	// Validate the facilities choice before changing any objects.
	for _, i := range s.Infra {
		if i.ModuleID == rack.ModuleID && i.Side == rack.Side && i.AfterRack == rack.Index && i.Placement == "integrated" && !o.RemoveFacilities {
			return "", fmt.Errorf("机柜有布局设施标记「%s」，请勾选一并删除标记或先迁移", i.Name)
		}
	}
	if o.Mode == "cascade" {
		removeDeviceContents(s, ids)
	} else {
		for i := range s.Devices {
			d := &s.Devices[i]
			if ids[d.ID] {
				d.RackID = ""
				d.StartU = 0
				if d.Status != "maintenance" {
					d.Status = "pending"
				}
			}
		}
	}
	infra := s.Infra[:0]
	for _, i := range s.Infra {
		if i.ModuleID == rack.ModuleID && i.Side == rack.Side && i.AfterRack == rack.Index && i.Placement == "integrated" {
			continue
		}
		infra = append(infra, i)
	}
	s.Infra = infra
	if len(OrderedRacks(*s, rack.ModuleID, rack.Side)) > 1 {
		if _, err := DeleteRack(s, id); err != nil {
			return "", err
		}
	} else {
		// A double row becomes a single row; retain all independent facility markers.
		departing := OrderedRowItems(*s, rack.ModuleID, rack.Side)
		remainingSide := "L"
		if rack.Side == "L" {
			remainingSide = "R"
		}
		count := len(OrderedRacks(*s, rack.ModuleID, remainingSide))
		tailOrder := 0
		for _, i := range s.Infra {
			if i.ModuleID == rack.ModuleID && i.Side == remainingSide && i.Order >= tailOrder {
				tailOrder = i.Order + 1
			}
		}
		racks := s.Racks[:0]
		for _, r := range s.Racks {
			if r.ID != id {
				if r.ModuleID == rack.ModuleID {
					r.Side = "L"
				}
				racks = append(racks, r)
			}
		}
		s.Racks = racks
		for i := range s.Infra {
			v := &s.Infra[i]
			if v.ModuleID != rack.ModuleID {
				continue
			}
			if v.Side == rack.Side {
				v.AfterRack = count
				for order, id := range departing {
					if v.ID == id {
						v.Order = tailOrder + order
						break
					}
				}
			}
			v.Side = "L"
		}
		m.Type = "single"
		m.RackLeft = count
		m.RackRight = 0
	}
	if err := Validate(s); err != nil {
		return "", err
	}
	if o.Mode == "cascade" {
		return fmt.Sprintf("删除机柜：%s；删除%d台设备及关联连接", rack.Name, len(ids)), nil
	}
	return fmt.Sprintf("删除机柜：%s；%d台设备保留在机房共享待上架区", rack.Name, len(ids)), nil
}
