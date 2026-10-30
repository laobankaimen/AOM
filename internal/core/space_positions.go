package core

import (
	"fmt"
	"sort"
)

func SetRoomPosition(s *State, moduleID string, position *CanvasPoint) (string, error) {
	return SetRoomTransform(s, moduleID, position, nil)
}
func SetRoomTransform(s *State, moduleID string, position *CanvasPoint, rotation *int) (string, error) {
	if rotation != nil && *rotation != 0 && *rotation != 90 && *rotation != 180 && *rotation != 270 {
		return "", fmt.Errorf("方向须为0/90/180/270度")
	}
	for i := range s.Modules {
		m := &s.Modules[i]
		if m.ID == moduleID {
			if position != nil && !ValidCanvasPoint(*position) {
				return "", fmt.Errorf("位置超出画布范围")
			}
			m.RoomPosition = position
			if rotation != nil {
				m.RoomRotation = *rotation
			}
			return "移动机房内微模块：" + m.Name, nil
		}
	}
	return "", fmt.Errorf("微模块不存在")
}
func MoveInlineInfrastructure(s *State, id, side string, afterRack int, beforeID string) (string, error) {
	var item *Infra
	for i := range s.Infra {
		if s.Infra[i].ID == id {
			item = &s.Infra[i]
			break
		}
	}
	if item == nil {
		return "", fmt.Errorf("基础设施不存在")
	}
	if item.Placement != "inline" {
		return "", fmt.Errorf("仅列内独立设施可直接拖动；柜内集成设施随柜移动")
	}
	if beforeID == id {
		return "", fmt.Errorf("目标不能是设施自身")
	}
	updated := *item
	updated.Side = side
	updated.AfterRack = afterRack
	if e := ValidateInfra(s, updated); e != nil {
		return "", e
	}
	siblings := []int{}
	for i, v := range s.Infra {
		if v.ID != id && v.ModuleID == item.ModuleID && v.Placement == "inline" && v.Side == side && v.AfterRack == afterRack {
			siblings = append(siblings, i)
		}
	}
	sort.SliceStable(siblings, func(i, j int) bool { return s.Infra[siblings[i]].Order < s.Infra[siblings[j]].Order })
	insert := len(siblings)
	if beforeID != "" {
		insert = -1
		for i, index := range siblings {
			if s.Infra[index].ID == beforeID {
				insert = i
				break
			}
		}
		if insert < 0 {
			return "", fmt.Errorf("插入位置必须是同模块同列同一柜间位置的独立设施")
		}
	}
	item.Side = side
	item.AfterRack = afterRack
	item.Order = insert
	for i, index := range siblings {
		order := i
		if i >= insert {
			order++
		}
		s.Infra[index].Order = order
	}
	return fmt.Sprintf("移动设施：%s → %s列第%d柜后", item.Name, side, afterRack), nil
}
