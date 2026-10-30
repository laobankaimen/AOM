package core

import (
	"fmt"
	"strings"
)

func AddRack(s *State, moduleID, side, name string) (string, error) {
	var module *Module
	for i := range s.Modules {
		if s.Modules[i].ID == moduleID {
			module = &s.Modules[i]
			break
		}
	}
	if module == nil {
		return "", fmt.Errorf("微模块不存在")
	}
	if side != "L" && side != "R" || module.Type == "single" && side != "L" {
		return "", fmt.Errorf("请选择微模块中存在的柜列")
	}
	count := module.RackLeft
	if side == "R" {
		count = module.RackRight
	}
	max := 12
	if module.Type == "single" {
		max = 24
	}
	if count >= max {
		return "", fmt.Errorf("该列最多%d个机柜", max)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		for i := count + 1; ; i++ {
			candidate := fmt.Sprintf("%s%02d", side, i)
			found := false
			for _, r := range s.Racks {
				if r.ModuleID == moduleID && r.Name == candidate {
					found = true
				}
			}
			if !found {
				name = candidate
				break
			}
		}
	}
	if !textOK(name, 80) {
		return "", fmt.Errorf("机柜名称须为1–80个字")
	}
	for _, r := range s.Racks {
		if r.ModuleID == moduleID && r.Name == name {
			return "", fmt.Errorf("微模块中已有机柜「%s」", name)
		}
	}
	s.Racks = append(s.Racks, Rack{ID: ID("rack"), ModuleID: moduleID, Name: name, Side: side, Index: count + 1, Capacity: 42})
	if side == "L" {
		module.RackLeft++
	} else {
		module.RackRight++
	}
	return "新增机柜：" + name, Validate(s)
}
func DeleteRack(s *State, id string) (string, error) {
	r := s.Rack(id)
	if r == nil {
		return "", fmt.Errorf("机柜不存在")
	}
	rack := *r
	for _, d := range s.Devices {
		if d.RackID == id {
			return "", fmt.Errorf("机柜「%s」仍有设备，请先移柜或下架后删除", rack.Name)
		}
	}
	for _, i := range s.Infra {
		if i.ModuleID == rack.ModuleID && i.Side == rack.Side && i.AfterRack == rack.Index && i.Placement == "integrated" {
			return "", fmt.Errorf("机柜「%s」仍有柜内设施「%s」，请先移动或删除设施", rack.Name, i.Name)
		}
	}
	row := OrderedRacks(*s, rack.ModuleID, rack.Side)
	if len(row) <= 1 {
		return "", fmt.Errorf("每列至少保留一个机柜；移除整个微模块请使用删除微模块")
	}
	inline := OrderedRowItems(*s, rack.ModuleID, rack.Side)
	for i, v := range s.Racks {
		if v.ID == id {
			s.Racks = append(s.Racks[:i], s.Racks[i+1:]...)
			break
		}
	}
	for i := range s.Racks {
		v := &s.Racks[i]
		if v.ModuleID == rack.ModuleID && v.Side == rack.Side && v.Index > rack.Index {
			v.Index--
		}
	}
	// Row objects keep their relative order when two neighbouring gaps join.

	for i := range s.Infra {
		v := &s.Infra[i]
		if v.ModuleID == rack.ModuleID && v.Side == rack.Side {
			if v.AfterRack >= rack.Index {
				v.AfterRack--
			}
			for order, id := range inline {
				if v.ID == id {
					v.Order = order
				}
			}
		}
	}
	for i := range s.Modules {
		m := &s.Modules[i]
		if m.ID == rack.ModuleID {
			if rack.Side == "L" {
				m.RackLeft--
			} else {
				m.RackRight--
			}
		}
	}
	return "删除空机柜：" + rack.Name, Validate(s)
}
func DeleteModule(s *State, id string) (string, error) {
	var name string
	for _, m := range s.Modules {
		if m.ID == id {
			name = m.Name
		}
	}
	if name == "" {
		return "", fmt.Errorf("微模块不存在")
	}
	for _, d := range s.Devices {
		if d.ModuleID == id {
			return "", fmt.Errorf("微模块「%s」仍有设备（含待上架），请先迁移设备再删除", name)
		}
	}
	racks := s.Racks[:0]
	for _, r := range s.Racks {
		if r.ModuleID != id {
			racks = append(racks, r)
		}
	}
	s.Racks = racks
	infra := s.Infra[:0]
	for _, i := range s.Infra {
		if i.ModuleID != id {
			infra = append(infra, i)
		}
	}
	s.Infra = infra
	modules := s.Modules[:0]
	for _, m := range s.Modules {
		if m.ID != id {
			modules = append(modules, m)
		}
	}
	s.Modules = modules
	return "删除空微模块及柜列设施：" + name, Validate(s)
}
