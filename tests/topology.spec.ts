import {openNetworkPanel,closeNetworkPanel,openCanvasMenu} from './navigation';
import {test,expect} from './fixtures';
import {type APIRequestContext,type Page} from '@playwright/test';
import { readFile } from 'node:fs/promises';
import type { State, Device } from '../src/model';

async function state(request:APIRequestContext):Promise<State>{const response=await request.get('/api/state');expect(response.ok()).toBeTruthy();return response.json()}
async function fixture(request:APIRequestContext,page:Page,name:string){
 let s=await state(request);
 const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'网络机房A',modules:[{name:'网络模块',type:'single',rack_count:2,u_height:42}],infra:{power:1,ac:1,env:1}}}});
 expect(response.status()).toBe(201);s=await response.json();const project=s.projects.find(p=>p.name===name)!;const roomA=s.sites.find(r=>r.projectId===project.id)!;
 const other=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_id:project.id,site_name:'网络机房B',modules:[{name:'计算模块',type:'single',rack_count:1,u_height:42}],infra:{power:0,ac:0,env:0}}}});
 expect(other.status()).toBe(201);s=await other.json();const roomB=s.sites.find(r=>r.projectId===project.id&&r.name==='网络机房B')!;
 const devices:Device[]=[];
 for(const [name,modelId,room] of [['核心测试','model-core',roomA],['接入测试','model-25g',roomA],['服务器测试','model-r722',roomB]] as const){
  const module=s.modules.find(m=>m.siteId===room.id)!;
  const created=await request.post('/api/devices',{data:{revision:s.revision,autoPlace:true,device:{name,assetTag:project.id+'-'+name,modelId,moduleId:module.id,status:'pending'}}});
  expect(created.ok()).toBeTruthy();s=await created.json();devices.push(s.devices.find(d=>d.assetTag===project.id+'-'+name)!);
 }
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);
 await page.getByRole('navigation',{name:'主导航'}).getByRole('button',{name:'网络拓扑',exact:true}).click();
 await expect(page.getByRole('heading',{name:'网络拓扑',exact:true,level:1})).toBeVisible();
 await expect(page.locator('.topology-workspace')).toHaveCount(1);
 await expect(page.locator('.heading-actions .topology-workspace')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'网络与分层',exact:true})).toBeVisible();await openNetworkPanel(page);await expect(page.getByRole('button',{name:'新增网络分区',exact:true})).toHaveCount(1);await closeNetworkPanel(page);
 await page.getByRole('navigation',{name:'主导航'}).getByRole('button',{name:'空间总览',exact:true}).click();
 await expect(page.locator('.topology-workspace')).toHaveCount(0);
 await page.getByRole('navigation',{name:'主导航'}).getByRole('button',{name:'网络拓扑',exact:true}).click();
 await expect(page.locator('.topology-workspace')).toHaveCount(1);
 return {s,project,roomA,roomB,devices};
}
async function createNetwork(page:Page,name:string,devices:Device[]){
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增网络分区',exact:true}).click();await page.getByRole('button',{name:'高级编辑',exact:true}).click();
 const dialog=page.locator('.editor-page:visible, dialog[open]');
 await dialog.getByLabel('网络名称',{exact:true}).fill(name);
 await dialog.getByLabel('VLAN ID',{exact:true}).fill('100');
 await dialog.getByLabel('网段 CIDR',{exact:true}).fill('10.20.0.0/24');
 await dialog.getByLabel('网关地址',{exact:true}).fill('10.20.0.1');
 for(const d of devices)await dialog.getByRole('checkbox',{name:'网络成员 '+d.name,exact:true}).check();
 await dialog.getByRole('button',{name:'保存网络分区',exact:true}).click();await expect(dialog).toHaveCount(0);
}

test('双视图：网络分区、跨机房物理连接、详情、导出与设备定位',async({page,request})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 const {project,devices,roomB}=await fixture(request,page,'拓扑实链路验收');
 await createNetwork(page,'业务网络',devices);
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增链路',exact:true}).click();const dialog=page.locator('.editor-page:visible, dialog[open]');
 await dialog.getByRole('combobox',{name:'端点A设备',exact:true}).selectOption(devices[1].id);
 await dialog.getByRole('combobox',{name:'端点B设备',exact:true}).selectOption(devices[2].id);
 await dialog.getByLabel('端点 A 端口',{exact:true}).fill('Ethernet1/1');await dialog.getByLabel('端点 B 端口',{exact:true}).fill('eth0');
 await dialog.getByRole('combobox',{name:'连接确认状态',exact:true}).selectOption('confirmed');
 await dialog.getByRole('checkbox',{name:'承载网络 业务网络',exact:true}).check();
 await dialog.getByLabel('速率 Gbps',{exact:true}).fill('25');await dialog.getByLabel('介质',{exact:true}).fill('光纤');
 await dialog.getByRole('button',{name:'保存设备链路',exact:true}).click();await expect(dialog).toHaveCount(0);
 let s=await state(request);const saved=s.links.find(l=>l.projectId===project.id)!;expect(saved.sourcePort).toBe('Ethernet1/1');expect(saved.status).toBe('confirmed');
 const edgeButton=page.getByRole('button',{name:'查看链路 接入测试 到 服务器测试',exact:true});
 await expect(edgeButton).toBeVisible();
 const edgeBounds=await edgeButton.boundingBox();
 expect(edgeBounds?.width).toBeGreaterThan(20);
 expect(edgeBounds?.height).toBeGreaterThan(15);
 await expect(edgeButton).toContainText('25 Gbps');
 await edgeButton.click();
 await expect(edgeButton).toHaveAttribute('aria-pressed','true');
 await expect(page.locator('.topology-inspector')).toContainText('Ethernet1/1');await expect(page.locator('.topology-inspector')).toContainText('25 Gbps');
 await page.getByRole('button',{name:'查看拓扑设备 接入测试',exact:true}).click();
 await edgeButton.focus();await edgeButton.press('Enter');
 await expect(page.locator('.topology-inspector')).toContainText('Ethernet1/1');
 await page.getByRole('button',{name:'缩小拓扑',exact:true}).click();
 await page.getByRole('button',{name:'查看拓扑设备 接入测试',exact:true}).click();
 await edgeButton.click();
 await expect(edgeButton).toHaveAttribute('aria-pressed','true');
 await page.getByRole('button',{name:'重置拓扑缩放',exact:true}).click();
 await page.screenshot({path:'.cache/topology-network.png',fullPage:true});
 await openNetworkPanel(page);await page.locator('.action-menu summary').filter({hasText:'拓扑更多操作'}).click();const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'导出项目拓扑JSON'}).click();
 const exported=JSON.parse(await readFile((await (await downloadPromise).path())!,'utf8'));expect(exported.project.id).toBe(project.id);expect(exported.links).toHaveLength(1);expect(exported.networks[0].vlan).toBe(100);
 await openNetworkPanel(page);await page.getByRole('combobox',{name:'拓扑机房范围',exact:true}).selectOption(roomB.id);await closeNetworkPanel(page);await expect(page.getByText(/当前筛选隐藏了 1 条/)).toBeVisible();
 await page.getByRole('button',{name:'查看拓扑设备 服务器测试',exact:true}).click();await expect(page.locator('.topology-inspector')).toContainText('业务网络');
 await page.getByRole('button',{name:'在机房中定位'}).click();await expect(page.getByRole('heading',{name:'42U 机柜立面',exact:true})).toBeVisible();
 await expect(page.getByRole('navigation',{name:'位置导航'})).toContainText(roomB.name);
 await expect(page.getByRole('button',{name:'服务器测试 U1',exact:true})).toBeVisible();
 expect(errors).toEqual([]);
});

test('网络推演预览只读、待确认连接及端口冲突保护',async({page,request})=>{
 const {devices,project}=await fixture(request,page,'拓扑推演验收');await createNetwork(page,'管理网络',devices);
 await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 管理网络',exact:true}).click();const before=await state(request);
 await openNetworkPanel(page);await page.locator('.action-menu summary').filter({hasText:'拓扑更多操作'}).click();await page.getByRole('button',{name:'按型号推演',exact:true}).click();const dialog=page.locator('.editor-page:visible, dialog[open]');await expect(dialog.getByText(/2 条候选连接/)).toBeVisible();expect((await state(request)).revision).toBe(before.revision);
 await dialog.getByRole('button',{name:'记录待确认连接',exact:true}).click();await expect(dialog).toHaveCount(0);
 let s=await state(request);const links=s.links.filter(l=>l.projectId===project.id);expect(links).toHaveLength(2);expect(links.every(l=>l.status==='pending'&&l.kind==='logical'&&l.origin==='inferred')).toBeTruthy();
 await openCanvasMenu(page);await page.getByRole('button',{name:'链路清单',exact:true}).click();const row=page.locator('.link-table tbody tr').filter({hasText:'服务器测试'});
 await row.getByRole('button',{name:'编辑链路 接入测试 到 服务器测试',exact:true}).click();
 await dialog.getByRole('combobox',{name:'连接类型',exact:true}).selectOption('physical');await dialog.getByRole('combobox',{name:'连接确认状态',exact:true}).selectOption('confirmed');
 await dialog.getByRole('button',{name:'保存设备链路',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('两端端口');
 await dialog.getByLabel('端点 A 端口',{exact:true}).fill('port1');await dialog.getByLabel('端点 B 端口',{exact:true}).fill('eth0');await dialog.getByRole('button',{name:'保存设备链路',exact:true}).click();await expect(dialog).toHaveCount(0);
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增链路',exact:true}).click();await dialog.getByRole('combobox',{name:'端点A设备',exact:true}).selectOption(devices[1].id);await dialog.getByRole('combobox',{name:'端点B设备',exact:true}).selectOption(devices[0].id);
 await dialog.getByLabel('端点 A 端口',{exact:true}).fill('port1');await dialog.getByLabel('端点 B 端口',{exact:true}).fill('uplink1');
 await dialog.getByRole('button',{name:'保存设备链路',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('物理端口已被链路');
 await dialog.getByRole('button',{name:'返回查看',exact:true}).click();s=await state(request);expect(s.links.filter(l=>l.projectId===project.id)).toHaveLength(2);
 await openNetworkPanel(page);await page.getByRole('button',{name:'编辑当前网络',exact:true}).click();await page.getByRole('button',{name:'高级编辑',exact:true}).click();await dialog.getByRole('checkbox',{name:'网络成员 服务器测试',exact:true}).uncheck();await dialog.getByRole('button',{name:'保存网络分区',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('链路两端必须都属于');
});

test('快捷创建重名提示、批量分组及双上联连接',async({page,request})=>{
 const {devices,project}=await fixture(request,page,'快捷网络验收');
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增网络分区',exact:true}).click();
 const quick=page.getByRole('region',{name:'快速网络配置',exact:true});await expect(quick).toBeVisible();await expect(page.locator('.editor-page')).toHaveCount(0);
 await quick.getByLabel('网络名称',{exact:true}).fill('数据网络');await quick.getByRole('button',{name:'创建网络',exact:true}).click();
 await expect(quick).toHaveCount(0);await expect(page.getByRole('navigation',{name:'网络层级'})).toContainText('数据网络');
 await openNetworkPanel(page);await page.getByRole('button',{name:'添加现有设备',exact:true}).click();
 const members=page.getByRole('region',{name:'批量加入网络',exact:true});await members.getByRole('button',{name:/全选筛选结果/}).click();await members.getByRole('button',{name:'加入网络',exact:true}).click();
 await expect(members).toHaveCount(0);
 let s=await state(request);const network=s.networks.find(n=>n.projectId===project.id&&n.name==='数据网络')!;expect(network.deviceIds.sort()).toEqual(devices.map(d=>d.id).sort());
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增子网络',exact:true}).click();await quick.getByLabel('网络名称',{exact:true}).fill('数据网络');
 await expect(quick.getByRole('alert')).toContainText('项目中已有');await expect(quick.getByRole('button',{name:'创建网络',exact:true})).toBeDisabled();
 expect((await state(request)).revision).toBe(s.revision);
 await quick.getByRole('button',{name:'打开已有网络',exact:true}).click();await expect(quick).toHaveCount(0);
 await openNetworkPanel(page);await page.getByRole('button',{name:'批量连线',exact:true}).click();const batch=page.getByRole('region',{name:'批量建立连接',exact:true});
 await batch.getByRole('checkbox',{name:'选择上联 核心测试',exact:true}).check();await batch.getByRole('checkbox',{name:'选择上联 接入测试',exact:true}).check();
 await batch.getByRole('checkbox',{name:'选择设备 服务器测试',exact:true}).check();await expect(batch).toContainText('将新增 2 条');
 const revision=s.revision;expect((await state(request)).revision).toBe(revision);
 await batch.getByRole('button',{name:'确认建立连接',exact:true}).click();await expect(batch).toHaveCount(0);
 s=await state(request);expect(s.revision).toBe(revision+1);const links=s.links.filter(l=>l.projectId===project.id);expect(links).toHaveLength(2);expect(links.every(l=>l.kind==='logical'&&l.status==='pending'&&!l.sourcePort&&!l.targetPort)).toBe(true);
 await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption('auto');await expect(page.getByRole('group',{name:'设备连接与网络分区图'})).toBeVisible();
 await page.getByRole('button',{name:'查看拓扑设备 服务器测试',exact:true}).click();await expect(page.getByRole('button',{name:'查看拓扑设备 服务器测试',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'查看拓扑设备 核心测试',exact:true})).toBeVisible();
});

test('多网络自动排列紧凑，界面收起恢复不修改业务数据',async({page,request})=>{
 const {project,devices}=await fixture(request,page,'紧凑网络与显示验收');let s=await state(request);
 for(const [name,ids] of [['空分区',[]],['计算分区',[devices[2].id]],['交换分区',[devices[0].id,devices[1].id]]] as [string,string[]][]){
  const response=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,name,purpose:'business',color:'#497cda',deviceIds:ids}}});expect(response.ok()).toBe(true);s=await response.json();
 }
 const net=s.networks.find(n=>n.projectId===project.id&&n.name==='计算分区')!,empty=s.networks.find(n=>n.projectId===project.id&&n.name==='空分区')!;
 const moved=await request.post(`/api/networks/${net.id}/position`,{data:{revision:s.revision,deviceId:devices[2].id,position:{x:2400,y:1000}}});expect(moved.ok()).toBe(true);s=await moved.json();
 await page.reload();await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await page.getByRole('navigation',{name:'主导航'}).getByRole('button',{name:'网络拓扑',exact:true}).click();
 await expect(page.locator(`[data-topology-lane="${empty.id}"]`)).toHaveCount(0);
 await openCanvasMenu(page);await page.getByRole('button',{name:'自动排列',exact:true}).click();await expect.poll(async()=>(await state(request)).revision).toBe(s.revision+1);
 let after=await state(request);expect(after.links).toEqual(s.links);expect(after.devices).toEqual(s.devices);expect(after.networks.find(n=>n.id===net.id)?.devicePositions?.[devices[2].id]).toEqual({x:0,y:0});
 const svg=page.getByRole('group',{name:'设备连接与网络分区图',exact:true});
 const viewBox=(await svg.getAttribute('viewBox'))!.split(' ').map(Number);expect(viewBox[2]).toBeLessThan(1400);expect(viewBox[3]).toBeLessThan(1000);
 const server=page.getByRole('button',{name:'查看拓扑设备 服务器测试',exact:true});await expect(server).toBeVisible();
 const revision=after.revision;
 await page.getByRole('button',{name:'专注画布',exact:true}).click();
 await expect(page.locator('.sidebar')).not.toBeVisible();await expect(page.locator('.network-summary')).not.toBeVisible();await expect(page.locator('.network-chips')).not.toBeVisible();await expect(page.locator('.graph-toolbar')).not.toBeVisible();await expect(svg).toBeVisible();
 await server.click();await expect(server).toHaveAttribute('aria-pressed','true');await expect(page.locator('.topology-inspector')).toBeVisible();await page.getByRole('button',{name:'收起拓扑详情',exact:true}).click();await expect(page.locator('.topology-inspector')).not.toBeVisible();await expect(server).toHaveAttribute('aria-pressed','true');
 await page.getByRole('button',{name:'显示详情',exact:true}).click();await expect(page.locator('.topology-inspector')).toBeVisible();
 await page.locator('.view-controls summary').click();await page.getByRole('button',{name:'恢复默认显示',exact:true}).click();
 await expect(page.locator('.sidebar')).toBeVisible();await openNetworkPanel(page);await page.locator('.topology-navigation summary').filter({hasText:'项目统计'}).click();await expect(page.locator('.network-summary')).toBeVisible();await expect(page.locator('.network-chips')).toBeVisible();
 expect((await state(request)).revision).toBe(revision);
 await page.getByRole('button',{name:'收起网络列表',exact:true}).click();await page.reload();await expect(page.locator('.network-chips')).not.toBeVisible();
 await page.locator('.view-controls summary').click();await page.getByRole('button',{name:'恢复默认显示',exact:true}).click();
});
