package core

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
)

type ImportRow struct {
	Device Device `json:"device"`
	Reason string `json:"reason"`
	Line   int    `json:"line"`
}
type ImportPreview struct {
	Rows     []ImportRow `json:"rows"`
	Total    int         `json:"total"`
	Placed   int         `json:"placed"`
	Pending  int         `json:"pending"`
	Revision int         `json:"revision"`
}

func PreviewCSV(original State, moduleID, content string) (ImportPreview, error) {
	result := ImportPreview{Rows: []ImportRow{}, Revision: original.Revision}
	if len(content) > 2*1024*1024 {
		return result, fmt.Errorf("CSV文件不能超过2MiB")
	}
	if !original.HasModule(moduleID) {
		return result, fmt.Errorf("请选择有效的目标模块")
	}
	cols, records, lines, err := csvRecords(content)
	if err != nil {
		return result, err
	}
	for _, h := range []string{"model", "quantity"} {
		if _, ok := cols[h]; !ok {
			return result, fmt.Errorf("缺少必填列：%s（型号/数量）", h)
		}
	}
	work := Clone(original)
	assets := map[string]bool{}
	for _, d := range work.Devices {
		assets[strings.ToLower(strings.TrimSpace(d.AssetTag))] = true
	}
	// Reserve explicit identities before generating any automatic identifiers.
	for i, row := range records {
		tag := csvField(cols, row, "asset_tag")
		if tag != "" {
			key := strings.ToLower(tag)
			if assets[key] {
				return result, fmt.Errorf("第%d行资产编号已存在或重复：%s", lines[i], tag)
			}
			assets[key] = true
		}
	}
	type pending struct {
		entry    ImportRow
		rack     *Rack
		explicit bool
	}
	entries := []pending{}
	for i, row := range records {
		get := func(key string) string { return csvField(cols, row, key) }
		line := lines[i]
		count, e := strconv.Atoi(get("quantity"))
		if e != nil || count < 1 || count > 2000 || len(entries)+count > 2000 {
			return result, fmt.Errorf("第%d行数量须为正整数，单次最多展开2000台", line)
		}
		var model *Model
		for j := range work.Models {
			if strings.EqualFold(work.Models[j].Name, get("model")) {
				model = &work.Models[j]
				break
			}
		}
		if model == nil {
			return result, fmt.Errorf("第%d行型号不存在：%s，请补充型号及角色", line, get("model"))
		}
		if count > 1 && (get("ip") != "" || get("sn") != "" || get("asset_tag") != "" || get("start_u") != "") {
			return result, fmt.Errorf("第%d行数量大于1时，IP/SN/资产编号/起始U须为空；逐台资料请拆成数量1的行", line)
		}
		var rack *Rack
		if get("rack") != "" {
			for j := range work.Racks {
				if work.Racks[j].ModuleID == moduleID && strings.EqualFold(work.Racks[j].Name, get("rack")) {
					if rack != nil {
						return result, fmt.Errorf("第%d行机柜名称不唯一", line)
					}
					rack = &work.Racks[j]
				}
			}
			if rack == nil {
				return result, fmt.Errorf("第%d行机柜%s不存在于目标模块", line, get("rack"))
			}
		}
		start := 0
		explicit := get("start_u") != ""
		if explicit {
			start, e = strconv.Atoi(get("start_u"))
			if e != nil || start < 1 || start > 42 || rack == nil {
				return result, fmt.Errorf("第%d行已知机位须同时填写机柜与1–42之间的起始U", line)
			}
		}
		prefix := get("asset_prefix")
		if prefix == "" {
			prefix = "AOM"
		}
		if len([]rune(prefix)) > 60 {
			return result, fmt.Errorf("第%d行资产前缀过长", line)
		}
		for index := 0; index < count; index++ {
			tag := get("asset_tag")
			if tag == "" {
				seq := 1
				for {
					tag = fmt.Sprintf("%s-%04d", prefix, seq)
					if !assets[strings.ToLower(tag)] {
						break
					}
					seq++
				}
				assets[strings.ToLower(tag)] = true
			}
			name := get("name")
			if name == "" {
				name = model.Name
			}
			if count > 1 {
				name = fmt.Sprintf("%s %02d", name, index+1)
			}
			d := Device{ID: ID("dev"), AssetTag: tag, Name: name, ModelID: model.ID, ModuleID: moduleID, IP: get("ip"), SN: get("sn"), Status: "pending", Note: get("note"), Source: "csv"}
			if explicit {
				d.RackID = rack.ID
				d.StartU = start
				d.Status = "installed"
				work.Devices = append(work.Devices, d)
				if e = Validate(&work); e != nil {
					return result, fmt.Errorf("第%d行：%w", line, e)
				}
			}
			entries = append(entries, pending{entry: ImportRow{Device: d, Line: line}, rack: rack, explicit: explicit})
		}
	}
	// Known U positions win over automatic placement, regardless of row order.
	for _, item := range entries {
		row := item.entry
		if !item.explicit {
			candidate := work
			if item.rack != nil {
				candidate.Racks = []Rack{*item.rack}
			}
			row.Reason = AutoPlace(&candidate, &row.Device)
			if row.Reason != "" && item.rack != nil {
				row.Reason = item.rack.Name + "：" + row.Reason
			}
			work.Devices = append(work.Devices, row.Device)
		}
		result.Rows = append(result.Rows, row)
		result.Total++
		if row.Device.RackID != "" {
			result.Placed++
		} else {
			result.Pending++
		}
	}
	if result.Total == 0 {
		return result, fmt.Errorf("CSV没有设备数据")
	}
	if e := Validate(&work); e != nil {
		return result, e
	}
	return result, nil
}
func ExportCSV(s State) string {
	var b strings.Builder
	b.WriteString("\ufeff")
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"资产编号", "名称", "型号", "项目", "机房", "点位", "模块", "机柜", "起始U", "U高", "IP", "SN", "状态", "备注"})
	safe := func(v string) string {
		trim := strings.TrimLeft(v, " \t\r\n")
		if trim != "" && strings.ContainsAny(trim[:1], "=+-@") {
			return "'" + v
		}
		return v
	}
	for _, d := range s.Devices {
		m := s.Model(d.ModelID)
		moduleName, siteName, rackName := "", "", "待上架"
		projectName, point := "", ""
		for _, mod := range s.Modules {
			if mod.ID == d.ModuleID {
				moduleName = mod.Name
				point = mod.Point
				for _, site := range s.Sites {
					if site.ID == mod.SiteID {
						siteName = site.Name
						for _, project := range s.Projects {
							if project.ID == site.ProjectID {
								projectName = project.Name
							}
						}
					}
				}
			}
		}
		if r := s.Rack(d.RackID); r != nil {
			rackName = r.Name
		}
		row := []string{d.AssetTag, d.Name, m.Name, projectName, siteName, point, moduleName, rackName, strconv.Itoa(d.StartU), strconv.Itoa(m.UHeight), d.IP, d.SN, d.Status, d.Note}
		for i := range row {
			row[i] = safe(row[i])
		}
		_ = w.Write(row)
	}
	w.Flush()
	return b.String()
}
