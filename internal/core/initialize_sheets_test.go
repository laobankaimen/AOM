package core

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func unifiedFixture(s State) InitRequest {
	return InitRequest{Revision: s.Revision, ProjectID: s.Projects[0].ID, SiteName: "总清单机房", DefaultTemplateID: s.Templates[0].ID, DefaultModuleName: "默认模块", UnifiedCSV: "模块,型号,数量,名称,资产编号,机柜,起始U,网络\n网络模块,RZ2805-48YS8CQ,1,接入,SW-001,L01,40,业务网\n计算模块,KunTai R722 K2,1,服务器,SRV-001,L01,1,业务网\n"}
}
func TestUnifiedInitializationCreatesAllFactsAndStableIDs(t *testing.T) {
	s := Seed()
	req := unifiedFixture(s)
	req.ProjectID = ""
	req.ProjectName = "新建导入项目"
	req.NetworksCSV = "网络名称,上级网络,用途,VLAN,网段,网关\n业务网,园区网,业务,101,10.10.0.0/24,10.10.0.1\n园区网,,互联,0,,\n"
	req.LinksCSV = "源资产编号,目标资产编号,源端口,目标端口,类型,速率,网络,确认状态\nSW-001,SRV-001,Eth1,eth0,物理,25,业务网,已确认\n"
	before := Clone(s)
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("preview modified input")
	}
	if p.NewModules != 2 || p.Devices != 2 || p.Placed != 2 || p.Networks != 2 || p.Links != 1 {
		t.Fatal("wrong preview summary")
	}
	if p.ProjectID == s.Projects[0].ID {
		t.Fatal("new project not created")
	}
	found := false
	for _, n := range p.State.Networks {
		if n.Name == "业务网" {
			found = true
			if n.VLAN != 101 || n.CIDR != "10.10.0.0/24" || n.ParentID == "" || len(n.DeviceIDs) != 2 {
				t.Fatal("network sheet facts lost")
			}
		}
	}
	if !found {
		t.Fatal("network missing")
	}
	d := p.State.Device(p.State.Links[0].TargetID)
	if d.AssetTag != "SRV-001" || d.StartU != 1 {
		t.Fatal("link/placement unstable")
	}
	if len(p.ResolvedModules) != 2 || p.ResolvedModules[0].Existing {
		t.Fatal("module resolutions missing")
	}
}
func TestKnownUReservedBeforeAutomaticEvenWhenListedLast(t *testing.T) {
	s := Seed()
	content := "型号,数量,资产编号,机柜,起始U\nKunTai R722 K2,1,AUTO,,\nKunTai R722 K2,1,FIXED,L01,1\n"
	p, e := PreviewCSV(s, s.Modules[0].ID, content)
	if e != nil {
		t.Fatal(e)
	}
	if p.Rows[0].Device.StartU != 3 || p.Rows[1].Device.StartU != 1 {
		t.Fatal("auto placement occupied fixed U")
	}
	p, e = PreviewCSV(s, s.Modules[0].ID, "型号,数量,资产编号,机柜\nKunTai R722 K2,1,SECOND,L02\n")
	if e != nil {
		t.Fatal(e)
	}
	if s.Rack(p.Rows[0].Device.RackID).Name != "L02" {
		t.Fatal("target rack ignored")
	}
}
func TestUnifiedReusesExistingAndBlocksSameFile(t *testing.T) {
	s := Seed()
	req := unifiedFixture(s)
	req.SiteID = s.Sites[0].ID
	req.UnifiedCSV = "模块,型号,数量,资产前缀\n" + s.Modules[0].Name + ",KunTai R722 K2,2,KNOWN\n"
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	if p.NewModules != 0 || !p.ResolvedModules[0].Existing || !reflect.DeepEqual(s.Racks, p.State.Racks) {
		t.Fatal("existing module altered")
	}
	_, e = PrepareInitialization(p.State, req)
	if e == nil || !strings.Contains(e.Error(), "已导入过相同清单") {
		t.Fatalf("same file not blocked: %v", e)
	}
}
func TestNewModelRequiresRoleAndNeverOverridesKnownModel(t *testing.T) {
	s := Seed()
	req := unifiedFixture(s)
	req.AllowNewModels = true
	req.UnifiedCSV = "模块,型号,数量,设备角色,U高\n自定义模块,新机型,2,服务器,\n"
	p, e := PrepareInitialization(s, req)
	if e != nil {
		t.Fatal(e)
	}
	if p.NewModels != 1 || p.Pending != 2 {
		t.Fatal("unknown height fabricated")
	}
	req.UnifiedCSV = "模块,型号,数量,设备角色,U高\n自定义模块,新机型,2,,2\n"
	if _, e = PrepareInitialization(s, req); e == nil {
		t.Fatal("unknown role guessed")
	}
	req.UnifiedCSV = "模块,型号,数量,设备角色,U高\n自定义模块,KunTai R722 K2,1,服务器,4\n"
	if _, e = PrepareInitialization(s, req); e == nil {
		t.Fatal("known height overwritten")
	}
}
func TestExplicitAssetsAndPlacementRejectErrors(t *testing.T) {
	s := Seed()
	for _, csv := range []string{
		"型号,数量,资产编号\nKunTai R722 K2,2,SAME\n",
		"型号,数量,资产编号\nKunTai R722 K2,1,SAME\nKunTai R722 K2,1,same\n",
		"型号,数量,资产编号,起始U\nKunTai R722 K2,1,FIXED,1\n",
		"型号,数量,机柜,起始U\nKunTai R722 K2,1,L01,42\n",
		"型号,数量,机柜\nKunTai R722 K2,1,不存在\n",
		"型号,数量,机柜,起始U\nRZ18001-1,1,L01,2\n",
	} {
		if _, e := PreviewCSV(s, s.Modules[0].ID, csv); e == nil {
			t.Fatalf("invalid CSV accepted: %s", csv)
		}
	}
}
func TestUnifiedOriginalLineNumbersIncludingQuotedNewlines(t *testing.T) {
	s := Seed()
	req := unifiedFixture(s)
	req.UnifiedCSV = "模块,型号,数量,备注\n甲,KunTai R722 K2,1,\"多行\n备注\"\n乙,KunTai R722 K2,1,\n甲,missing,1,\n"
	_, e := PrepareInitialization(s, req)
	if e == nil || !strings.Contains(e.Error(), "总清单第5行") {
		t.Fatalf("wrong source line: %v", e)
	}
}
func TestNetworkDefinitionConflictAndUnknownParentRollback(t *testing.T) {
	s := networkFixture()
	for _, content := range []string{"网络名称,VLAN\n业务网,999\n", "网络名称,上级网络\n子网,missing\n", "网络名称,上级网络\n甲,乙\n乙,甲\n", "网络名称,网段,网关\n新网,10.0.0.0/24,10.1.0.1\n"} {
		req := unifiedFixture(s)
		req.NetworksCSV = content
		if _, e := PrepareInitialization(s, req); e == nil {
			t.Fatal("network conflict accepted")
		}
	}
}
func TestNewProjectInvalidUnifiedBatchRollsBackEverything(t *testing.T) {
	store, e := Open(filepath.Join(t.TempDir(), "app.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	s, _ := store.Snapshot()
	req := unifiedFixture(s)
	req.ProjectID = ""
	req.ProjectName = "失败项目"
	req.LinksCSV = "source,target\nSW-001,missing\n"
	_, e = store.Apply(s.Revision, "initialize", func(next *State) (string, error) {
		p, e := PrepareInitialization(*next, req)
		if e != nil {
			return "", e
		}
		*next = p.State
		return "batch", nil
	})
	if e == nil {
		t.Fatal("unresolved link accepted")
	}
	after, _ := store.Snapshot()
	if !reflect.DeepEqual(s, after) {
		t.Fatal("partial project created")
	}
}
func TestInitializationFingerprintIgnoresHeaderLanguageBOMAndMode(t *testing.T) {
	a, e := InitializationFingerprint("m", "\ufeff模块,型号,数量,名称\r\n甲,KunTai R722 K2,1,设备\r\n")
	if e != nil {
		t.Fatal(e)
	}
	b, e := InitializationFingerprint("m", "name,quantity,model\n设备,1,KunTai R722 K2\n")
	if e != nil || a != b {
		t.Fatal("equivalent CSV mode/header not normalized")
	}
	other, _ := InitializationFingerprint("other", "name,quantity,model\n设备,1,KunTai R722 K2\n")
	if a == other {
		t.Fatal("fingerprint not scoped to module")
	}
}
