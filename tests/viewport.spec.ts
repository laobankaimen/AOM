import {test,expect} from './fixtures';
import {enterRoom,enterModule} from './navigation';
import type {State} from '../src/model';
import type {Page} from '@playwright/test';
async function contained(page:Page,scroll:string,svg:string){
 await expect.poll(()=>page.evaluate(({scroll,svg})=>{const outer=document.querySelector(scroll)!,drawing=document.querySelector(svg)!;const a=outer.getBoundingClientRect(),b=drawing.getBoundingClientRect();return b.width<=a.width+1&&b.height<=a.height+1&&b.left>=a.left-1&&b.top>=a.top-1&&b.right<=a.right+1&&b.bottom<=a.bottom+1},{scroll,svg})).toBe(true);
}
test('宽视口与小屏适应完整画布，放大只调整图形且可恢复',async({page,request})=>{
 let s:State=await (await request.get('/api/state')).json();const result=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'画布适配验收',site_name:'宽屏机房',modules:[{name:'单排模块',type:'single',rack_count:5,u_height:42},{name:'双排模块',type:'double',rack_left:4,rack_right:4,u_height:42},{name:'综合模块',type:'single',rack_count:4,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(result.ok()).toBe(true);s=await result.json();const project=s.projects.find(p=>p.name==='画布适配验收')!;
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,'宽屏机房');
 for(const viewport of [{width:1440,height:1000},{width:3200,height:1800},{width:390,height:844}]){
  await page.setViewportSize(viewport);
  for(const view of ['平面布局','立体示意']){
   await page.getByRole('button',{name:view,exact:true}).click();await page.getByRole('button',{name:'适应机房位置图',exact:true}).click();
   await contained(page,'.room-position-scroll','.room-position-svg');
   for(const n of await page.locator('.room-position-module').all()){const b=(await n.boundingBox())!,a=(await page.locator('.room-position-scroll').boundingBox())!;expect(b.x).toBeGreaterThanOrEqual(a.x-1);expect(b.y+b.height).toBeLessThanOrEqual(a.y+a.height+1)}
   const before=await page.locator('.room-position-svg').evaluate(el=>(el as SVGSVGElement).getScreenCTM()!.a);
   await page.getByRole('button',{name:'放大机房位置图',exact:true}).click();await expect.poll(()=>page.locator('.room-position-svg').evaluate(el=>(el as SVGSVGElement).getScreenCTM()!.a)).toBeGreaterThan(before*1.2);
   await page.getByRole('button',{name:'适应机房位置图',exact:true}).click();await contained(page,'.room-position-scroll','.room-position-svg');
  }
  const main=(await page.locator('.app-main>main').boundingBox())!,outer=(await page.locator('.app-main').boundingBox())!;expect(main.width).toBeGreaterThan(outer.width*.97);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 }
 await page.setViewportSize({width:3200,height:1800});await enterModule(page,'双排模块');await page.getByRole('button',{name:'适应预览宽度',exact:true}).click();await contained(page,'.floor-scroll','.floor-svg');
 const initial=(await page.locator('.floor-svg').boundingBox())!;await page.getByRole('button',{name:'放大俯视图',exact:true}).click();await expect.poll(async()=>(await page.locator('.floor-svg').boundingBox())!.width).toBeGreaterThan(initial.width*1.2);
 await page.getByRole('button',{name:'适应预览宽度',exact:true}).click();await contained(page,'.floor-scroll','.floor-svg');
 expect((await (await request.get('/api/state')).json()).revision).toBe(s.revision);
 await page.screenshot({path:'.cache/wide-canvas.png',fullPage:true});
});

test('拓扑图与分组图适应宽高，放大不重新排列设备',async({page,request})=>{
 let s:State=await (await request.get('/api/state')).json();const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'拓扑缩放验收',site_name:'拓扑机房',modules:[{name:'拓扑模块',type:'single',rack_count:2,u_height:42}],infra:{power:0,ac:0,env:0}}}});expect(response.ok()).toBe(true);s=await response.json();const project=s.projects.find(p=>p.name==='拓扑缩放验收')!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,12,缩放服务器,SCALE-SERVER\nRZ2805-48YS8CQ,2,缩放交换机,SCALE-SW'}});expect(imported.ok()).toBe(true);s=await imported.json();
 const network=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,name:'缩放网络',purpose:'business',color:'#497cda',deviceIds:s.devices.filter(d=>d.moduleId===module.id).map(d=>d.id)}}});expect(network.ok()).toBe(true);s=await network.json();
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await page.getByRole('button',{name:'网络拓扑',exact:true}).click();
 for(const viewport of [{width:1440,height:1000},{width:3200,height:1800},{width:390,height:844}]){
  await page.setViewportSize(viewport);
  for(const mode of ['detail','auto']){
   await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption(mode);await page.getByRole('button',{name:'重置拓扑缩放',exact:true}).click();
   const container=mode==='detail'?'.topology-canvas':'.summary-canvas',drawing=container+' svg';await contained(page,container,drawing);
   const before=await page.locator(drawing).evaluate(el=>(el as SVGSVGElement).getScreenCTM()!.a),positions=await page.locator(drawing+' [data-network-node]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('transform')));
   await page.getByRole('button',{name:'放大拓扑',exact:true}).click();await expect.poll(()=>page.locator(drawing).evaluate(el=>(el as SVGSVGElement).getScreenCTM()!.a)).toBeGreaterThan(before);
   expect(await page.locator(drawing+' [data-network-node]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('transform')))).toEqual(positions);
   await page.getByRole('button',{name:'重置拓扑缩放',exact:true}).click();await contained(page,container,drawing);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
 }
 expect((await (await request.get('/api/state')).json()).revision).toBe(s.revision);
});
