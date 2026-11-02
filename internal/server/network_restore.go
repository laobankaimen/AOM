package server

import (
	"aom/internal/core"
	"fmt"
	"net/http"
)

// Restore membership metadata atomically. Revision checking prevents an undo
// from overwriting any transaction committed since the operation being undone.
func (a *API) restoreMembership(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision  int            `json:"revision"`
		ProjectID string         `json:"projectId"`
		Networks  []core.Network `json:"networks"`
		Links     []core.Link    `json:"links"`
	}
	if !decode(w, r, &req) {
		return
	}
	state, err := a.store.Apply(req.Revision, "network.members.restore", func(s *core.State) (string, error) {
		if req.ProjectID == "" || len(req.Networks) == 0 {
			return "", fmt.Errorf("需要项目和网络归属快照")
		}
		seen := map[string]bool{}
		for _, snapshot := range req.Networks {
			n := s.Network(snapshot.ID)
			if n == nil || n.ProjectID != req.ProjectID || snapshot.ProjectID != req.ProjectID || seen[n.ID] {
				return "", fmt.Errorf("网络已变化，无法恢复归属")
			}
			seen[n.ID] = true
			n.DeviceIDs = snapshot.DeviceIDs
			n.DeviceLayers = snapshot.DeviceLayers
			n.DevicePositions = snapshot.DevicePositions
		}
		linkSeen := map[string]bool{}
		for _, snapshot := range req.Links {
			found := false
			for i := range s.Links {
				l := &s.Links[i]
				if l.ID == snapshot.ID {
					if l.ProjectID != req.ProjectID || snapshot.ProjectID != req.ProjectID || linkSeen[l.ID] {
						return "", fmt.Errorf("链路已变化，无法恢复归属")
					}
					linkSeen[l.ID] = true
					l.NetworkIDs = snapshot.NetworkIDs
					found = true
					break
				}
			}
			if !found {
				return "", fmt.Errorf("链路已删除，无法恢复归属")
			}
		}
		return "恢复网络成员及链路标签", nil
	})
	if err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, state)
}
