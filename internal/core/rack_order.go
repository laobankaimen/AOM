package core

import (
	"fmt"
	"sort"
)

// Reorder changes physical positions only. Rack identity, names, devices and
// links remain stable. Cabinet-integrated facilities travel with their cabinet;
// standalone facilities stay at their row position (clamped at a shortened end).
func ReorderRacks(s *State, moduleID string, left, right []string) (string, error) {
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
	if module.Type == "single" {
		if len(left) < 1 || len(left) > 24 || len(right) != 0 {
			return "", fmt.Errorf("单排须保留1–24柜，不能移入右列")
		}
	} else if len(left) < 1 || len(left) > 12 || len(right) < 1 || len(right) > 12 {
		return "", fmt.Errorf("双排每列须保留1–12柜")
	}
	expected := map[string]bool{}
	oldPositions := map[string]string{}
	for _, r := range s.Racks {
		if r.ModuleID == moduleID {
			expected[r.ID] = true
			oldPositions[fmt.Sprintf("%s:%d", r.Side, r.Index)] = r.ID
		}
	}
	seen := map[string]bool{}
	for _, id := range append(append([]string{}, left...), right...) {
		if !expected[id] || seen[id] {
			return "", fmt.Errorf("排序必须包含本模块的每个机柜且只能出现一次")
		}
		seen[id] = true
	}
	if len(seen) != len(expected) {
		return "", fmt.Errorf("排序不能添加或删除机柜")
	}
	attached := map[string]string{}
	for _, i := range s.Infra {
		if i.ModuleID == moduleID && i.Placement == "integrated" {
			attached[i.ID] = oldPositions[fmt.Sprintf("%s:%d", i.Side, i.AfterRack)]
		}
	}
	for _, row := range []struct {
		side string
		ids  []string
	}{{"L", left}, {"R", right}} {
		for i, id := range row.ids {
			rack := s.Rack(id)
			rack.Side = row.side
			rack.Index = i + 1
		}
	}
	module.RackLeft = len(left)
	module.RackRight = len(right)
	for i := range s.Infra {
		v := &s.Infra[i]
		if v.ModuleID != moduleID {
			continue
		}
		if id := attached[v.ID]; id != "" {
			r := s.Rack(id)
			v.Side = r.Side
			v.AfterRack = r.Index
		} else {
			count := len(left)
			if v.Side == "R" {
				count = len(right)
			}
			if v.AfterRack > count {
				v.AfterRack = count
			}
		}
	}
	if err := Validate(s); err != nil {
		return "", err
	}
	return "调整机柜排列：" + module.Name, nil
}
func OrderedRacks(s State, moduleID, side string) []Rack {
	out := []Rack{}
	for _, r := range s.Racks {
		if r.ModuleID == moduleID && r.Side == side {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// OrderedRowItems includes racks and standalone infrastructure in display order.
func OrderedRowItems(s State, moduleID, side string) []string {
	racks := OrderedRacks(s, moduleID, side)
	infra := []Infra{}
	for _, i := range s.Infra {
		if i.ModuleID == moduleID && i.Side == side && i.Placement == "inline" {
			infra = append(infra, i)
		}
	}
	sort.SliceStable(infra, func(i, j int) bool { return infra[i].Order < infra[j].Order })
	result := []string{}
	appendInfra := func(after int) {
		for _, i := range infra {
			if i.AfterRack == after {
				result = append(result, i.ID)
			}
		}
	}
	appendInfra(0)
	for _, r := range racks {
		result = append(result, r.ID)
		appendInfra(r.Index)
	}
	return result
}

// Insert one rack between any two row objects. Other objects retain their order;
// integrated facilities remain attached to the same rack identity.
func InsertRack(s *State, moduleID, rackID, side, beforeID string) (string, error) {
	rack := s.Rack(rackID)
	if rack == nil || rack.ModuleID != moduleID {
		return "", fmt.Errorf("请选择当前微模块中的机柜")
	}
	if side != "L" && side != "R" {
		return "", fmt.Errorf("目标列无效")
	}
	rows := map[string][]string{"L": OrderedRowItems(*s, moduleID, "L"), "R": OrderedRowItems(*s, moduleID, "R")}
	if beforeID != "" && !Contains(rows[side], beforeID) {
		return "", fmt.Errorf("插入位置已变化，请刷新后重试")
	}
	if beforeID == rackID {
		return "机柜位置未变化", nil
	}
	original := map[string][]string{"L": append([]string{}, rows["L"]...), "R": append([]string{}, rows["R"]...)}
	rows[rack.Side] = RemoveID(rows[rack.Side], rackID)
	target := rows[side]
	at := len(target)
	if beforeID != "" {
		for i, id := range target {
			if id == beforeID {
				at = i
				break
			}
		}
	}
	target = append(target, "")
	copy(target[at+1:], target[at:])
	target[at] = rackID
	rows[side] = target
	same := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if same(rows["L"], original["L"]) && same(rows["R"], original["R"]) {
		return "机柜位置未变化", nil
	}
	rackRows := map[string][]string{"L": {}, "R": {}}
	for _, side := range []string{"L", "R"} {
		for _, id := range rows[side] {
			if s.Rack(id) != nil {
				rackRows[side] = append(rackRows[side], id)
			}
		}
	}
	if _, err := ReorderRacks(s, moduleID, rackRows["L"], rackRows["R"]); err != nil {
		return "", err
	}
	for _, side := range []string{"L", "R"} {
		count, order := 0, 0
		for _, id := range rows[side] {
			if s.Rack(id) != nil {
				count++
				continue
			}
			for i := range s.Infra {
				v := &s.Infra[i]
				if v.ID == id {
					v.Side = side
					v.AfterRack = count
					v.Order = order
					order++
					break
				}
			}
		}
	}
	if err := Validate(s); err != nil {
		return "", err
	}
	return "插入机柜：" + rack.Name, nil
}
