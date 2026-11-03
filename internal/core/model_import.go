package core

import (
	"fmt"
	"strconv"
	"strings"
)

type ModelImportRow struct {
	Model    Model `json:"model"`
	Line     int   `json:"line"`
	Existing bool  `json:"existing"`
}
type ModelImportPreview struct {
	Revision int              `json:"revision"`
	Rows     []ModelImportRow `json:"rows"`
	Added    int              `json:"added"`
	Skipped  int              `json:"skipped"`
}

// Model imports only append. Conflicting definitions fail the entire preview/commit.
func PreviewModels(original State, content string) (ModelImportPreview, error) {
	result := ModelImportPreview{Revision: original.Revision, Rows: []ModelImportRow{}}
	if len(content) > 2*1024*1024 {
		return result, fmt.Errorf("CSV文件不能超过2MiB")
	}
	cols, rows, lines, err := csvRecords(content)
	if err != nil {
		return result, err
	}
	for _, key := range []string{"model", "role"} {
		if _, ok := cols[key]; !ok {
			return result, fmt.Errorf("缺少必填列：型号、设备角色")
		}
	}
	if len(rows) > 2000 {
		return result, fmt.Errorf("一次最多导入2000个型号")
	}
	work := Clone(original)
	for i, row := range rows {
		get := func(key string) string { return csvField(cols, row, key) }
		role := work.ResolveRole(get("role"))
		if role == "" {
			return result, fmt.Errorf("第%d行角色不存在：%s，请先在角色管理中添加", lines[i], get("role"))
		}
		height := 0
		if get("u_height") != "" {
			height, err = strconv.Atoi(get("u_height"))
			if err != nil || height < 0 || height > 42 {
				return result, fmt.Errorf("第%d行U高须为0–42整数", lines[i])
			}
		}
		m := Model{ID: ID("model"), Name: get("model"), Vendor: get("vendor"), Role: role, UHeight: height, Ports: get("ports")}
		existing := false
		for _, old := range work.Models {
			if strings.EqualFold(old.Name, m.Name) {
				existing = true
				if old.Role != m.Role || old.UHeight != m.UHeight || old.Vendor != m.Vendor || old.Ports != m.Ports {
					return result, fmt.Errorf("第%d行型号%s与已有定义冲突，请在型号详情中核对；导入不会覆盖", lines[i], m.Name)
				}
				m = old
				break
			}
		}
		if !existing {
			work.Models = append(work.Models, m)
			if err := Validate(&work); err != nil {
				return result, fmt.Errorf("第%d行：%w", lines[i], err)
			}
			result.Added++
		} else {
			result.Skipped++
		}
		result.Rows = append(result.Rows, ModelImportRow{Model: m, Line: lines[i], Existing: existing})
	}
	if len(result.Rows) == 0 {
		return result, fmt.Errorf("清单中没有型号数据")
	}
	return result, nil
}
