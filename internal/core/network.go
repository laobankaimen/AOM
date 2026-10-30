package core

import (
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
)

type CanvasPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Network struct {
	Layers          []NetworkLayer         `json:"layers,omitempty"`
	DeviceLayers    map[string]string      `json:"deviceLayers,omitempty"`
	ParentID        string                 `json:"parentId"`
	Position        *CanvasPoint           `json:"position,omitempty"`
	DevicePositions map[string]CanvasPoint `json:"devicePositions,omitempty"`
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"projectId"`
	Name            string                 `json:"name"`
	Purpose         string                 `json:"purpose"`
	VLAN            int                    `json:"vlan"`
	CIDR            string                 `json:"cidr"`
	Gateway         string                 `json:"gateway"`
	Color           string                 `json:"color"`
	DeviceIDs       []string               `json:"deviceIds"`
	Note            string                 `json:"note"`
}
type Link struct {
	ID         string   `json:"id"`
	ProjectID  string   `json:"projectId"`
	SourceID   string   `json:"sourceId"`
	TargetID   string   `json:"targetId"`
	SourcePort string   `json:"sourcePort"`
	TargetPort string   `json:"targetPort"`
	NetworkIDs []string `json:"networkIds"`
	Kind       string   `json:"kind"`
	Medium     string   `json:"medium"`
	SpeedGbps  float64  `json:"speedGbps"`
	Status     string   `json:"status"`
	Origin     string   `json:"origin"`
	Note       string   `json:"note"`
}

var networkColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *State) DeviceProject(id string) string {
	d := s.Device(id)
	if d == nil {
		return ""
	}
	for _, m := range s.Modules {
		if m.ID == d.ModuleID {
			for _, room := range s.Sites {
				if room.ID == m.SiteID {
					return room.ProjectID
				}
			}
		}
	}
	return ""
}
func (s *State) Network(id string) *Network {
	for i := range s.Networks {
		if s.Networks[i].ID == id {
			return &s.Networks[i]
		}
	}
	return nil
}
func Contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func ValidateNetwork(s *State, checkID func(string) bool) error {
	if len(s.Networks) > 1000 || len(s.Links) > 20000 {
		return fmt.Errorf("网络分区最多1000个，链路最多20000条")
	}
	names := map[string]bool{}
	for _, n := range s.Networks {
		if err := ValidateNetworkLayers(&n); err != nil {
			return err
		}
		validProject := false
		for _, p := range s.Projects {
			if p.ID == n.ProjectID {
				validProject = true
			}
		}
		key := n.ProjectID + ":" + strings.ToLower(strings.TrimSpace(n.Name))
		if !checkID(n.ID) {
			return fmt.Errorf("网络「%s」的内部编号为空或重复，请刷新页面后重试", n.Name)
		}
		if !validProject {
			return fmt.Errorf("网络「%s」所属项目不存在，请重新选择项目", n.Name)
		}
		if !textOK(n.Name, 80) {
			return fmt.Errorf("请填写网络名称，长度不能超过80个字")
		}
		if names[key] {
			return fmt.Errorf("项目中已存在网络「%s」，请打开已有网络或换一个名称（不同子网络也不能同名）", strings.TrimSpace(n.Name))
		}
		names[key] = true
		if n.ParentID != "" {
			parent := s.Network(n.ParentID)
			if parent == nil || parent.ProjectID != n.ProjectID {
				return fmt.Errorf("父子网络必须存在且属于同一项目")
			}
		}
		ancestors := map[string]bool{n.ID: true}
		cursor := n.ParentID
		depth := 0
		for cursor != "" {
			if ancestors[cursor] {
				return fmt.Errorf("子网络不能移动到自身或后代之下")
			}
			ancestors[cursor] = true
			parent := s.Network(cursor)
			if parent == nil {
				return fmt.Errorf("父网络不存在")
			}
			cursor = parent.ParentID
			depth++
			if depth >= 8 {
				return fmt.Errorf("子网络层级最多8层")
			}
		}
		if n.Position != nil && !ValidCanvasPoint(*n.Position) {
			return fmt.Errorf("网络图位置超出范围")
		}
		for id, point := range n.DevicePositions {
			if !Contains(n.DeviceIDs, id) || !ValidCanvasPoint(point) {
				return fmt.Errorf("设备图位置或网络成员无效")
			}
		}

		if n.VLAN < 0 || n.VLAN > 4094 {
			return fmt.Errorf("VLAN须为1–4094；0表示未指定")
		}
		if !networkColor.MatchString(n.Color) {
			return fmt.Errorf("网络颜色须为#RRGGBB")
		}
		switch n.Purpose {
		case "business", "management", "storage", "interconnect", "other":
		default:
			return fmt.Errorf("网络用途无效")
		}
		if len([]rune(n.Note)) > 2000 {
			return fmt.Errorf("网络备注过长")
		}
		var subnet *net.IPNet
		if n.CIDR != "" {
			ip, block, e := net.ParseCIDR(n.CIDR)
			if e != nil || !ip.Equal(block.IP) {
				return fmt.Errorf("网段须为有效网络地址CIDR，例如10.0.0.0/24")
			}
			subnet = block
		}
		if n.Gateway != "" {
			ip := net.ParseIP(n.Gateway)
			if ip == nil {
				return fmt.Errorf("网关地址无效")
			}
			if subnet == nil || !subnet.Contains(ip) {
				return fmt.Errorf("网关须属于已填写的网段")
			}
		}
		seen := map[string]bool{}
		for _, id := range n.DeviceIDs {
			if seen[id] || s.DeviceProject(id) != n.ProjectID {
				return fmt.Errorf("网络成员重复、不存在或属于其他项目")
			}
			seen[id] = true
		}
	}
	ports := map[string]string{}
	logical := map[string]bool{}
	for _, l := range s.Links {
		if !checkID(l.ID) || l.SourceID == l.TargetID || l.ProjectID == "" || s.DeviceProject(l.SourceID) != l.ProjectID || s.DeviceProject(l.TargetID) != l.ProjectID {
			return fmt.Errorf("链路两端必须是同项目内的不同设备")
		}
		if l.Kind != "physical" && l.Kind != "logical" {
			return fmt.Errorf("链路类型须为物理或逻辑")
		}
		if l.Status != "pending" && l.Status != "confirmed" {
			return fmt.Errorf("链路须为待确认或已确认")
		}
		if l.Origin != "manual" && l.Origin != "inferred" {
			return fmt.Errorf("链路来源无效")
		}
		if math.IsNaN(l.SpeedGbps) || math.IsInf(l.SpeedGbps, 0) || l.SpeedGbps < 0 || l.SpeedGbps > 100000 {
			return fmt.Errorf("速率范围无效，0表示未确认")
		}
		if len([]rune(l.SourcePort)) > 80 || len([]rune(l.TargetPort)) > 80 || len([]rune(l.Medium)) > 80 || len([]rune(l.Note)) > 2000 {
			return fmt.Errorf("链路端口、介质或备注过长")
		}
		if l.Kind == "physical" {
			if l.Status == "confirmed" && (strings.TrimSpace(l.SourcePort) == "" || strings.TrimSpace(l.TargetPort) == "") {
				return fmt.Errorf("确认物理链路前须填写两端端口")
			}
			for _, p := range []struct{ id, port string }{{l.SourceID, l.SourcePort}, {l.TargetID, l.TargetPort}} {
				if strings.TrimSpace(p.port) != "" {
					key := p.id + ":" + strings.ToLower(strings.TrimSpace(p.port))
					if old := ports[key]; old != "" {
						return fmt.Errorf("物理端口已被链路%s占用（含待确认预占）", old)
					}
					ports[key] = l.ID
				}
			}
		} else {
			a, b := l.SourceID, l.TargetID
			if a > b {
				a, b = b, a
			}
			key := a + ":" + b
			if logical[key] {
				return fmt.Errorf("同一对设备已存在逻辑链路，请编辑其网络归属")
			}
			logical[key] = true
		}
		seen := map[string]bool{}
		for _, id := range l.NetworkIDs {
			n := s.Network(id)
			if n == nil || n.ProjectID != l.ProjectID || seen[id] {
				return fmt.Errorf("链路网络归属无效或重复")
			}
			seen[id] = true
			if !Contains(n.DeviceIDs, l.SourceID) || !Contains(n.DeviceIDs, l.TargetID) {
				return fmt.Errorf("链路两端必须都属于所选网络分区：%s", n.Name)
			}
		}
	}
	return nil
}

// Suggest only logical, pending associations within explicitly chosen members.
// Never invent physical ports, redundancy, VLANs or IP assignments.
func SuggestLinks(s State, networkID string) ([]Link, error) {
	n := s.Network(networkID)
	if n == nil {
		return nil, fmt.Errorf("请选择网络分区并录入成员")
	}
	levels := map[string]int{"core": 0, "aggregation": 1, "access": 2, "management": 2, "server": 3, "storage": 3}
	result := []Link{}
	for _, id := range n.DeviceIDs {
		child := s.Device(id)
		m := s.Model(child.ModelID)
		level, ok := levels[m.Role]
		if !ok || level == 0 {
			continue
		}
		linked := false
		for _, l := range s.Links {
			if (l.SourceID == id || l.TargetID == id) && Contains(l.NetworkIDs, n.ID) {
				other := l.SourceID
				if other == id {
					other = l.TargetID
				}
				if device := s.Device(other); device != nil {
					parentLevel, exists := levels[s.Model(device.ModelID).Role]
					if exists && parentLevel < level {
						linked = true
					}
				}
			}
		}
		if linked {
			continue
		}
		var parent *Device
		bestLevel := -1
		bestLocal := false
		for _, candidateID := range n.DeviceIDs {
			candidate := s.Device(candidateID)
			candidateLevel, ok := levels[s.Model(candidate.ModelID).Role]
			if !ok || candidateLevel >= level {
				continue
			}
			local := candidate.ModuleID == child.ModuleID
			if parent == nil || candidateLevel > bestLevel || (candidateLevel == bestLevel && local && !bestLocal) {
				parent = candidate
				bestLevel = candidateLevel
				bestLocal = local
			}
		}
		if parent == nil {
			continue
		}
		duplicate := false
		for _, l := range s.Links {
			if (l.SourceID == parent.ID && l.TargetID == id) || (l.TargetID == parent.ID && l.SourceID == id) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		result = append(result, Link{ID: ID("link"), ProjectID: n.ProjectID, SourceID: parent.ID, TargetID: id, NetworkIDs: []string{n.ID}, Kind: "logical", Status: "pending", Origin: "inferred", Note: "按型号层级推演；连接、冗余与端口均需现场确认"})
	}
	return result, nil
}

func ValidCanvasPoint(p CanvasPoint) bool {
	return !math.IsNaN(p.X) && !math.IsInf(p.X, 0) && !math.IsNaN(p.Y) && !math.IsInf(p.Y, 0) && p.X >= 0 && p.X <= 10000 && p.Y >= 0 && p.Y <= 10000
}
func RemoveID(ids []string, id string) []string {
	out := []string{}
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}

// Moving membership preserves actual assets and cables. Only obsolete network
// tags are removed from incident links; no new network connectivity is inferred.
func MoveNetworkMember(s *State, deviceID, sourceID, targetID, mode string) (string, error) {
	if mode != "move" && mode != "add" && mode != "remove" {
		return "", fmt.Errorf("成员操作无效")
	}
	d := s.Device(deviceID)
	if d == nil {
		return "", fmt.Errorf("设备不存在")
	}
	project := s.DeviceProject(deviceID)
	source, target := s.Network(sourceID), s.Network(targetID)
	if mode == "move" || mode == "remove" {
		if source == nil || source.ProjectID != project || !Contains(source.DeviceIDs, deviceID) {
			return "", fmt.Errorf("源网络中没有此设备")
		}
	}
	if mode == "move" || mode == "add" {
		if target == nil || target.ProjectID != project {
			return "", fmt.Errorf("目标子网络须属于设备所在项目")
		}
	}
	if mode == "move" && sourceID == targetID {
		return "", fmt.Errorf("设备已经位于该网络")
	}
	changed := 0
	if mode == "move" || mode == "remove" {
		source.DeviceIDs = RemoveID(source.DeviceIDs, deviceID)
		delete(source.DevicePositions, deviceID)
		delete(source.DeviceLayers, deviceID)
		for i := range s.Links {
			l := &s.Links[i]
			if (l.SourceID == deviceID || l.TargetID == deviceID) && Contains(l.NetworkIDs, sourceID) {
				l.NetworkIDs = RemoveID(l.NetworkIDs, sourceID)
				changed++
			}
		}
	}
	if mode == "move" || mode == "add" {
		if !Contains(target.DeviceIDs, deviceID) {
			target.DeviceIDs = append(target.DeviceIDs, deviceID)
		}
	}
	if err := Validate(s); err != nil {
		return "", err
	}
	return fmt.Sprintf("调整网络成员：%s；保留设备与链路，移除%d条链路的原网络标签", d.Name, changed), nil
}
