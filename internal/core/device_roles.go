package core

import (
	"fmt"
	"regexp"
	"strings"
)

type DeviceRole struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	BaseRole string `json:"baseRole"`
}

func DefaultDeviceRoles() []DeviceRole {
	return []DeviceRole{{"server", "计算服务器", "#497cda", "server"}, {"core", "核心交换", "#8c67cf", "core"}, {"aggregation", "汇聚交换", "#a786df", "aggregation"}, {"access", "业务接入", "#1b9c88", "access"}, {"management", "管理接入", "#51b5a3", "management"}, {"storage", "存储设备", "#c49745", "storage"}, {"infra", "基础设施", "#7e8997", "infra"}}
}
func (s *State) RoleDefinitions() []DeviceRole {
	roles := DefaultDeviceRoles()
	for _, r := range s.DeviceRoles {
		found := false
		for i := range roles {
			if roles[i].ID == r.ID {
				roles[i] = r
				found = true
				break
			}
		}
		if !found {
			roles = append(roles, r)
		}
	}
	return roles
}
func (s *State) RoleBase(id string) string {
	for _, r := range s.RoleDefinitions() {
		if r.ID == id {
			return r.BaseRole
		}
	}
	return "infra"
}
func (s *State) ResolveRole(value string) string {
	for _, r := range s.RoleDefinitions() {
		if strings.EqualFold(value, r.ID) || strings.EqualFold(value, r.Name) {
			return r.ID
		}
	}
	aliases := map[string]string{"服务器": "server", "核心": "core", "汇聚": "aggregation", "汇聚交换": "aggregation", "接入": "access", "存储": "storage"}
	return aliases[value]
}
func ValidateDeviceRoles(s *State) error {
	if len(s.DeviceRoles) > 128 {
		return fmt.Errorf("设备角色最多128个")
	}
	ids, names := map[string]bool{}, map[string]bool{}
	validBase := map[string]bool{}
	for _, r := range DefaultDeviceRoles() {
		validBase[r.ID] = true
	}
	color := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	for _, r := range s.DeviceRoles {
		if ids[r.ID] || !textOK(r.ID, 80) {
			return fmt.Errorf("角色标识重复或无效")
		}
		ids[r.ID] = true
	}
	for _, r := range s.RoleDefinitions() {
		key := strings.ToLower(strings.TrimSpace(r.Name))
		if !textOK(r.Name, 40) || names[key] || !color.MatchString(r.Color) || !validBase[r.BaseRole] {
			return fmt.Errorf("角色名称须唯一且不超过40字，颜色或拓扑分类无效")
		}
		if validBase[r.ID] && r.BaseRole != r.ID {
			return fmt.Errorf("内置角色的拓扑分类不能修改")
		}
		names[key] = true
	}
	return nil
}
