package core

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type InitModule struct {
	sourceLines map[int]int
	TemplateID  string `json:"templateId"`
	ModuleID    string `json:"moduleId"`
	Name        string `json:"name"`
	Content     string `json:"content"`
}
type InitRequest struct {
	ProjectName       string       `json:"projectName"`
	UnifiedCSV        string       `json:"unifiedCSV"`
	DefaultTemplateID string       `json:"defaultTemplateId"`
	DefaultModuleName string       `json:"defaultModuleName"`
	NetworksCSV       string       `json:"networksCSV"`
	AllowNewModels    bool         `json:"allowNewModels"`
	Revision          int          `json:"revision"`
	ProjectID         string       `json:"projectId"`
	SiteID            string       `json:"siteId"`
	SiteName          string       `json:"siteName"`
	Modules           []InitModule `json:"modules"`
	LinksCSV          string       `json:"linksCSV"`
	Infer             bool         `json:"infer"`
}
type InitPreview struct {
	ProjectID       string               `json:"projectId"`
	NewModels       int                  `json:"newModels"`
	ResolvedModules []InitResolvedModule `json:"resolvedModules"`
	Revision        int                  `json:"revision"`
	SiteID          string               `json:"siteId"`
	NewModules      int                  `json:"newModules"`
	Devices         int                  `json:"devices"`
	Placed          int                  `json:"placed"`
	Pending         int                  `json:"pending"`
	Networks        int                  `json:"networks"`
	Links           int                  `json:"links"`
	Rows            []ImportRow          `json:"rows"`
	Warnings        []string             `json:"warnings"`
	State           State                `json:"state"`
}

func csvRecords(content string) (map[string]int, [][]string, []int, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\ufeff")))
	r.TrimLeadingSpace = true
	head, e := r.Read()
	if e != nil {
		return nil, nil, nil, fmt.Errorf("CSV表头无效")
	}
	aliases := map[string]string{
		"型号": "model", "设备型号": "model", "数量": "quantity", "台数": "quantity", "名称": "name", "设备名称": "name", "资产前缀": "asset_prefix", "资产编号": "asset_tag", "asset": "asset_tag", "序列号": "sn", "备注": "note", "机柜": "rack", "机柜名称": "rack", "起始u": "start_u", "起始u位": "start_u", "u位": "start_u", "模块": "module", "微模块": "module", "微模块名称": "module", "布局模板": "template", "模板": "template", "设备角色": "role", "角色": "role", "u高": "u_height", "高度u": "u_height", "厂商": "vendor",
		"网络": "networks", "所属网络": "networks", "网络分区": "networks", "源资产编号": "source", "目标资产编号": "target", "源端口": "source_port", "目标端口": "target_port", "类型": "kind", "速率": "speed", "介质": "medium", "确认状态": "status", "网络名称": "network_name", "上级网络": "parent", "用途": "purpose", "vlan id": "vlan", "网段": "cidr", "网关": "gateway", "颜色": "color",
	}
	cols := map[string]int{}
	for i, h := range head {
		h = strings.ToLower(strings.TrimSpace(h))
		if a := aliases[h]; a != "" {
			h = a
		}
		if _, ok := cols[h]; ok {
			return nil, nil, nil, fmt.Errorf("重复表头：%s", h)
		}
		cols[h] = i
	}
	rows := [][]string{}
	lines := []int{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, nil, e
		}
		line, _ := r.FieldPos(0)
		rows = append(rows, row)
		lines = append(lines, line)
	}
	return cols, rows, lines, nil
}
func csvField(cols map[string]int, row []string, key string) string {
	if i, ok := cols[key]; ok {
		return strings.TrimSpace(row[i])
	}
	return ""
}
func networkNames(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == ';' || r == '；' })
	out := []string{}
	for _, name := range parts {
		name = strings.TrimSpace(name)
		if name != "" && !Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}
func PrepareInitialization(original State, req InitRequest) (InitPreview, error) {
	result := InitPreview{Revision: original.Revision, Rows: []ImportRow{}, Warnings: []string{}}
	work := Clone(original)
	if strings.TrimSpace(req.UnifiedCSV) == "" && (len(req.Modules) < 1 || len(req.Modules) > 8) {
		return result, fmt.Errorf("每次初始化包含1–8个模块")
	}
	if len(req.UnifiedCSV)+len(req.NetworksCSV)+len(req.LinksCSV) > 2*1024*1024 {
		return result, fmt.Errorf("初始化清单总计不能超过2MiB")
	}
	if req.ProjectID == "" {
		name := strings.TrimSpace(req.ProjectName)
		if !textOK(name, 80) {
			return result, fmt.Errorf("请输入新项目名称")
		}
		for _, p := range work.Projects {
			if strings.EqualFold(p.Name, name) {
				return result, fmt.Errorf("项目名称已存在，请选择已有项目")
			}
		}
		req.ProjectID = ID("project")
		work.Projects = append(work.Projects, Project{ID: req.ProjectID, Name: name})
	}
	projectValid := false
	for _, p := range work.Projects {
		if p.ID == req.ProjectID {
			projectValid = true
		}
	}
	if !projectValid {
		return result, fmt.Errorf("请选择目标项目")
	}
	siteID := req.SiteID
	if siteID != "" {
		found := false
		for _, site := range work.Sites {
			if site.ID == siteID && site.ProjectID == req.ProjectID {
				found = true
			}
		}
		if !found {
			return result, fmt.Errorf("机房不属于所选项目")
		}
	} else {
		if !textOK(req.SiteName, 80) {
			return result, fmt.Errorf("请输入新机房名称")
		}
		for _, site := range work.Sites {
			if site.ProjectID == req.ProjectID && site.Name == strings.TrimSpace(req.SiteName) {
				return result, fmt.Errorf("同名机房已存在，请选择已有机房")
			}
		}
	}
	if strings.TrimSpace(req.UnifiedCSV) != "" {
		if len(req.Modules) > 0 {
			return result, fmt.Errorf("总清单与分模块清单不能同时提交")
		}
		var e error
		req.Modules, e = ResolveInitializationModules(work, req)
		if e != nil {
			return result, e
		}
	}
	if len(req.Modules) < 1 || len(req.Modules) > 8 {
		return result, fmt.Errorf("一张总清单最多包含8个模块")
	}
	startNetworks, startLinks := len(work.Networks), len(work.Links)
	touched := []string{}
	totalBytes := len(req.LinksCSV) + len(req.NetworksCSV)
	usedModules := map[string]bool{}
	if e := ImportNetworkDefinitions(&work, req.ProjectID, req.NetworksCSV); e != nil {
		return result, e
	}
	result.ResolvedModules = []InitResolvedModule{}
	getNetwork := func(name string) (string, error) {
		for _, n := range work.Networks {
			if n.ProjectID == req.ProjectID && strings.EqualFold(n.Name, name) {
				return n.ID, nil
			}
		}
		if !textOK(name, 80) {
			return "", fmt.Errorf("网络名称须为1–80字")
		}
		n := Network{ID: ID("network"), ProjectID: req.ProjectID, Name: name, Purpose: "other", Color: "#497cda", DeviceIDs: []string{}, Note: "初始化清单创建；VLAN、网段和用途待确认"}
		work.Networks = append(work.Networks, n)
		result.Warnings = append(result.Warnings, "新网络「"+name+"」的VLAN、网段和用途待填写")
		return n.ID, nil
	}
	for _, input := range req.Modules {
		totalBytes += len(input.Content)
		if totalBytes > 2*1024*1024 {
			return result, fmt.Errorf("初始化清单总计不能超过2MiB")
		}
		moduleID := input.ModuleID
		if moduleID != "" {
			found := false
			for _, m := range work.Modules {
				if m.ID == moduleID && m.SiteID == siteID {
					found = true
				}
			}
			if !found {
				return result, fmt.Errorf("已有模块不属于目标机房")
			}
		} else {
			template := work.Template(input.TemplateID)
			if template == nil {
				return result, fmt.Errorf("请选择有效布局模板")
			}
			plan := CopyModulePlan(template.Plan)
			plan.Name = strings.TrimSpace(input.Name)
			if plan.Name == "" {
				plan.Name = template.Plan.Name
			}
			before := len(work.Modules)
			_, e := AddPlan(&work, Plan{ProjectID: req.ProjectID, SiteID: siteID, SiteName: strings.TrimSpace(req.SiteName), Modules: []ModulePlan{plan}})
			if e != nil {
				return result, e
			}
			moduleID = work.Modules[before].ID
			siteID = work.Modules[before].SiteID
			result.NewModules++
		}
		moduleName := ""
		for _, m := range work.Modules {
			if m.ID == moduleID {
				moduleName = m.Name
			}
		}
		result.ResolvedModules = append(result.ResolvedModules, InitResolvedModule{ID: moduleID, Name: moduleName, Existing: input.ModuleID != ""})
		if usedModules[moduleID] {
			return result, fmt.Errorf("同一模块只能列出一次")
		}
		usedModules[moduleID] = true
		if strings.TrimSpace(input.Content) == "" {
			continue
		}
		fingerprint, e := InitializationFingerprint(moduleID, input.Content)
		if e != nil {
			return result, e
		}
		for _, d := range work.Devices {
			if d.Source == fingerprint {
				return result, fmt.Errorf("模块%s已导入过相同清单，请勿重复提交；新增批次请使用明确的新资产编号", moduleName)
			}
		}
		{
			created, e := ImportModelDefinitions(&work, input.Content, req.AllowNewModels)
			if e != nil {
				return result, fmt.Errorf("模块%s：%s", moduleName, initializationLineError(e, input.sourceLines))
			}
			for _, name := range created {
				result.NewModels++
				result.Warnings = append(result.Warnings, "新型号「"+name+"」按清单注册，请核对高度及角色")
			}
		}
		preview, e := PreviewCSV(work, moduleID, input.Content)
		if e != nil {
			return result, fmt.Errorf("模块%s：%s", moduleName, initializationLineError(e, input.sourceLines))
		}
		if result.Devices+preview.Total > 2000 {
			return result, fmt.Errorf("单次最多展开2000台设备")
		}
		cols, rows, lines, e := csvRecords(input.Content)
		if e != nil {
			return result, e
		}
		byLine := map[int][]string{}
		for i, line := range lines {
			byLine[line] = networkNames(csvField(cols, rows[i], "networks"))
		}
		for _, row := range preview.Rows {
			row.Device.Source = fingerprint
			work.Devices = append(work.Devices, row.Device)
			displayRow := row
			if sourceLine, ok := input.sourceLines[row.Line]; ok {
				displayRow.Line = sourceLine
			}
			result.Rows = append(result.Rows, displayRow)
			if row.Reason != "" {
				result.Warnings = append(result.Warnings, row.Device.AssetTag+"："+row.Reason)
			}
			for _, name := range byLine[row.Line] {
				id, e := getNetwork(name)
				if e != nil {
					return result, e
				}
				n := work.Network(id)
				n.DeviceIDs = append(n.DeviceIDs, row.Device.ID)
				if !Contains(touched, id) {
					touched = append(touched, id)
				}
			}
		}
		result.Devices += preview.Total
		result.Placed += preview.Placed
		result.Pending += preview.Pending
	}
	if strings.TrimSpace(req.LinksCSV) != "" {
		cols, rows, _, e := csvRecords(req.LinksCSV)
		if e != nil {
			return result, fmt.Errorf("连接清单：%w", e)
		}
		for _, key := range []string{"source", "target"} {
			if _, ok := cols[key]; !ok {
				return result, fmt.Errorf("连接清单缺少%s列", key)
			}
		}
		if len(rows) > 2000 {
			return result, fmt.Errorf("连接清单最多2000条")
		}
		lookup := map[string]string{}
		for _, d := range work.Devices {
			if work.DeviceProject(d.ID) == req.ProjectID {
				lookup[strings.ToLower(d.AssetTag)] = d.ID
			}
		}
		for i, row := range rows {
			source, target := lookup[strings.ToLower(csvField(cols, row, "source"))], lookup[strings.ToLower(csvField(cols, row, "target"))]
			if source == "" || target == "" {
				return result, fmt.Errorf("连接清单第%d条的源/目标资产编号不存在，请按设备预览中的资产编号填写", i+1)
			}
			kind := csvField(cols, row, "kind")
			switch kind {
			case "", "物理":
				kind = "physical"
			case "逻辑":
				kind = "logical"
			}
			status := csvField(cols, row, "status")
			switch status {
			case "", "待确认":
				status = "pending"
			case "已确认":
				status = "confirmed"
			}
			speed := 0.0
			if value := csvField(cols, row, "speed"); value != "" {
				speed, e = strconv.ParseFloat(value, 64)
				if e != nil {
					return result, fmt.Errorf("连接清单速率无效")
				}
			}
			l := Link{ID: ID("link"), ProjectID: req.ProjectID, SourceID: source, TargetID: target, SourcePort: csvField(cols, row, "source_port"), TargetPort: csvField(cols, row, "target_port"), Kind: kind, Status: status, SpeedGbps: speed, Medium: csvField(cols, row, "medium"), Origin: "manual", NetworkIDs: []string{}, Note: "初始化连接清单"}
			for _, name := range networkNames(csvField(cols, row, "networks")) {
				id, e := getNetwork(name)
				if e != nil {
					return result, e
				}
				n := work.Network(id)
				if !Contains(n.DeviceIDs, source) || !Contains(n.DeviceIDs, target) {
					return result, fmt.Errorf("连接网络%s的两端成员尚未在设备清单中声明", name)
				}
				l.NetworkIDs = append(l.NetworkIDs, id)
			}
			work.Links = append(work.Links, l)
		}
	}
	if e := Validate(&work); e != nil {
		return result, e
	}
	if req.Infer {
		for _, id := range touched {
			links, e := SuggestLinks(work, id)
			if e != nil {
				return result, e
			}
			work.Links = append(work.Links, links...)
		}
		if len(touched) == 0 {
			result.Warnings = append(result.Warnings, "设备清单未填写网络，未进行连接推演")
		}
	}
	if result.NewModules == 0 && result.Devices == 0 && len(work.Links) == startLinks {
		return result, fmt.Errorf("请选择模板创建模块或上传设备/连接清单")
	}
	if e := Validate(&work); e != nil {
		return result, e
	}
	result.Networks = len(work.Networks) - startNetworks
	result.Links = len(work.Links) - startLinks
	result.SiteID = siteID
	result.ProjectID = req.ProjectID
	result.State = work
	return result, nil
}

var initializationLinePattern = regexp.MustCompile(`第([0-9]+)行`)

func initializationLineError(err error, sourceLines map[int]int) string {
	if len(sourceLines) == 0 {
		return err.Error()
	}
	return initializationLinePattern.ReplaceAllStringFunc(err.Error(), func(text string) string {
		parts := initializationLinePattern.FindStringSubmatch(text)
		i, _ := strconv.Atoi(parts[1])
		if line, ok := sourceLines[i]; ok {
			return fmt.Sprintf("总清单第%d行", line)
		}
		return text
	})
}

func InitializationFingerprint(moduleID, content string) (string, error) {
	cols, rows, _, e := csvRecords(content)
	if e != nil {
		return "", e
	}
	keys := []string{}
	for key := range cols {
		if key != "module" && key != "template" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(keys)
	for _, row := range rows {
		values := []string{}
		for _, key := range keys {
			values = append(values, csvField(cols, row, key))
		}
		_ = w.Write(values)
	}
	w.Flush()
	if e = w.Error(); e != nil {
		return "", e
	}
	return fmt.Sprintf("initialization:%x", sha256.Sum256([]byte(moduleID+"\n"+b.String()))), nil
}
