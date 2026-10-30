package core

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

type InitResolvedModule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Existing bool   `json:"existing"`
}

func ResolveInitializationModules(s State, req InitRequest) ([]InitModule, error) {
	cols, rows, lines, e := csvRecords(req.UnifiedCSV)
	if e != nil {
		return nil, fmt.Errorf("总清单：%w", e)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("总清单没有设备行")
	}
	names := []string{}
	groups := map[string][][]string{}
	sourceLines := map[string][]int{}
	templateIDs := map[string]string{}
	moduleIDs := map[string]string{}
	headers := make([]string, len(cols))
	for name, i := range cols {
		headers[i] = name
	}
	for i, row := range rows {
		name := csvField(cols, row, "module")
		if name == "" {
			name = strings.TrimSpace(req.DefaultModuleName)
		}
		if name == "" {
			return nil, fmt.Errorf("总清单第%d行未填写模块，请填写模块列或默认模块名称", lines[i])
		}
		if !textOK(name, 80) {
			return nil, fmt.Errorf("总清单第%d行模块名称过长", lines[i])
		}
		key := strings.ToLower(name)
		if _, ok := groups[key]; !ok {
			if len(names) >= 8 {
				return nil, fmt.Errorf("总清单最多8个模块")
			}
			names = append(names, name)
			groups[key] = [][]string{}
			for _, m := range s.Modules {
				if req.SiteID != "" && m.SiteID == req.SiteID && strings.EqualFold(m.Name, name) {
					if moduleIDs[key] != "" {
						return nil, fmt.Errorf("机房内模块名称不唯一：%s", name)
					}
					moduleIDs[key] = m.ID
				}
			}
		}
		requested := csvField(cols, row, "template")
		if requested != "" {
			found := ""
			for _, t := range s.Templates {
				if t.ID == requested || strings.EqualFold(t.Name, requested) {
					found = t.ID
					break
				}
			}
			if found == "" {
				return nil, fmt.Errorf("总清单第%d行布局模板不存在：%s", lines[i], requested)
			}
			if old := templateIDs[key]; old != "" && old != found {
				return nil, fmt.Errorf("模块%s指定了不同的模板", name)
			}
			templateIDs[key] = found
		}
		groups[key] = append(groups[key], row)
		sourceLines[key] = append(sourceLines[key], lines[i])
	}
	out := []InitModule{}
	for _, name := range names {
		key := strings.ToLower(name)
		templateID := templateIDs[key]
		if templateID == "" {
			templateID = req.DefaultTemplateID
		}
		if moduleIDs[key] == "" && s.Template(templateID) == nil {
			return nil, fmt.Errorf("新模块%s需要选择默认布局模板或填写模板列", name)
		}
		var b strings.Builder
		w := csv.NewWriter(&b)
		_ = w.Write(headers)
		w.WriteAll(groups[key])
		if e := w.Error(); e != nil {
			return nil, e
		}
		_, _, groupLines, e := csvRecords(b.String())
		if e != nil {
			return nil, e
		}
		lineMap := map[int]int{}
		for i, line := range groupLines {
			lineMap[line] = sourceLines[key][i]
		}
		out = append(out, InitModule{TemplateID: templateID, ModuleID: moduleIDs[key], Name: name, Content: b.String(), sourceLines: lineMap})
	}
	return out, nil
}
func ImportModelDefinitions(s *State, content string, allowNew bool) ([]string, error) {
	cols, rows, lines, e := csvRecords(content)
	if e != nil {
		return nil, e
	}
	created := []string{}
	roles := map[string]string{"服务器": "server", "计算服务器": "server", "核心": "core", "核心交换": "core", "汇聚": "aggregation", "接入": "access", "业务接入": "access", "管理接入": "management", "存储": "storage", "基础设施": "infra"}
	for i, row := range rows {
		name := csvField(cols, row, "model")
		var existing *Model
		for j := range s.Models {
			if strings.EqualFold(s.Models[j].Name, name) {
				existing = &s.Models[j]
				break
			}
		}
		role := csvField(cols, row, "role")
		if v := roles[role]; v != "" {
			role = v
		}
		height := 0
		heightText := csvField(cols, row, "u_height")
		if heightText != "" {
			height, e = strconv.Atoi(heightText)
			if e != nil || height < 0 || height > 42 {
				return nil, fmt.Errorf("第%d行U高须为0–42整数", lines[i])
			}
		}
		if existing != nil {
			if role != "" && existing.Role != role || heightText != "" && existing.UHeight != height {
				return nil, fmt.Errorf("第%d行型号%s参数与型号库不一致，请核对，初始化不会覆盖已有型号", lines[i], name)
			}
			continue
		}
		if !allowNew {
			return nil, fmt.Errorf("第%d行型号不存在：%s；可填写角色并启用注册新型号", lines[i], name)
		}
		valid := false
		for _, v := range []string{"server", "core", "aggregation", "access", "management", "storage", "infra"} {
			if role == v {
				valid = true
			}
		}
		if !textOK(name, 100) || !valid {
			return nil, fmt.Errorf("第%d行未知型号%s，请填写设备角色（服务器/核心/汇聚/接入/管理接入/存储/基础设施）", lines[i], name)
		}
		s.Models = append(s.Models, Model{ID: ID("model"), Name: name, Vendor: csvField(cols, row, "vendor"), Role: role, UHeight: height})
		created = append(created, name)
	}
	return created, nil
}
func ImportNetworkDefinitions(s *State, projectID, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	cols, rows, lines, e := csvRecords(content)
	if e != nil {
		return fmt.Errorf("网络清单：%w", e)
	}
	if len(rows) > 200 {
		return fmt.Errorf("网络参数清单最多200行")
	}
	if _, ok := cols["network_name"]; !ok {
		return fmt.Errorf("网络清单缺少网络名称列")
	}
	names := map[string]bool{}
	parents := map[string]string{}
	purposes := map[string]string{"业务": "business", "业务网络": "business", "管理": "management", "管理网络": "management", "存储": "storage", "存储网络": "storage", "互联": "interconnect", "其他": "other"}
	for i, row := range rows {
		name := csvField(cols, row, "network_name")
		if !textOK(name, 80) {
			return fmt.Errorf("网络清单第%d行名称无效", lines[i])
		}
		key := strings.ToLower(name)
		if names[key] {
			return fmt.Errorf("网络清单名称重复：%s", name)
		}
		names[key] = true
		var n *Network
		for j := range s.Networks {
			if s.Networks[j].ProjectID == projectID && strings.EqualFold(s.Networks[j].Name, name) {
				n = &s.Networks[j]
				break
			}
		}
		if n == nil {
			v := Network{ID: ID("network"), ProjectID: projectID, Name: name, Purpose: "other", Color: "#497cda", DeviceIDs: []string{}}
			s.Networks = append(s.Networks, v)
			n = &s.Networks[len(s.Networks)-1]
		}
		checkString := func(column string, field *string) error {
			v := csvField(cols, row, column)
			if v == "" {
				return nil
			}
			if column == "purpose" {
				if mapped := purposes[v]; mapped != "" {
					v = mapped
				}
			}
			if *field != "" && !(column == "purpose" && *field == "other") && !(column == "color" && *field == "#497cda") && *field != v {
				return fmt.Errorf("网络%s的%s与现有配置冲突，不会覆盖", name, column)
			}
			*field = v
			return nil
		}
		for column, field := range map[string]*string{"purpose": &n.Purpose, "cidr": &n.CIDR, "gateway": &n.Gateway, "color": &n.Color} {
			if e = checkString(column, field); e != nil {
				return e
			}
		}
		if text := csvField(cols, row, "vlan"); text != "" {
			v, e := strconv.Atoi(text)
			if e != nil || v < 0 || v > 4094 {
				return fmt.Errorf("网络清单第%d行VLAN无效", lines[i])
			}
			if n.VLAN != 0 && n.VLAN != v {
				return fmt.Errorf("网络%s的VLAN与现有配置冲突", name)
			}
			n.VLAN = v
		}
		if parent := csvField(cols, row, "parent"); parent != "" {
			parents[n.ID] = parent
		}
	}
	for id, name := range parents {
		var parent *Network
		for j := range s.Networks {
			if s.Networks[j].ProjectID == projectID && strings.EqualFold(s.Networks[j].Name, name) {
				parent = &s.Networks[j]
				break
			}
		}
		if parent == nil {
			return fmt.Errorf("父网络%s不存在，请在清单或系统中先声明", name)
		}
		n := s.Network(id)
		if n.ParentID != "" && n.ParentID != parent.ID {
			return fmt.Errorf("网络%s的上级与现有配置冲突", n.Name)
		}
		n.ParentID = parent.ID
	}
	return Validate(s)
}
