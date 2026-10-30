import {openNetworkPanel,closeNetworkPanel,openCanvasMenu} from './navigation';
import {test,expect} from './fixtures';
import type {State} from '../src/model';

test('详情关闭可恢复，网络整组拖动在两种视图中保存且不改变成员与审计',async({page,request})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 const snapshot=async():Promise<State>=>(await request.get('/api/state')).json();let s=await snapshot();
 const planned=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'网络整组移动验收',site_name:'拖动机房',modules:[{name:'拖动模块',type:'single',rack_count:2,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(planned.ok()).toBe(true);s=await planned.json();const project=s.projects.find(p=>p.name==='网络整组移动验收')!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,4,分区服务器,GROUP-'+project.id}});expect(imported.ok()).toBe(true);s=await imported.json();const devices=s.devices.filter(d=>d.moduleId===module.id);
 for(const [i,name] of ['网络A','网络B'].entries()){
  const response=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,name,purpose:'business',color:i?'#159784':'#497cda',deviceIds:devices.slice(i*2,i*2+2).map(d=>d.id)}}});expect(response.ok()).toBe(true);s=await response.json();
 }
 const linked=await request.post('/api/links',{data:{revision:s.revision,link:{projectId:project.id,sourceId:devices[0].id,targetId:devices[2].id,kind:'logical',status:'pending',networkIds:[]}}});expect(linked.ok()).toBe(true);s=await linked.json();const a=s.networks.find(n=>n.projectId===project.id&&n.name==='网络A')!,original=s,audits=await(await request.get('/api/audits')).json();
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await page.getByRole('navigation',{name:'主导航'}).getByRole('button',{name:'网络拓扑',exact:true}).click();
 await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption('detail');await expect(page.getByRole('button',{name:'显示详情',exact:true})).toBeVisible();await page.getByRole('button',{name:'显示详情',exact:true}).click();await expect(page.locator('.topology-inspector')).toContainText('选择设备或链路');
 const first=page.getByRole('button',{name:'查看拓扑设备 '+devices[0].name,exact:true});await first.click();await page.getByRole('button',{name:'收起拓扑详情',exact:true}).click();await expect(page.locator('.topology-inspector')).not.toBeVisible();await expect(first).toHaveAttribute('aria-pressed','true');
 await page.getByRole('button',{name:'显示详情',exact:true}).click();await expect(page.locator('.topology-inspector')).toContainText(devices[0].name);await page.getByRole('button',{name:'收起拓扑详情',exact:true}).click();await first.click();await expect(page.locator('.topology-inspector')).toContainText(devices[0].name);
 await page.getByRole('button',{name:'取消拓扑选择',exact:true}).click();await page.getByRole('button',{name:'收起拓扑详情',exact:true}).click();
 for(const mode of ['detail','auto']){
  await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption(mode);
  const handle=page.getByRole('button',{name:'移动网络分区 网络A',exact:true}),lane=page.locator(`[data-topology-lane="${a.id}"]`),svg=page.locator(mode==='detail'?'.topology-canvas svg':'.summary-canvas svg');
  await handle.scrollIntoViewIfNeeded();const rect=(await handle.boundingBox())!,x=rect.x+rect.width*.35,y=rect.y+rect.height*.45;
  const revision=(await snapshot()).revision,viewBox=await svg.getAttribute('viewBox'),initialX=Number(await lane.locator(':scope>rect').first().getAttribute('x'));
  const paths=await svg.locator('path[d]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('d')));
  await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x+25,y+18,{steps:10});await expect(lane).toHaveClass(/network-dragging/);
  await expect.poll(async()=>Number(await lane.locator(':scope>rect').first().getAttribute('x'))).toBeGreaterThan(initialX);expect(await svg.getAttribute('viewBox')).toBe(viewBox);expect((await snapshot()).revision).toBe(revision);
  const afterPaths=await svg.locator('path[d]').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('d')));expect(afterPaths).not.toEqual(paths);
  const target={x:Number(await lane.locator(':scope>rect').first().getAttribute('x')),y:Number(await lane.locator(':scope>rect').first().getAttribute('y'))};
  await page.mouse.up();await expect.poll(async()=>(await snapshot()).revision).toBe(revision+1);const saved=(await snapshot()).networks.find(n=>n.id===a.id)!;expect(saved.position).toEqual(target);
  // Escape reverts the preview without persisting another position.
  await handle.scrollIntoViewIfNeeded();const b=(await handle.boundingBox())!;await page.mouse.move(b.x+40,b.y+20);await page.mouse.down();await page.mouse.move(b.x+58,b.y+32,{steps:8});await expect(lane).toHaveClass(/network-dragging/);await page.keyboard.press('Escape');await page.mouse.up();expect((await snapshot()).revision).toBe(revision+1);expect((await snapshot()).networks.find(n=>n.id===a.id)?.position).toEqual(target);
 }
 s=await snapshot();expect(s.devices).toEqual(original.devices);expect(s.links).toEqual(original.links);expect(s.networks.find(n=>n.id===a.id)?.deviceIds).toEqual(a.deviceIds);expect(s.networks.find(n=>n.id===a.id)?.parentId).toBe(a.parentId);expect(await(await request.get('/api/audits')).json()).toEqual(audits);
 const saved=s.networks.find(n=>n.id===a.id)!.position!;await page.reload();const handle=page.getByRole('button',{name:'移动网络分区 网络A',exact:true});await expect(handle).toBeVisible();await handle.focus();await handle.press('Alt+ArrowRight');await expect.poll(async()=>(await snapshot()).networks.find(n=>n.id===a.id)?.position?.x).toBe(saved.x+12);
 await openCanvasMenu(page);await page.getByRole('button',{name:'自动排列',exact:true}).click();await expect.poll(async()=>(await snapshot()).networks.find(n=>n.id===a.id)?.position).toBeUndefined();expect(await(await request.get('/api/audits')).json()).toEqual(audits);expect(errors).toEqual([]);
});

test('机柜与设施详情收起后保留选择，可重新显示或点击对象打开',async({page,request})=>{
 const {enterRoom,enterModule}=await import('./navigation');let s:State=await(await request.get('/api/state')).json();
 const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'详情恢复验收',site_name:'详情机房',modules:[{name:'详情模块',type:'single',rack_count:2,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(response.ok()).toBe(true);s=await response.json();const project=s.projects.find(p=>p.name==='详情恢复验收')!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!,rack=s.racks.find(r=>r.moduleId===module.id)!,infra=s.infra.find(i=>i.moduleId===module.id&&i.placement==='inline')!;
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);await enterModule(page,module.name);
 const node=page.getByRole('button',{name:'选择机柜 '+rack.name,exact:true});await node.click();await page.getByRole('button',{name:'收起机柜详情',exact:true}).click();await expect(node).toHaveAttribute('aria-pressed','true');await expect(page.locator('.detail-panel')).not.toBeVisible();
 await page.getByRole('button',{name:'显示详情',exact:true}).click();await expect(page.locator('.detail-panel')).toContainText(rack.name);await page.getByRole('button',{name:'收起机柜详情',exact:true}).click();await node.click();await expect(page.locator('.detail-panel')).toBeVisible();
 const facility=page.getByRole('button',{name:'查看设施 '+infra.name,exact:true});await facility.click();await page.getByRole('button',{name:'收起设施详情',exact:true}).click();await expect(facility).toHaveAttribute('aria-pressed','true');await facility.click();await expect(page.locator('.detail-panel')).toContainText(infra.name);await page.getByRole('button',{name:'取消布局选择',exact:true}).click();await expect(page.locator('.detail-panel')).toContainText('选择机柜或设施');
 expect((await(await request.get('/api/state')).json()).revision).toBe(s.revision);
});
