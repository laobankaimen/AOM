package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type RoomEntrance struct {
	Side   string  `json:"side"`
	Offset float64 `json:"offset"`
}
type Site struct {
	Entrance  *RoomEntrance `json:"entrance,omitempty"`
	ProjectID string        `json:"projectId"`
	ID        string        `json:"id"`
	Name      string        `json:"name"`
}
type Module struct {
	RoomRotation int          `json:"roomRotation,omitempty"`
	RoomPosition *CanvasPoint `json:"roomPosition,omitempty"`
	ProductModel string       `json:"productModel"`
	Point        string       `json:"point"`
	ID           string       `json:"id"`
	SiteID       string       `json:"siteId"`
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	RackLeft     int          `json:"rackLeft"`
	RackRight    int          `json:"rackRight"`
}
type Rack struct {
	ID       string `json:"id"`
	ModuleID string `json:"moduleId"`
	Name     string `json:"name"`
	Side     string `json:"side"`
	Index    int    `json:"index"`
	Capacity int    `json:"capacity"`
}
type Infra struct {
	Order     int    `json:"order,omitempty"`
	Side      string `json:"side"`
	AfterRack int    `json:"afterRack"`
	Placement string `json:"placement"`
	ID        string `json:"id"`
	ModuleID  string `json:"moduleId"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}
type Model struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Vendor  string `json:"vendor"`
	Role    string `json:"role"`
	UHeight int    `json:"uHeight"`
	Ports   string `json:"ports"`
}
type Device struct {
	TopologyPosition *CanvasPoint `json:"topologyPosition,omitempty"`
	ID               string       `json:"id"`
	AssetTag         string       `json:"assetTag"`
	Name             string       `json:"name"`
	ModelID          string       `json:"modelId"`
	ModuleID         string       `json:"moduleId"`
	RackID           string       `json:"rackId"`
	StartU           int          `json:"startU"`
	IP               string       `json:"ip"`
	SN               string       `json:"sn"`
	Status           string       `json:"status"`
	Note             string       `json:"note"`
	Source           string       `json:"source"`
}
type State struct {
	DeviceRoles   []DeviceRole     `json:"deviceRoles,omitempty"`
	Templates     []ModuleTemplate `json:"templates"`
	Networks      []Network        `json:"networks"`
	Links         []Link           `json:"links"`
	Projects      []Project        `json:"projects"`
	SchemaVersion int              `json:"schemaVersion"`
	Revision      int              `json:"revision"`
	Sites         []Site           `json:"sites"`
	Modules       []Module         `json:"modules"`
	Racks         []Rack           `json:"racks"`
	Infra         []Infra          `json:"infra"`
	Models        []Model          `json:"models"`
	Devices       []Device         `json:"devices"`
}
type ModulePlan struct {
	RoomRotation int                 `json:"room_rotation,omitempty"`
	RoomPosition *CanvasPoint        `json:"room_position,omitempty"`
	RackNames    map[string][]string `json:"rack_names,omitempty"`
	ProductModel string              `json:"product_model"`
	Point        string              `json:"point"`
	Layout       []InfraSlot         `json:"layout"`
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	RackCount    int                 `json:"rack_count"`
	RackLeft     int                 `json:"rack_left"`
	RackRight    int                 `json:"rack_right"`
	UHeight      int                 `json:"u_height"`
	Infra        *InfraPlan          `json:"infra,omitempty"`
}
type InfraPlan struct {
	Power int `json:"power"`
	AC    int `json:"ac"`
	Env   int `json:"env"`
}
type InfraSlot struct {
	Order     int    `json:"order,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Side      string `json:"side"`
	AfterRack int    `json:"after_rack"`
	Placement string `json:"placement"`
}
type Plan struct {
	ProjectID   string       `json:"project_id,omitempty"`
	ProjectName string       `json:"project_name,omitempty"`
	SiteName    string       `json:"site_name"`
	SiteID      string       `json:"site_id,omitempty"`
	Modules     []ModulePlan `json:"modules"`
	Infra       InfraPlan    `json:"infra"`
}

func ID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
func Clone(s State) State { b, _ := json.Marshal(s); var v State; _ = json.Unmarshal(b, &v); return v }
func (s *State) Model(id string) *Model {
	for i := range s.Models {
		if s.Models[i].ID == id {
			return &s.Models[i]
		}
	}
	return nil
}
func (s *State) Rack(id string) *Rack {
	for i := range s.Racks {
		if s.Racks[i].ID == id {
			return &s.Racks[i]
		}
	}
	return nil
}
func (s *State) Device(id string) *Device {
	for i := range s.Devices {
		if s.Devices[i].ID == id {
			return &s.Devices[i]
		}
	}
	return nil
}
func (s *State) HasModule(id string) bool {
	for _, m := range s.Modules {
		if m.ID == id {
			return true
		}
	}
	return false
}
func textOK(s string, max int) bool { return strings.TrimSpace(s) != "" && len([]rune(s)) <= max }

func Validate(s *State) error {
	if s.SchemaVersion != 5 {
		return fmt.Errorf("不支持的数据版本")
	}
	if len(s.Devices) > 10000 || len(s.Racks) > 1000 || len(s.Models) > 1000 {
		return fmt.Errorf("超过当前版本容量上限（10000台设备/1000柜/1000型号）")
	}
	ids := map[string]bool{}
	checkID := func(id string) bool {
		if id == "" || len(id) > 100 || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	projects := map[string]bool{}
	for _, p := range s.Projects {
		if !checkID(p.ID) || !textOK(p.Name, 80) {
			return fmt.Errorf("项目名称或ID无效")
		}
		projects[p.ID] = true
	}
	sites := map[string]bool{}
	modules := map[string]bool{}
	for _, v := range s.Sites {
		if !checkID(v.ID) || !textOK(v.Name, 80) || !projects[v.ProjectID] {
			return fmt.Errorf("机房名称、ID或所属项目无效")
		}
		if v.Entrance != nil {
			switch v.Entrance.Side {
			case "north", "south", "east", "west":
			default:
				return fmt.Errorf("请选择有效入口方向")
			}
			if math.IsNaN(v.Entrance.Offset) || math.IsInf(v.Entrance.Offset, 0) || v.Entrance.Offset < 0 || v.Entrance.Offset > 100 {
				return fmt.Errorf("入口位置须为0–100%%")
			}
		}
		sites[v.ID] = true
	}
	for _, v := range s.Modules {
		if v.RoomRotation != 0 && v.RoomRotation != 90 && v.RoomRotation != 180 && v.RoomRotation != 270 {
			return fmt.Errorf("微模块方向须为0/90/180/270度")
		}
		if v.RoomPosition != nil && !ValidCanvasPoint(*v.RoomPosition) {
			return fmt.Errorf("微模块图位置须为0–10000范围内的有限坐标")
		}
		if !checkID(v.ID) || !sites[v.SiteID] || !textOK(v.Name, 80) || (v.Type != "single" && v.Type != "double") {
			return fmt.Errorf("模块参数无效")
		}
		if v.Type == "single" && (v.RackLeft < 1 || v.RackLeft > 24 || v.RackRight != 0) {
			return fmt.Errorf("单排机柜数应为1–24")
		}
		if v.Type == "double" && (v.RackLeft < 1 || v.RackLeft > 12 || v.RackRight < 1 || v.RackRight > 12) {
			return fmt.Errorf("双排左右机柜数应各为1–12")
		}
		if len([]rune(v.ProductModel)) > 120 || len([]rune(v.Point)) > 80 {
			return fmt.Errorf("微模块型号或点位名称过长")
		}
		modules[v.ID] = true
	}
	rackPositions := map[string]bool{}
	rackCounts := map[string]int{}
	for _, v := range s.Racks {
		if !checkID(v.ID) || !modules[v.ModuleID] || !textOK(v.Name, 80) || v.Capacity != 42 || v.Index < 1 || (v.Side != "L" && v.Side != "R") {
			return fmt.Errorf("机柜参数无效")
		}
		var m Module
		for _, item := range s.Modules {
			if item.ID == v.ModuleID {
				m = item
				break
			}
		}
		count := m.RackLeft
		if v.Side == "R" {
			count = m.RackRight
		}
		if v.Index > count {
			return fmt.Errorf("机柜位置超出所属列")
		}
		key := fmt.Sprintf("%s:%s:%d", v.ModuleID, v.Side, v.Index)
		if rackPositions[key] {
			return fmt.Errorf("机柜列内位置重复")
		}
		rackPositions[key] = true
		rackCounts[v.ModuleID+":"+v.Side]++
	}
	for _, m := range s.Modules {
		if rackCounts[m.ID+":L"] != m.RackLeft || rackCounts[m.ID+":R"] != m.RackRight {
			return fmt.Errorf("机柜数量与列规划不一致")
		}
	}

	for _, v := range s.Infra {
		if !checkID(v.ID) || !modules[v.ModuleID] || !textOK(v.Name, 80) {
			return fmt.Errorf("基础设施参数无效")
		}
		if err := ValidateInfra(s, v); err != nil {
			return err
		}
	}

	if err := ValidateDeviceRoles(s); err != nil {
		return err
	}
	roles := map[string]bool{}
	for _, role := range s.RoleDefinitions() {
		roles[role.ID] = true
	}
	names := map[string]bool{}
	for _, v := range s.Models {
		key := strings.ToLower(strings.TrimSpace(v.Name))
		if !checkID(v.ID) || !textOK(v.Name, 100) || names[key] || v.UHeight < 0 || v.UHeight > 42 || len([]rune(v.Vendor)) > 80 || len([]rune(v.Ports)) > 500 {
			return fmt.Errorf("型号名称重复或高度无效（0代表待确认，最大42U）")
		}
		names[key] = true
		if !roles[v.Role] {
			return fmt.Errorf("型号角色无效，请先添加设备角色")
		}
	}
	assets := map[string]bool{}
	sns := map[string]bool{}
	occupied := map[string]string{}
	for _, v := range s.Devices {
		if v.TopologyPosition != nil && !ValidCanvasPoint(*v.TopologyPosition) {
			return fmt.Errorf("设备图位置超出范围")
		}
		asset := strings.ToLower(strings.TrimSpace(v.AssetTag))
		sn := strings.ToLower(strings.TrimSpace(v.SN))
		model := s.Model(v.ModelID)
		if !checkID(v.ID) || !textOK(v.AssetTag, 80) || !textOK(v.Name, 120) || assets[asset] || model == nil || !modules[v.ModuleID] {
			return fmt.Errorf("设备名称、资产编号、型号或目标模块无效；资产编号不得重复")
		}
		assets[asset] = true
		if len([]rune(v.Note)) > 2000 || len([]rune(v.SN)) > 120 {
			return fmt.Errorf("设备备注或SN过长")
		}
		if sn != "" {
			if sns[sn] {
				return fmt.Errorf("SN重复：%s", v.SN)
			}
			sns[sn] = true
		}
		if v.IP != "" && net.ParseIP(v.IP) == nil {
			return fmt.Errorf("IP地址无效：%s", v.IP)
		}
		if v.Status != "pending" && v.Status != "installed" && v.Status != "maintenance" {
			return fmt.Errorf("设备状态无效")
		}
		if v.RackID == "" {
			if v.StartU != 0 || v.Status == "installed" {
				return fmt.Errorf("待上架设备的U位必须为0，状态不能为已上架")
			}
			continue
		}
		rack := s.Rack(v.RackID)
		if rack == nil || rack.ModuleID != v.ModuleID {
			return fmt.Errorf("机柜不存在或不属于目标模块")
		}
		if model.UHeight == 0 {
			return fmt.Errorf("型号 %s 的U高待确认，不能上架", model.Name)
		}
		if v.StartU < 1 || v.StartU+model.UHeight-1 > rack.Capacity {
			return fmt.Errorf("设备 %s 超出1–42U范围", v.Name)
		}
		if v.Status == "pending" {
			return fmt.Errorf("已有机位的设备不能标为待上架")
		}
		for u := v.StartU; u < v.StartU+model.UHeight; u++ {
			key := fmt.Sprintf("%s:%d", v.RackID, u)
			if other := occupied[key]; other != "" {
				return fmt.Errorf("U位冲突：%s 与 %s 占用相同U位", v.Name, other)
			}
			occupied[key] = v.Name
		}
	}
	if e := ValidateTemplates(s, checkID); e != nil {
		return e
	}
	return ValidateNetwork(s, checkID)
}

func AddPlan(s *State, p Plan) (string, error) {
	if len(p.Modules) < 1 || len(p.Modules) > 8 {
		return "", fmt.Errorf("一次创建需包含1–8个模块")
	}
	for _, n := range []int{p.Infra.Power, p.Infra.AC, p.Infra.Env} {
		if n < 0 || n > 8 {
			return "", fmt.Errorf("每类基础设施槽位数须为0–8")
		}
	}
	projectID := p.ProjectID
	if p.SiteID != "" {
		for _, site := range s.Sites {
			if site.ID == p.SiteID {
				if projectID != "" && projectID != site.ProjectID {
					return "", fmt.Errorf("目标机房不属于所选项目")
				}
				projectID = site.ProjectID
			}
		}
	}
	if projectID == "" && strings.TrimSpace(p.ProjectName) != "" {
		name := strings.TrimSpace(p.ProjectName)
		if !textOK(name, 80) {
			return "", fmt.Errorf("项目名称须为1–80字")
		}
		for _, project := range s.Projects {
			if project.Name == name {
				return "", fmt.Errorf("项目名称已存在，请选择已有项目")
			}
		}
		projectID = ID("project")
		s.Projects = append(s.Projects, Project{projectID, name})
	}
	if projectID == "" && len(s.Projects) > 0 {
		projectID = s.Projects[0].ID
	}
	foundProject := false
	for _, project := range s.Projects {
		if project.ID == projectID {
			foundProject = true
		}
	}
	if !foundProject {
		return "", fmt.Errorf("请选择项目或填写新项目名称")
	}
	siteID := p.SiteID
	if siteID == "" {
		p.SiteName = strings.TrimSpace(p.SiteName)
		if !textOK(p.SiteName, 80) {
			return "", fmt.Errorf("请输入1–80字机房名称")
		}
		for _, site := range s.Sites {
			if site.Name == p.SiteName && site.ProjectID == projectID {
				return "", fmt.Errorf("同名机房已存在，请选择向已有机房增加模块")
			}
		}
		siteID = ID("site")
		s.Sites = append(s.Sites, Site{ID: siteID, Name: p.SiteName, ProjectID: projectID})
	} else {
		found := false
		for _, v := range s.Sites {
			if v.ID == siteID {
				found = true
				p.SiteName = v.Name
			}
		}
		if !found {
			return "", fmt.Errorf("目标机房不存在")
		}
	}
	for _, v := range p.Modules {
		infra := p.Infra
		if v.Infra != nil {
			infra = *v.Infra
		}
		for _, n := range []int{infra.Power, infra.AC, infra.Env} {
			if n < 0 || n > 8 {
				return "", fmt.Errorf("每类基础设施槽位数须为0–8")
			}
		}
		if v.UHeight != 42 {
			return "", fmt.Errorf("本期仅支持42U机柜")
		}
		for _, m := range s.Modules {
			if m.SiteID == siteID && m.Name == strings.TrimSpace(v.Name) {
				return "", fmt.Errorf("模块名称重复：%s", v.Name)
			}
		}
		m := Module{RoomRotation: v.RoomRotation, RoomPosition: v.RoomPosition, ProductModel: strings.TrimSpace(v.ProductModel), Point: strings.TrimSpace(v.Point), ID: ID("mod"), SiteID: siteID, Name: strings.TrimSpace(v.Name), Type: v.Type, RackLeft: v.RackLeft, RackRight: v.RackRight}
		if v.Type == "single" {
			m.RackLeft = v.RackCount
			m.RackRight = 0
		}
		if !textOK(m.Name, 80) || (m.Type != "single" && m.Type != "double") || m.RackLeft < 1 || m.RackLeft > 24 || (m.Type == "double" && (m.RackLeft > 12 || m.RackRight < 1 || m.RackRight > 12)) {
			return "", fmt.Errorf("模块名称、类型或机柜数量无效")
		}
		s.Modules = append(s.Modules, m)
		for _, side := range []string{"L", "R"} {
			count := m.RackLeft
			if side == "R" {
				count = m.RackRight
			}
			for i := 1; i <= count; i++ {
				name := fmt.Sprintf("%s%02d", side, i)
				if names, ok := v.RackNames[side]; ok {
					if len(names) != count {
						return "", fmt.Errorf("导入机柜名称数量与列数量不一致")
					}
					name = strings.TrimSpace(names[i-1])
					if !textOK(name, 80) {
						return "", fmt.Errorf("导入机柜名称无效")
					}
				}
				s.Racks = append(s.Racks, Rack{ID: ID("rack"), ModuleID: m.ID, Name: name, Side: side, Index: i, Capacity: 42})
			}
		}
		if v.Layout != nil {
			if len(v.Layout) > 48 {
				return "", fmt.Errorf("每模块基础设施最多48项")
			}
			for _, slot := range v.Layout {
				s.Infra = append(s.Infra, Infra{ID: ID("infra"), ModuleID: m.ID, Kind: slot.Kind, Name: slot.Name, Side: slot.Side, AfterRack: slot.AfterRack, Placement: slot.Placement, Order: slot.Order})
			}
		} else {
			index := 0
			for _, x := range []struct {
				kind, name string
				n          int
			}{{"power", "配电", infra.Power}, {"ac", "空调", infra.AC}, {"env", "环境监测", infra.Env}} {
				for i := 1; i <= x.n; i++ {
					side := "L"
					if m.Type == "double" && i%2 == 0 {
						side = "R"
					}
					after := 0
					if x.kind == "ac" {
						after = m.RackLeft / 2
						if side == "R" {
							after = m.RackRight / 2
						}
					}
					placement := "inline"
					if x.kind == "env" {
						placement = "integrated"
						after = 1
					}
					s.Infra = append(s.Infra, Infra{ID: ID("infra"), ModuleID: m.ID, Kind: x.kind, Name: fmt.Sprintf("%s预留 %d", x.name, i), Side: side, AfterRack: after, Placement: placement})
					index++
				}
			}
		}
	}
	if err := Validate(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("创建规划：%s · %d个模块", p.SiteName, len(p.Modules)), nil
}
func Seed() State {
	s := State{SchemaVersion: 5, Templates: DefaultTemplates(), Networks: []Network{}, Links: []Link{}, Projects: []Project{{ID: "project-default", Name: "默认项目"}}, Sites: []Site{}, Modules: []Module{}, Racks: []Rack{}, Infra: []Infra{}, Models: []Model{}, Devices: []Device{}}
	for _, m := range []Model{
		{"model-r722", "KunTai R722 K2", "KunTai", "server", 2, "2 × 网络接口（速率待确认）"},
		{"model-r624", "KunTai R624 K2", "KunTai", "server", 2, "2 × 网络接口（速率待确认）"},
		{"model-core", "RZ18001-1", "待确认", "core", 0, "框式；U高、板卡与端口待确认"},
		{"model-25g", "RZ2805-48YS8CQ", "待确认", "access", 1, "48 × 25G + 8 × 100G"},
		{"model-mgmt", "RZ0803-H48T4Y2CZ", "待确认", "management", 1, "千兆管理接入；端口详情待确认"},
		{"model-agg12", "RSZ20012", "待确认", "aggregation", 1, "端口模板待确认"},
		{"model-agg13", "RSZ20013", "待确认", "aggregation", 1, "端口模板待确认"},
		{"model-agg22", "RSZ2022", "待确认", "aggregation", 1, "端口模板待确认"},
	} {
		s.Models = append(s.Models, m)
	}
	_, err := AddPlan(&s, Plan{ProjectID: "project-default", SiteName: "主机房", Modules: []ModulePlan{
		{Name: "点位1 · 单排微模块", Point: "点位1", ProductModel: "曙光数创 C2000（SG6600）", Type: "single", RackCount: 5, UHeight: 42, Infra: &InfraPlan{1, 1, 1}},
		{Name: "点位1 · 双排微模块", Point: "点位1", ProductModel: "曙光数创 C2000（SG6600）", Type: "double", RackLeft: 4, RackRight: 4, UHeight: 42, Infra: &InfraPlan{2, 2, 2}},
		{Name: "点位2 · 单排微模块", Point: "点位2", ProductModel: "曙光数创 C500（C500-40K）", Type: "single", RackCount: 4, UHeight: 42, Layout: []InfraSlot{
			{Kind: "power", Name: "综合柜配电 · 位置待确认", Side: "L", AfterRack: 1, Placement: "integrated"},
			{Kind: "ac", Name: "综合柜制冷 · 位置待确认", Side: "L", AfterRack: 1, Placement: "integrated"},
			{Kind: "ups", Name: "综合柜UPS · 位置待确认", Side: "L", AfterRack: 1, Placement: "integrated"},
			{Kind: "env", Name: "综合柜监控 · 位置待确认", Side: "L", AfterRack: 1, Placement: "integrated"},
		}},
	}, Infra: InfraPlan{1, 1, 1}})
	if err != nil {
		panic(err)
	}
	return s
}
func Place(s *State, deviceID, rackID string, start int) error {
	d := s.Device(deviceID)
	if d == nil {
		return fmt.Errorf("设备不存在")
	}
	d.RackID = rackID
	d.StartU = start
	if rackID == "" {
		d.StartU = 0
		d.Status = "pending"
	} else {
		r := s.Rack(rackID)
		if r == nil {
			return fmt.Errorf("机柜不存在")
		}
		d.ModuleID = r.ModuleID
		if d.Status != "maintenance" {
			d.Status = "installed"
		}
	}
	return Validate(s)
}
func AutoPlace(s *State, d *Device) string {
	model := s.Model(d.ModelID)
	if model == nil || model.UHeight == 0 {
		return "型号U高待确认"
	}
	racks := []Rack{}
	for _, r := range s.Racks {
		if r.ModuleID == d.ModuleID {
			racks = append(racks, r)
		}
	}
	sort.SliceStable(racks, func(i, j int) bool {
		if racks[i].Side != racks[j].Side {
			return racks[i].Side < racks[j].Side
		}
		return racks[i].Index < racks[j].Index
	})
	for _, r := range racks {
		used := [43]bool{}
		for _, other := range s.Devices {
			if other.RackID == r.ID {
				m := s.Model(other.ModelID)
				for u := other.StartU; u < other.StartU+m.UHeight && u <= 42; u++ {
					used[u] = true
				}
			}
		}
		for start := 1; start+model.UHeight-1 <= 42; start++ {
			free := true
			for u := start; u < start+model.UHeight; u++ {
				if used[u] {
					free = false
					break
				}
			}
			if free {
				d.RackID = r.ID
				d.StartU = start
				d.Status = "installed"
				return ""
			}
		}
	}
	return "目标模块没有足够的连续U位"
}

func ValidateInfra(s *State, v Infra) error {
	if v.Order < 0 || v.Order > 48 {
		return fmt.Errorf("设施位置顺序无效")
	}
	var m *Module
	for i := range s.Modules {
		if s.Modules[i].ID == v.ModuleID {
			m = &s.Modules[i]
			break
		}
	}
	if m == nil {
		return fmt.Errorf("基础设施所属模块不存在")
	}
	switch v.Kind {
	case "power", "ac", "env", "ups", "battery":
	default:
		return fmt.Errorf("基础设施类型无效")
	}
	if v.Side != "L" && v.Side != "R" {
		return fmt.Errorf("请选择基础设施所属列")
	}
	count := m.RackLeft
	if v.Side == "R" {
		count = m.RackRight
	}
	if count == 0 || v.AfterRack < 0 || v.AfterRack > count {
		return fmt.Errorf("基础设施列或机柜位置无效")
	}
	if v.Placement != "inline" && v.Placement != "integrated" {
		return fmt.Errorf("请选择列内独立设施或柜内集成设施")
	}
	if v.Placement == "integrated" && v.AfterRack < 1 {
		return fmt.Errorf("柜内集成设施须选择所在机柜")
	}
	return nil
}
