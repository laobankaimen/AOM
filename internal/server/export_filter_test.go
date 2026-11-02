package server

import (
	"aom/internal/core"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestCSVExportScopeAndFilters(t *testing.T) {
	h, store := testAPI(t)
	s, _ := store.Snapshot()
	originalModule := s.Modules[0]
	originalRoom := s.Sites[0]
	create := func(body map[string]any, endpoint string) {
		t.Helper()
		body["revision"] = s.Revision
		b, _ := json.Marshal(body)
		response := call(h, "POST", endpoint, string(b))
		if response.Code < 200 || response.Code >= 300 {
			t.Fatal(response.Body.String())
		}
		s, _ = store.Snapshot()
	}
	create(map[string]any{"plan": map[string]any{"project_name": "另一项目", "site_name": "另一机房", "modules": []map[string]any{{"name": "另一个模块", "type": "single", "rack_count": 1, "u_height": 42}}, "infra": map[string]int{"power": 0, "ac": 0, "env": 0}}}, "/api/plans")
	var other core.Module
	for _, m := range s.Modules {
		if m.ID != originalModule.ID && m.SiteID != originalRoom.ID {
			other = m
			break
		}
	}
	for _, d := range []core.Device{
		{AssetTag: "EXPORT-1", Name: "目标服务器", SN: "SN-FIND", ModelID: "model-r722", ModuleID: originalModule.ID, Status: "pending"},
		{AssetTag: "EXPORT-2", Name: "其他设备", ModelID: "model-r722", ModuleID: originalModule.ID, Status: "maintenance"},
		{AssetTag: "EXPORT-3", Name: "目标服务器", ModelID: "model-r722", ModuleID: other.ID, Status: "pending"},
	} {
		create(map[string]any{"device": d, "autoPlace": false}, "/api/devices")
	}
	for _, tc := range []struct {
		params url.Values
		tags   []string
	}{
		{url.Values{"projectId": {originalRoom.ProjectID}}, []string{"EXPORT-1", "EXPORT-2"}},
		{url.Values{"siteId": {originalRoom.ID}, "status": {"pending"}, "query": {"sn-find"}}, []string{"EXPORT-1"}},
		{url.Values{"projectId": {originalRoom.ProjectID}, "query": {"R722"}}, []string{"EXPORT-1", "EXPORT-2"}},
		{url.Values{"projectId": {"missing"}}, nil},
		{url.Values{"scope": {"all"}, "projectId": {"missing"}}, []string{"EXPORT-1", "EXPORT-2", "EXPORT-3"}},
	} {
		t.Run(fmt.Sprint(tc.params), func(t *testing.T) {
			response := call(h, "GET", "/api/export/devices.csv?"+tc.params.Encode(), "")
			if response.Code != 200 {
				t.Fatal(response.Code)
			}
			rows, e := csv.NewReader(strings.NewReader(strings.TrimPrefix(response.Body.String(), "\ufeff"))).ReadAll()
			if e != nil {
				t.Fatal(e)
			}
			if len(rows) != len(tc.tags)+1 {
				t.Fatalf("unexpected rows %v", rows)
			}
			for i, tag := range tc.tags {
				if rows[i+1][0] != tag {
					t.Fatalf("expected %s, got %v", tag, rows[i+1])
				}
			}
		})
	}
}
