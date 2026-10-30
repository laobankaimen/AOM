package core

import (
	"fmt"
	"strings"
)

type NetworkLayer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func DefaultNetworkLayers() []NetworkLayer {
	return []NetworkLayer{{"core", "核心层"}, {"aggregation", "汇聚层"}, {"access", "接入层"}, {"compute", "计算 / 存储"}, {"other", "其他设备"}}
}
func NetworkLayers(n *Network) []NetworkLayer {
	if len(n.Layers) > 0 {
		return n.Layers
	}
	return DefaultNetworkLayers()
}
func ValidateNetworkLayers(n *Network) error {
	if len(n.Layers) > 16 {
		return fmt.Errorf("一个网络最多16个显示层")
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, l := range NetworkLayers(n) {
		name := strings.ToLower(strings.TrimSpace(l.Name))
		if !textOK(l.ID, 80) || !textOK(l.Name, 40) || ids[l.ID] || names[name] {
			return fmt.Errorf("显示层名称须为1–40字，名称和编号不能重复")
		}
		ids[l.ID] = true
		names[name] = true
	}
	for device, layer := range n.DeviceLayers {
		if !Contains(n.DeviceIDs, device) || !ids[layer] {
			return fmt.Errorf("设备分层无效，请选择本网络的设备和存在的显示层")
		}
	}
	return nil
}
