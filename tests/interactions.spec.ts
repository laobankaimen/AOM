import {enterRoom,enterModule} from './navigation';
import {test,expect} from './fixtures';
import {type APIRequestContext,type Page} from '@playwright/test';
import type {State} from '../src/model';
async function getState(request:APIRequestContext):Promise<State>{return (await request.get('/api/state')).json()}
async function setup(request:APIRequestContext,page:Page,name:string,count=4){let s=await getState(request);const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'交互机房',modules:[{name:'交互微模块',type:'single',rack_count:count,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(response.ok()).toBeTruthy();s=await response.json();const project=s.projects.find(p=>p.name===name)!;const room=s.sites.find(r=>r.projectId===project.id)!;const module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,2,拖动设备,UI-'+project.id}});expect(imported.ok()).toBeTruthy();s=await imported.json();const racks=s.racks.filter(r=>r.moduleId===module.id).sort((a,b)=>a.index-b.index),devices=s.devices.filter(d=>d.moduleId===module.id);
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);await enterModule(page,module.name);return {project,room,module,racks,devices,s};}
async function pointAt(page:Page,rackID:string,u:number){const svg=page.locator(`svg[data-drop-rack="${rackID}"]`);const b=(await svg.boundingBox())!;return {x:b.x+140/280*b.width,y:b.y+(36+(42-u)*20+10)/934*b.height}}

test('预览图拖动机柜、保留设备与集成设施、重载和键盘排列',async({page,request})=>{
 const {project,module,racks,devices,s}=await setup(request,page,'机柜排序验收');
 const card=page.locator('.floor-panel');
 const first=card.locator(`[data-cabinet-id="${racks[0].id}"]`),third=card.locator(`[data-cabinet-id="${racks[2].id}"]`);
 await first.scrollIntoViewIfNeeded();const a=(await first.boundingBox())!,b=(await third.boundingBox())!;
 await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(b.x+b.width*.85,b.y+b.height/2,{steps:15});await page.mouse.up();
 await expect.poll(async()=>(await getState(request)).racks.find(r=>r.id===racks[0].id)?.index).toBe(3);
 let after=await getState(request);expect(after.devices.filter(d=>d.moduleId===module.id)).toEqual(devices);
 const integrated=s.infra.find(i=>i.moduleId===module.id&&i.placement==='integrated')!;expect(after.infra.find(i=>i.id===integrated.id)?.afterRack).toBe(3);
 await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.getByRole('region',{name:'微模块布局配置'})).toHaveCount(0);
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,'交互机房');await enterModule(page,module.name);
 const row=page.locator('.floor-panel').locator('[data-cabinet-row="L"]');await expect(row.locator('[data-cabinet-id]').nth(2)).toHaveAttribute('data-cabinet-id',racks[0].id);
 await first.focus();await first.press('Alt+ArrowLeft');await expect.poll(async()=>(await getState(request)).racks.find(r=>r.id===racks[0].id)?.index).toBe(2);
 await page.screenshot({path:'.cache/reordered-cabinets.png',fullPage:true});
});

test('设备跨柜拖放、冲突回滚、取消拖动与面包屑返回',async({page,request})=>{
 await page.setViewportSize({width:1440,height:1600});const {room,module,racks,devices}=await setup(request,page,'跨柜拖放验收');
 await page.getByRole('button',{name:'立面图',exact:true}).click();
 await page.getByRole('button',{name:'两柜并排',exact:true}).click();await page.getByRole('combobox',{name:'并排目标机柜',exact:true}).selectOption(racks[1].id);
 const start=await pointAt(page,racks[0].id,1),end=await pointAt(page,racks[1].id,10);
 await page.mouse.move(start.x,start.y);await page.mouse.down();await page.mouse.move(end.x,end.y,{steps:14});await expect(page.locator('.drag-status')).toContainText('释放完成移动');await page.mouse.up();
 await expect.poll(async()=>(await getState(request)).devices.find(d=>d.id===devices[0].id)?.rackId).toBe(racks[1].id);
 let after=await getState(request);expect(after.devices.find(d=>d.id===devices[0].id)?.startU).toBe(10);const revision=after.revision;
 const conflictStart=await pointAt(page,racks[1].id,10),conflictEnd=await pointAt(page,racks[0].id,3);
 await page.mouse.move(conflictStart.x,conflictStart.y);await page.mouse.down();await page.mouse.move(conflictEnd.x,conflictEnd.y,{steps:14});await expect(page.locator('.drag-status')).toContainText('已占用');await page.mouse.up();
 await expect(page.locator('.drag-status')).toContainText('已保留原机位');after=await getState(request);expect(after.revision).toBe(revision);expect(after.devices.find(d=>d.id===devices[0].id)?.rackId).toBe(racks[1].id);
 const cancelStart=await pointAt(page,racks[1].id,10),cancelEnd=await pointAt(page,racks[0].id,15);await page.mouse.move(cancelStart.x,cancelStart.y);await page.mouse.down();await page.mouse.move(cancelEnd.x,cancelEnd.y,{steps:10});await page.keyboard.press('Escape');await page.mouse.up();expect((await getState(request)).revision).toBe(revision);
 const crumbs=page.getByRole('navigation',{name:'位置导航'});await expect(crumbs).toContainText(room.name);await expect(crumbs).toContainText(module.name);await expect(crumbs).toContainText(racks[1].name);
 await page.locator('.selected-device').getByRole('button',{name:'编辑详细信息',exact:true}).click();const editor=page.getByRole('region',{name:'编辑设备 / 调整机位',exact:true});await expect(editor).toBeVisible();await expect(page.getByRole('dialog')).toHaveCount(0);await expect(crumbs).toContainText('编辑设备');
 await crumbs.getByRole('link',{name:`返回微模块 ${module.name}`,exact:true}).click();await expect(editor).toHaveCount(0);await expect(page.getByRole('group',{name:`${module.name}俯视图`,exact:true})).toBeVisible();
});

test('长柜列紧凑总览、目标柜导航、双柜操作与详情编辑页',async({page,request})=>{
 const {module,racks}=await setup(request,page,'长柜列验收',18);
 await page.getByRole('button',{name:'立面图',exact:true}).click();await page.getByRole('button',{name:'紧凑总览',exact:true}).click();
 await expect(page.locator('.elevation-card')).toHaveCount(18);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.getByRole('button',{name:'定位机柜 L18',exact:true}).click();await expect(page.locator(`[data-rack-id="${racks[17].id}"]`)).toHaveClass(/focused/);
 await page.getByRole('button',{name:'两柜并排',exact:true}).click();await page.getByRole('combobox',{name:'并排目标机柜',exact:true}).selectOption(racks[0].id);await expect(page.locator('.elevation-card')).toHaveCount(2);
 await page.getByRole('button',{name:'编辑微模块详细信息',exact:true}).click();const editor=page.getByRole('region',{name:'微模块布局配置',exact:true});await expect(editor).toBeVisible();await editor.getByLabel('微模块名称',{exact:true}).fill('长柜列编辑成功');await editor.getByRole('button',{name:'保存模块信息',exact:true}).click();await expect(editor.getByRole('status')).toContainText('已保存');await editor.getByRole('button',{name:'返回查看',exact:true}).click();
 await expect(page.getByRole('navigation',{name:'位置导航'})).toContainText('长柜列编辑成功');expect((await getState(request)).modules.find(m=>m.id===module.id)?.name).toBe('长柜列编辑成功');
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:'.cache/rack-workbench-mobile.png',fullPage:true});
});
