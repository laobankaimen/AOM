package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ModuleTemplate struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Plan        ModulePlan `json:"plan"`
	UpdatedAt   string     `json:"updatedAt"`
}

func (s *State) Template(id string) *ModuleTemplate {
	for i := range s.Templates {
		if s.Templates[i].ID == id {
			return &s.Templates[i]
		}
	}
	return nil
}
func CopyModulePlan(p ModulePlan) ModulePlan {
	b, _ := json.Marshal(p)
	var result ModulePlan
	_ = json.Unmarshal(b, &result)
	return result
}
func CaptureModulePlan(s State, id string) (ModulePlan, error) {
	var module *Module
	for i := range s.Modules {
		if s.Modules[i].ID == id {
			module = &s.Modules[i]
			break
		}
	}
	if module == nil {
		return ModulePlan{}, fmt.Errorf("微模块不存在")
	}
	p := ModulePlan{Name: module.Name, ProductModel: module.ProductModel, Point: module.Point, Type: module.Type, RackCount: module.RackLeft, RackLeft: module.RackLeft, RackRight: module.RackRight, UHeight: 42, RackNames: map[string][]string{"L": {}, "R": {}}, Layout: []InfraSlot{}}
	for _, side := range []string{"L", "R"} {
		for _, r := range OrderedRacks(s, id, side) {
			p.RackNames[side] = append(p.RackNames[side], r.Name)
		}
	}
	for _, i := range s.Infra {
		if i.ModuleID == id {
			p.Layout = append(p.Layout, InfraSlot{Kind: i.Kind, Name: i.Name, Side: i.Side, AfterRack: i.AfterRack, Placement: i.Placement, Order: i.Order})
		}
	}
	return p, nil
}
func ValidateModulePlan(p ModulePlan) error {
	if !textOK(p.Name, 80) || len([]rune(p.ProductModel)) > 120 || len([]rune(p.Point)) > 80 || p.UHeight != 42 {
		return fmt.Errorf("模板模块名称、型号或42U标准无效")
	}
	left, right := p.RackLeft, p.RackRight
	if p.Type == "single" {
		left = p.RackCount
		right = 0
		if left < 1 || left > 24 {
			return fmt.Errorf("单排模板须为1–24柜")
		}
	} else if p.Type == "double" {
		if left < 1 || left > 12 || right < 1 || right > 12 {
			return fmt.Errorf("双排模板每列须为1–12柜")
		}
	} else {
		return fmt.Errorf("模板排列须为单排或双排")
	}
	names := map[string]bool{}
	for _, row := range []struct {
		side  string
		count int
	}{{"L", left}, {"R", right}} {
		if values, ok := p.RackNames[row.side]; ok {
			if len(values) != row.count {
				return fmt.Errorf("模板柜名数量与列数量不一致")
			}
			for _, name := range values {
				key := strings.ToLower(strings.TrimSpace(name))
				if !textOK(name, 80) || names[key] {
					return fmt.Errorf("模板机柜名称须非空且不重复")
				}
				names[key] = true
			}
		}
	}
	if len(p.Layout) > 48 {
		return fmt.Errorf("模板设施最多48项")
	}
	if p.Infra != nil {
		for _, n := range []int{p.Infra.Power, p.Infra.AC, p.Infra.Env} {
			if n < 0 || n > 8 {
				return fmt.Errorf("模板基础设施数量须为0–8")
			}
		}
	}
	check := State{Modules: []Module{{ID: "check", Type: p.Type, RackLeft: left, RackRight: right}}}
	for _, slot := range p.Layout {
		if !textOK(slot.Name, 80) {
			return fmt.Errorf("设施名称无效")
		}
		if e := ValidateInfra(&check, Infra{ModuleID: "check", Name: slot.Name, Kind: slot.Kind, Side: slot.Side, AfterRack: slot.AfterRack, Placement: slot.Placement, Order: slot.Order}); e != nil {
			return e
		}
	}
	return nil
}
func ValidateTemplates(s *State, checkID func(string) bool) error {
	if len(s.Templates) > 200 {
		return fmt.Errorf("布局模板最多200个")
	}
	names := map[string]bool{}
	for _, t := range s.Templates {
		key := strings.ToLower(strings.TrimSpace(t.Name))
		if !checkID(t.ID) || !textOK(t.Name, 80) || names[key] || len([]rune(t.Description)) > 1000 {
			return fmt.Errorf("模板名称须唯一且不超过80字，说明最多1000字")
		}
		names[key] = true
		if e := ValidateModulePlan(t.Plan); e != nil {
			return fmt.Errorf("模板%s：%w", t.Name, e)
		}
	}
	return nil
}
func DefaultTemplates() []ModuleTemplate {
	base := []ModulePlan{
		{Name: "单排微模块", ProductModel: "曙光数创 C2000（SG6600）", Type: "single", RackCount: 5, UHeight: 42, Infra: &InfraPlan{Power: 1, AC: 1, Env: 1}},
		{Name: "双排微模块", ProductModel: "曙光数创 C2000（SG6600）", Type: "double", RackLeft: 4, RackRight: 4, UHeight: 42, Infra: &InfraPlan{Power: 2, AC: 2, Env: 2}},
		{Name: "综合微模块", ProductModel: "曙光数创 C500（C500-40K）", Type: "single", RackCount: 4, UHeight: 42, Layout: []InfraSlot{{Kind: "power", Name: "集成供电", Side: "L", AfterRack: 1, Placement: "integrated"}, {Kind: "ac", Name: "集成制冷", Side: "L", AfterRack: 1, Placement: "integrated"}, {Kind: "ups", Name: "集成UPS", Side: "L", AfterRack: 1, Placement: "integrated"}, {Kind: "env", Name: "环境监测", Side: "L", AfterRack: 1, Placement: "integrated"}}},
	}
	result := []ModuleTemplate{}
	for i, p := range base {
		result = append(result, ModuleTemplate{ID: fmt.Sprintf("template-default-%d", i+1), Name: []string{"C2000 单排 · 5柜", "C2000 双排 · 左4右4", "C500 单排 · 4柜示意"}[i], Description: "项目规划示例；实际柜数、设施位置与可用U位须现场确认。", Plan: p})
	}
	return result
}
