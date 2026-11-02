import {openNetworkPanel,closeNetworkPanel,openCanvasMenu} from './navigation';
import {test,expect} from './fixtures';
import {type Page,type APIRequestContext} from '@playwright/test';
import type {State} from '../src/model';
import {enterRoom,enterModule} from './navigation';
async function snapshot(request:APIRequestContext):Promise<State>{return (await request.get('/api/state')).json()}
async function fixture(page:Page,request:APIRequestContext,name:string){let s=await snapshot(request);const result=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'逐层机房',modules:[{name:'逐层模块',type:'single',rack_count:2,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(result.ok()).toBeTruthy();s=await result.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nRZ2805-48YS8CQ,1,树状交换机,TREE-SW-'+project.id+'\nKunTai R722 K2,1,树状服务器,TREE-SERVER-'+project.id}});expect(imported.ok()).toBeTruthy();s=await imported.json();const devices=s.devices.filter(d=>d.moduleId===module.id);await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);return {project,room,module,devices,s};}

test('首页只显示统计与机房概况，逐层查看和独立编辑',async({page,request})=>{
 const {room,module}=await fixture(page,request,'渐进浏览验收');
 await expect(page.getByRole('region',{name:'项目总览',exact:true})).toBeVisible();
 await expect(page.locator('.project-home')).toBeVisible();await expect(page.locator('.floor-panel,.elevation-card,.detail-panel,.module-tabs')).toHaveCount(0);await expect(page.locator('.overview-module')).toHaveCount(0);await expect(page.getByRole('button',{name:'导入设备',exact:true})).toHaveCount(0);
 await page.screenshot({path:'.cache/home-summary.png',fullPage:true});
 await enterRoom(page,room.name);await expect(page.locator('.overview-module')).toHaveCount(1);await expect(page.locator('.floor-panel,.elevation-card')).toHaveCount(0);
 await enterModule(page,module.name);await expect(page.locator('.floor-panel')).toBeVisible();await expect(page.locator('.project-home,.overview-module')).toHaveCount(0);
 await page.getByRole('button',{name:'编辑微模块详细信息',exact:true}).click();await expect(page.getByRole('region',{name:'微模块布局配置',exact:true})).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0);
 await page.getByRole('navigation',{name:'位置导航'}).getByRole('link',{name:`返回机房 ${room.name}`,exact:true}).click();await expect(page.locator('.editor-page')).toHaveCount(0);await expect(page.getByRole('heading',{name:'微模块总览',exact:true})).toBeVisible();
 await page.getByRole('navigation',{name:'位置导航'}).getByRole('link',{name:/返回项目/}).click();await expect(page.locator('.project-home')).toBeVisible();
});

test('子网络创建编辑、结构移动、设备布局与成员移动分离',async({page,request})=>{
 const {project,devices}=await fixture(page,request,'子网交互验收');
 let s=await snapshot(request);
 for(const [name,parent] of [['根网络',''],['子网A','根网络'],['子网B','根网络']] as const){const parentId=s.networks.find(n=>n.projectId===project.id&&n.name===parent)?.id||'';const response=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,parentId,name,purpose:'business',vlan:0,cidr:'',gateway:'',color:'#497cda',deviceIds:name==='子网A'?devices.map(d=>d.id):[],note:''}}});expect(response.ok()).toBeTruthy();s=await response.json()}
 const root=s.networks.find(n=>n.projectId===project.id&&n.name==='根网络')!,a=s.networks.find(n=>n.projectId===project.id&&n.name==='子网A')!,b=s.networks.find(n=>n.projectId===project.id&&n.name==='子网B')!;
 await page.reload();await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);
 await page.getByRole('link',{name:'网络拓扑',exact:true}).click();await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 根网络',exact:true}).click();
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增子网络',exact:true}).click();await page.getByRole('button',{name:'高级编辑',exact:true}).click();const editor=page.getByRole('region',{name:'新增网络分区',exact:true});await expect(editor).toBeVisible();await expect(editor.getByRole('combobox',{name:'上级网络',exact:true})).toHaveValue(root.id);await editor.getByLabel('网络名称',{exact:true}).fill('子网C');await editor.getByRole('button',{name:'保存网络分区',exact:true}).click();await expect(editor).toHaveCount(0);
 await openNetworkPanel(page);await page.locator(`[data-network-card="${a.id}"]`).dragTo(page.locator(`[data-network-card="${b.id}"]`));await expect.poll(async()=>(await snapshot(request)).networks.find(n=>n.id===a.id)?.parentId).toBe(b.id);
 await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 子网B',exact:true}).click();await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 子网A',exact:true}).click();
 await expect(page.getByRole('navigation',{name:'网络层级'})).toContainText('根网络');await expect(page.getByRole('navigation',{name:'网络层级'})).toContainText('子网B');
 const node=page.getByRole('button',{name:'查看拓扑设备 树状服务器',exact:true});await node.scrollIntoViewIfNeeded();const bounds=(await node.boundingBox())!;await page.mouse.move(bounds.x+35,bounds.y+25);await page.mouse.down();await page.mouse.move(bounds.x+85,bounds.y+55,{steps:12});await page.mouse.up();await expect.poll(async()=>(await snapshot(request)).networks.find(n=>n.id===a.id)?.devicePositions?.[devices[1].id]?.x).toBeGreaterThan(0);
 const beforeMove=await snapshot(request);expect(beforeMove.devices.find(d=>d.id===devices[1].id)?.rackId).toBe(devices[1].rackId);await node.click();await page.getByRole('button',{name:'设备网络设置',exact:true}).click();await page.getByRole('combobox',{name:'设备目标子网络',exact:true}).selectOption(b.id);await page.getByRole('button',{name:'移动设备',exact:true}).click();await expect.poll(async()=>(await snapshot(request)).networks.find(n=>n.id===b.id)?.deviceIds.includes(devices[1].id)).toBe(true);
 s=await snapshot(request);expect(s.networks.find(n=>n.id===a.id)!.deviceIds).not.toContain(devices[1].id);expect(s.devices.find(d=>d.id===devices[1].id)?.rackId).toBe(devices[1].rackId);
 await page.getByRole('navigation',{name:'网络层级'}).getByRole('link',{name:'子网B',exact:true}).click();await page.getByRole('button',{name:'查看拓扑设备 树状服务器',exact:true}).click();await page.getByRole('button',{name:'设备网络设置',exact:true}).click();await page.getByRole('button',{name:'从当前子网移出',exact:true}).click();s=await snapshot(request);expect(s.devices.some(d=>d.id===devices[1].id)).toBe(true);expect(s.networks.find(n=>n.id===b.id)!.deviceIds).not.toContain(devices[1].id);
 await openNetworkPanel(page);await page.getByRole('button',{name:'添加现有设备',exact:true}).click();await page.getByRole('checkbox',{name:'选择设备 树状服务器',exact:true}).check();await page.getByRole('button',{name:'加入网络',exact:true}).click();await expect.poll(async()=>(await snapshot(request)).networks.find(n=>n.id===b.id)?.deviceIds.includes(devices[1].id)).toBe(true);
 await page.getByRole('button',{name:'查看拓扑设备 树状服务器',exact:true}).click();await page.getByRole('button',{name:'编辑设备详情',exact:true}).click();await expect(page.getByRole('region',{name:'编辑设备 / 调整机位',exact:true})).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('导航入口唯一，布局选择互斥，次要操作收起且菜单不挤动画布',async({page,request})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 const {room,module,s}=await fixture(page,request,'清晰导航与选择验收');
 const nav=page.getByRole('navigation',{name:'主导航'});
 await expect(page.locator('.workspace-view-tabs')).toHaveCount(0);
 await expect(page.locator('.site-link.selected')).toHaveCount(0);
 await enterRoom(page,room.name);await enterModule(page,module.name);
 await expect(nav.getByRole('link',{name:'空间总览',exact:true})).toHaveAttribute('aria-current','page');
 await expect(page.locator('.heading-actions')).not.toContainText('创建机房');
 await expect(page.locator('.module-tabs')).toHaveCount(0);
 await expect(page.getByRole('heading',{name:'选择机柜或设施'})).toBeVisible();
 const floor=page.locator('.floor-svg');
 await expect(floor.locator('[aria-pressed="true"]')).toHaveCount(0);
 const rack=s.racks.find(r=>r.moduleId===module.id)!,infra=s.infra.find(i=>i.moduleId===module.id&&i.placement==='inline')!;
 await page.getByRole('button',{name:`选择机柜 ${rack.name}`,exact:true}).click();
 await expect(floor.locator('[aria-pressed="true"]')).toHaveCount(1);
 await expect(page.locator('.detail-panel')).toContainText(rack.name);
 await expect(page.locator('.detail-panel')).toContainText('已选中 · 机柜');
 await page.getByRole('button',{name:`查看设施 ${infra.name}`,exact:true}).click();
 await expect(floor.locator('[aria-pressed="true"]')).toHaveCount(1);
 await expect(floor.locator('[data-cabinet-id][aria-pressed="true"]')).toHaveCount(0);
 await expect(page.locator('.detail-panel')).toContainText(infra.name);
 await expect(page.locator('.detail-panel')).toContainText('已选中 · 基础设施');
 await page.getByRole('button',{name:'取消布局选择',exact:true}).click();
 await expect(floor.locator('[aria-pressed="true"]')).toHaveCount(0);
 await expect(page.getByRole('heading',{name:'选择机柜或设施'})).toBeVisible();
 const save=page.getByRole('button',{name:'保存当前布局为模板',exact:true});await expect(save).not.toBeVisible();
 const before=(await floor.boundingBox())!,beforeScroll=await page.evaluate(()=>scrollY);
 const more=page.locator('.action-menu summary').filter({hasText:'模块更多操作'});
 await more.click();await expect(save).toBeVisible();
 const after=(await floor.boundingBox())!;expect(after.y+await page.evaluate(()=>scrollY)).toBeCloseTo(before.y+beforeScroll,1);expect(after.width).toBeCloseTo(before.width,1);
 await page.keyboard.press('Escape');await expect(save).not.toBeVisible();await expect(more).toBeFocused();
 await more.click();await page.getByRole('heading',{name:module.name,level:1,exact:true}).click();await expect(save).not.toBeVisible();
 await nav.getByRole('link',{name:'网络拓扑',exact:true}).click();
 await expect(nav.getByRole('link',{name:'网络拓扑',exact:true})).toHaveAttribute('aria-current','page');
 await expect(page.locator('.topology-workspace')).toHaveCount(1);await expect(page.locator('.workspace-view-tabs')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'查看拓扑设备 树状服务器',exact:true})).toBeVisible();await page.getByRole('button',{name:'查看拓扑设备 树状服务器',exact:true}).click();
 await expect(page.locator('.topology-inspector')).toContainText('已选中 · 设备');
 await expect(page.getByRole('combobox',{name:'设备目标子网络',exact:true})).not.toBeVisible();
 await page.getByRole('button',{name:'取消拓扑选择',exact:true}).click();
 await expect(page.locator('.topology-inspector')).toContainText('选择设备或链路');
 expect(errors).toEqual([]);
});
