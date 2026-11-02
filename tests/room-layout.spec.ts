import {test,expect} from './fixtures';
import {type Page,type APIRequestContext} from '@playwright/test';import type {State} from '../src/model';import {enterRoom,enterModule} from './navigation';
async function snapshot(request:APIRequestContext):Promise<State>{return (await request.get('/api/state')).json()}
async function setup(page:Page,request:APIRequestContext,name:string){let s=await snapshot(request);const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'布局机房',modules:[{name:'单排模块',type:'single',rack_count:3,u_height:42},{name:'双排模块',type:'double',rack_left:3,rack_right:3,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(response.ok()).toBeTruthy();s=await response.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,modules=s.modules.filter(m=>m.siteId===room.id);await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);return {project,room,modules,state:s};}
async function svgPoint(page:Page,selector:string,x:number,y:number){return page.locator(selector).evaluate((element,p)=>{const m=(element as SVGSVGElement).getScreenCTM()!;const result=new DOMPoint(p.x,p.y).matrixTransform(m);return {x:result.x,y:result.y}}, {x,y})}
test('机房立体位置图拖动、键盘微调、刷新保留及点击查看',async({page,request})=>{
 const {project,room,modules,state}=await setup(page,request,'位置图验收');const module=modules[0];const node=page.getByRole('button',{name:`选择布局模块 ${module.name}`,exact:true});await expect(page.getByRole('group',{name:'机房微模块位置图',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'立体示意',exact:true})).toBeVisible();
 await node.scrollIntoViewIfNeeded();const bounds=(await node.boundingBox())!;await page.mouse.move(bounds.x+bounds.width/2,bounds.y+bounds.height/2);await page.mouse.down();await page.mouse.move(bounds.x+bounds.width/2+24,bounds.y+bounds.height/2+18,{steps:12});await page.mouse.up();
 await expect.poll(async()=>(await snapshot(request)).modules.find(m=>m.id===module.id)?.roomPosition).toBeTruthy();let s=await snapshot(request);const saved=s.modules.find(m=>m.id===module.id)!.roomPosition!;expect(s.racks).toEqual(state.racks);await expect(page.getByRole('region',{name:'微模块布局配置',exact:true})).toHaveCount(0);await expect(page.locator('.floor-panel')).toHaveCount(0);
 await page.screenshot({path:'.cache/room-isometric.png',fullPage:true});await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);await node.focus();await node.press('Alt+ArrowRight');await expect.poll(async()=>(await snapshot(request)).modules.find(m=>m.id===module.id)?.roomPosition?.x).toBe(saved.x+10);
 await page.getByRole('button',{name:'平面布局',exact:true}).click();await node.click();const before=(await snapshot(request)).revision;const pos=(await node.boundingBox())!;await page.mouse.move(pos.x+pos.width/2,pos.y+pos.height/2);await page.mouse.down();await page.mouse.move(pos.x+pos.width/2+20,pos.y+pos.height/2+15,{steps:8});await page.keyboard.press('Escape');await page.mouse.up();expect((await snapshot(request)).revision).toBe(before);
 await page.getByRole('button',{name:'进入模块',exact:true}).click();await expect(page.getByRole('group',{name:module.name+'俯视图',exact:true})).toBeVisible();await page.getByRole('navigation',{name:'位置导航'}).getByRole('link',{name:`返回机房 ${room.name}`,exact:true}).click();await node.click();await page.locator('.action-menu summary').filter({hasText:'模块位置操作'}).click();await page.getByRole('button',{name:'恢复默认位置',exact:true}).click();await expect.poll(async()=>(await snapshot(request)).modules.find(m=>m.id===module.id)?.roomPosition).toBeUndefined();
 await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
});

test('列间设施跨列拖动、取消与详情编辑分离',async({page,request})=>{
 const {project,room,modules,state}=await setup(page,request,'设施拖动验收');const module=modules[1],ac=state.infra.find(i=>i.moduleId===module.id&&i.kind==='ac')!;await enterModule(page,module.name);
 const tile=page.getByRole('button',{name:`查看设施 ${ac.name}`,exact:true});await tile.scrollIntoViewIfNeeded();const b=(await tile.boundingBox())!;const destination=await svgPoint(page,'.floor-svg',850,350);
 await page.mouse.move(b.x+b.width/2,b.y+b.height/2);await page.mouse.down();await page.mouse.move(destination.x,destination.y,{steps:12});await expect(page.locator('.infra-drop-indicator')).toBeVisible();await page.mouse.up();await expect.poll(async()=>(await snapshot(request)).infra.find(i=>i.id===ac.id)?.side).toBe('R');
 let s=await snapshot(request);expect(s.infra.find(i=>i.id===ac.id)?.afterRack).toBe(3);expect(s.infra.find(i=>i.id===ac.id)?.name).toBe(ac.name);expect(s.racks).toEqual(state.racks);await expect(page.locator('.editor-page')).toHaveCount(0);
 await tile.focus();await tile.press('Alt+ArrowLeft');await expect.poll(async()=>(await snapshot(request)).infra.find(i=>i.id===ac.id)?.afterRack).toBe(2);await page.screenshot({path:'.cache/infra-drag.png',fullPage:true});
 const revision=(await snapshot(request)).revision;const start=(await tile.boundingBox())!,cancelTarget=await svgPoint(page,'.floor-svg',100,130);await page.mouse.move(start.x+start.width/2,start.y+start.height/2);await page.mouse.down();await page.mouse.move(cancelTarget.x,cancelTarget.y,{steps:8});await page.keyboard.press('Escape');await page.mouse.up();expect((await snapshot(request)).revision).toBe(revision);
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);await enterModule(page,module.name);await expect(page.locator(`[data-cabinet-row="R"] [data-infrastructure-id="${ac.id}"]`)).toBeVisible();await tile.click();await expect(page.getByRole('heading',{name:'设施详情',exact:true})).toBeVisible();await expect(page.locator('.editor-page')).toHaveCount(0);await page.locator('.detail-panel').getByRole('button',{name:'编辑详细信息',exact:true}).click();await expect(page.getByRole('region',{name:'微模块布局配置',exact:true})).toBeVisible();
});

test('微模块平面与立体旋转方向保存，拖动连续且只在释放后写入',async({page,request})=>{
 const browserErrors:string[]=[];
 page.on('pageerror',e=>browserErrors.push(e.message));
 page.on('console',message=>{if(message.type()==='error')browserErrors.push(message.text())});
 const {project,room,modules}=await setup(page,request,'方向与流畅拖动验收');
 const module=modules[0],node=page.getByRole('button',{name:`选择布局模块 ${module.name}`,exact:true});
 const svg=page.locator('.room-position-svg');
 await page.getByRole('button',{name:'平面布局',exact:true}).click();await node.click();
 await page.getByRole('button',{name:'旋转微模块90度',exact:true}).click();
 await expect.poll(async()=>(await snapshot(request)).modules.find(m=>m.id===module.id)?.roomRotation).toBe(90);
 await expect(node).toHaveAttribute('data-room-rotation','90');
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);
 await expect(node).toHaveAttribute('data-room-rotation','90');await node.focus();await node.press('r');
 await expect.poll(async()=>(await snapshot(request)).modules.find(m=>m.id===module.id)?.roomRotation).toBe(180);
 const transform=()=>node.evaluate(el=>{
  const matrix=(el as SVGGraphicsElement).getScreenCTM()!;
  const values=(el.getAttribute('transform')||'').match(/translate\(([^,]+),([^\)]+)\)/);
  return {x:matrix.e,y:matrix.f,finite:!!values&&Number.isFinite(Number(values[1]))&&Number.isFinite(Number(values[2]))};
 });
 for(const view of ['平面布局','立体示意']){
  await page.getByRole('button',{name:view,exact:true}).click();
  for(const zoom of [75,100,150]){
   await page.getByRole('button',{name:'适应机房位置图',exact:true}).click();
   if(zoom===75)await page.getByRole('button',{name:'缩小机房位置图',exact:true}).click();
   if(zoom===150)for(let i=0;i<2;i++)await page.getByRole('button',{name:'放大机房位置图',exact:true}).click();
   const handle=node.locator('.room-module-caption rect');await handle.scrollIntoViewIfNeeded();
   const b=(await handle.boundingBox())!,x=b.x+b.width/2,y=b.y+b.height/2;
   const revision=(await snapshot(request)).revision;
   const before=await transform();expect(before.finite).toBe(true);
   const geometry=await node.locator('polygon').first().getAttribute('points'),bounds=await svg.getAttribute('viewBox');
   await page.mouse.move(x,y);await page.mouse.down();
   // Check exact pointer following, not only that two invalid strings differ.
   for(const delta of [{x:7,y:6},{x:8,y:6},{x:9,y:7},{x:11,y:8}]){
    await page.mouse.move(x+delta.x,y+delta.y);
    await expect.poll(async()=>{const p=await transform();return p.finite&&Math.abs(p.x-before.x-delta.x)<.25&&Math.abs(p.y-before.y-delta.y)<.25}).toBe(true);
   }
   await expect(node).toHaveClass(/is-dragging/);
   expect(await node.locator('polygon').first().getAttribute('points')).toBe(geometry);
   expect(await svg.getAttribute('viewBox')).toBe(bounds);
   expect((await snapshot(request)).revision).toBe(revision);
   await page.keyboard.press('Escape');await page.mouse.up();
   expect((await snapshot(request)).revision).toBe(revision);
   const cancelled=await transform();expect(cancelled.finite).toBe(true);expect(cancelled.x).toBeCloseTo(before.x,2);expect(cancelled.y).toBeCloseTo(before.y,2);
   // Normal drop must persist a finite position and preserve the 180-degree orientation.
   await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x+9,y+6,{steps:6});await page.mouse.up();
   await expect.poll(async()=>(await snapshot(request)).revision).toBe(revision+1);
   const saved=(await snapshot(request)).modules.find(m=>m.id===module.id)!;
   expect(Number.isFinite(saved.roomPosition?.x)).toBe(true);expect(Number.isFinite(saved.roomPosition?.y)).toBe(true);expect(saved.roomRotation).toBe(180);
   expect((await transform()).finite).toBe(true);
   await expect(page.getByRole('button',{name:'旋转微模块90度',exact:true})).toBeEnabled();
  }
 }
 expect(browserErrors).toEqual([]);
});

test('机柜与设施使用相同插入反馈，跨列松手保存，取消及无效落点不写入',async({page,request})=>{
 const {project,room,modules,state}=await setup(page,request,'机柜插入反馈验收');
 const module=modules[1],racks=state.racks.filter(r=>r.moduleId===module.id);
 const rack=racks.find(r=>r.side==='L'&&r.index===1)!;
 await enterModule(page,module.name);
 const tile=page.locator(`[data-cabinet-id="${rack.id}"]`),indicator=page.locator('.cabinet-drop-indicator');
 let writes=0;page.on('request',req=>{if(req.method()==='POST'&&req.url().endsWith('/rack-insert'))writes++});
 const start=async()=>{await tile.scrollIntoViewIfNeeded();const b=(await tile.boundingBox())!;await page.mouse.move(b.x+b.width/2,b.y+b.height/2);await page.mouse.down()};
 const move=async(x:number,y:number)=>{const p=await svgPoint(page,'.floor-svg',x,y);await page.mouse.move(p.x,p.y,{steps:8})};
 await start();await move(850,350);
 await expect(tile).toHaveClass(/cabinet-dragging/);
 await expect(indicator).toBeVisible();await expect(indicator).toContainText('R列 · 列尾');
 await expect(indicator).toHaveAttribute('data-side','R');
 expect(writes).toBe(0);expect((await snapshot(request)).revision).toBe(state.revision);
 await page.mouse.up();
 await expect.poll(async()=>{const r=(await snapshot(request)).racks.find(r=>r.id===rack.id)!;return [r.side,r.index]}).toEqual(['R',4]);
 expect(writes).toBe(1);await expect(indicator).toHaveCount(0);await expect(page.locator('.editor-page')).toHaveCount(0);
 let s=await snapshot(request);expect(s.devices).toEqual(state.devices);expect(s.racks.find(r=>r.id===rack.id)?.name).toBe(rack.name);
 // The same row, leading position, and cancellation share the same preview.
 await start();await move(85,350);await expect(indicator).toContainText('R列 · R01前');
 await page.keyboard.press('Escape');await page.mouse.up();
 expect((await snapshot(request)).revision).toBe(s.revision);expect(writes).toBe(1);await expect(indicator).toHaveCount(0);
 await start();await move(85,350);await page.mouse.up();
 await expect.poll(async()=>(await snapshot(request)).racks.find(r=>r.id===rack.id)?.index).toBe(1);expect(writes).toBe(2);
 // Dropping in the aisle cancels instead of opening an editor or saving.
 s=await snapshot(request);await start();await move(400,260);await expect(indicator).toHaveCount(0);await page.mouse.up();
 expect((await snapshot(request)).revision).toBe(s.revision);expect(writes).toBe(2);
 await page.reload();await expect(page.getByRole('combobox',{name:'选择项目',exact:true})).toHaveValue(project.id);await expect(page.getByRole('group',{name:`${module.name}俯视图`,exact:true})).toBeVisible();await expect(page.getByRole('navigation',{name:'位置导航'})).toContainText(room.name);
 await expect(page.locator('[data-cabinet-row="R"] [data-cabinet-id]').first()).toHaveAttribute('data-cabinet-id',rack.id);
 // Keyboard row changes use the same persisted ordering.
 await tile.focus();await tile.press('Alt+ArrowUp');
 await expect.poll(async()=>(await snapshot(request)).racks.find(r=>r.id===rack.id)?.side).toBe('L');
});
