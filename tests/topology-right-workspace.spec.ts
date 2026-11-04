import {test,expect} from './fixtures';
import type {APIRequestContext,Page,Locator} from '@playwright/test';
import type {State} from '../src/model';
import {navigationHash} from '../src/navigationState';
const state=async(api:APIRequestContext):Promise<State>=>(await api.get('/api/state')).json();
async function setup(page:Page,api:APIRequestContext,name:string,positioned=true,groupGap=false){
 let s=await state(api);const response=await api.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:name,modules:[{name:'网络模块',type:'single',rack_count:3,u_height:42}],infra:{power:0,ac:0,env:0}}}});expect(response.ok()).toBeTruthy();s=await response.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await api.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,20,网络设备,RIGHT-'+project.id}});expect(imported.ok()).toBeTruthy();s=await imported.json();const devices=s.devices.filter(d=>d.moduleId===module.id),leftDevices=devices.slice(0,4),rightDevices=devices.slice(4),layers=[{id:'access',name:'接入层'},{id:'compute',name:'计算层'}];
 for(const [i,name,members] of [[0,'左侧网络',leftDevices],[1,'右侧网络',rightDevices]] as const){const deviceLayers=Object.fromEntries(members.map((d,j)=>[d.id,i&&j>=14?'compute':'access'])),devicePositions=Object.fromEntries(members.map((d,j)=>[d.id,{x:(groupGap&&(!i||j<14)?624:0)+(j===2?416:j%2*208),y:Math.floor(j/2)*112}]));const created=await api.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,siteId:room.id,name,purpose:'management',color:i?'#9478b5':'#497cda',deviceIds:members.map(d=>d.id),layersVisible:true,layers,deviceLayers,devicePositions,position:positioned?{x:i?1300:20,y:22}:undefined}}});expect(created.ok()).toBeTruthy();s=await created.json()}
 const networks=s.networks.filter(n=>n.projectId===project.id),right=networks.find(n=>n.name==='右侧网络')!,left=networks.find(n=>n.name==='左侧网络')!;
 await page.goto('/'+navigationHash({view:'topology',project:project.id,room:'',module:'',rack:'',network:'',filterRoom:room.id}));await expect(page.locator('.topology-canvas')).toBeVisible();return {right,left,rightDevices,leftDevices,devices,project,room};
}
async function viewport(page:Page){return page.locator('.topology-canvas').evaluate(el=>{const matrix=el.querySelector('svg')!.getScreenCTM()!;return {left:el.scrollLeft,top:el.scrollTop,e:matrix.e,f:matrix.f,a:matrix.a,d:matrix.d}})}
async function coords(node:Locator){return node.evaluate(el=>{const p=(el as SVGGElement).transform.baseVal.consolidate()!.matrix;return {x:Math.round(p.e*1000)/1000,y:Math.round(p.f*1000)/1000}})}
async function moveTo(page:Page,node:Locator,target:{x:number;y:number}){const points=await node.evaluate((el,target)=>{const matrix=(el as SVGGElement).ownerSVGElement!.getScreenCTM()!,p=(el as SVGGElement).transform.baseVal.consolidate()!.matrix,a=new DOMPoint(p.e+40,p.f+24).matrixTransform(matrix),b=new DOMPoint(target.x+40,target.y+24).matrixTransform(matrix);return {a:{x:a.x,y:a.y},b:{x:b.x,y:b.y}}},target);await page.mouse.move(points.a.x,points.a.y);await page.mouse.down();await page.mouse.move(points.b.x,points.b.y,{steps:12})}

test('右侧网络打开新增、设备和修改面板保持画布屏幕位置、滚动与缩放',async({page,request})=>{
 await setup(page,request,'右侧面板视野');const canvas=page.locator('.topology-canvas');await canvas.evaluate(el=>{el.scrollLeft=el.scrollWidth;el.scrollTop=0});
 for(const name of ['修改网络 右侧网络','在右侧网络内新建网络','为右侧网络添加设备']){
  const before=await viewport(page);expect(before.left).toBeGreaterThan(200);await page.getByRole('button',{name,exact:true}).click();await expect(page.getByRole('complementary',{name:'网络列表面板',exact:true})).toBeVisible();expect(await viewport(page)).toEqual(before);if(name==='修改网络 右侧网络')await page.locator('.topology-workbench').screenshot({path:'.cache/topology-right-quick-overlay.png'});await page.getByRole('button',{name:'关闭快捷操作',exact:true}).click();expect(await viewport(page)).toEqual(before);
 }
});

for(const side of ['左侧','右侧'] as const)test(side+'网络展开组后设备可拖到本层左侧空白，松手刷新不回弹',async({page,request})=>{
 const data=await setup(page,request,side+'展开后落点');if(side==='右侧')await page.locator('.topology-canvas').evaluate(el=>{el.scrollLeft=el.scrollWidth;el.scrollTop=0});
 const network=side==='右侧'?data.right:data.left,members=side==='右侧'?data.rightDevices:data.leftDevices,group=page.getByRole('button',{name:side==='右侧'?'展开 KunTai R722 K2 14台':'展开 KunTai R722 K2 4台',exact:true});await group.click();if(side==='右侧')await page.locator('.topology-canvas').evaluate(el=>{el.scrollLeft=el.scrollWidth});
 const anchor=page.locator(`[data-network-node="${members[0].id}"]`),node=page.locator(`[data-network-node="${members[2].id}"]`),p=await coords(anchor),target={x:p.x,y:p.y+112},before=await state(request),view=await viewport(page);await moveTo(page,node,target);await expect(page.locator('.topology-drag-hint')).toContainText('保存位置');await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);expect(await coords(node)).toEqual(target);expect(await viewport(page)).toEqual(view);await page.reload();await page.getByRole('button',{name:side==='右侧'?'展开 KunTai R722 K2 14台':'展开 KunTai R722 K2 4台',exact:true}).click();expect(await coords(node)).toEqual(target);const after=await state(request);expect(after.devices).toEqual(before.devices);expect(after.links).toEqual(before.links);expect(after.networks.find(n=>n.id===network.id)?.deviceLayers).toEqual(network.deviceLayers);
});

test('右侧自动布局网络调整设备后保持网络原点，落点不随网络宽度收缩而跳动',async({page,request})=>{
 const data=await setup(page,request,'右侧自动布局落点',false);await page.getByRole('button',{name:'展开 KunTai R722 K2 4台',exact:true}).click();await page.getByRole('button',{name:'展开 KunTai R722 K2 14台',exact:true}).click();
 const node=page.locator(`[data-network-node="${data.rightDevices[3].id}"]`),anchor=page.locator(`[data-network-node="${data.rightDevices[0].id}"]`);await node.scrollIntoViewIfNeeded();const p=await coords(anchor),target={x:p.x,y:p.y+112},before=await state(request);await moveTo(page,node,target);await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);expect(await coords(node)).toEqual(target);
});

for(const side of ['左侧','右侧'] as const)test(side+'合并设备组可向本层左侧空白移动，画布不跟随宽度收缩而跳动',async({page,request})=>{
 const data=await setup(page,request,side+'整组落点',true,true),canvas=page.locator('.topology-canvas');if(side==='右侧')await canvas.evaluate(el=>{el.scrollLeft=el.scrollWidth});const node=page.getByRole('button',{name:side==='右侧'?'展开 KunTai R722 K2 14台':'展开 KunTai R722 K2 4台',exact:true}),p=await coords(node),target={x:p.x-416,y:p.y},before=await state(request),view=await viewport(page);await moveTo(page,node,target);await expect(page.locator('.topology-drag-hint')).toContainText('保存位置');await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);expect(await coords(node)).toEqual(target);expect(await viewport(page)).toEqual(view);
});

test('缩放后已有右侧详情时切换三种左侧快捷面板，图形视野仍不变',async({page,request})=>{
 await setup(page,request,'详情与快捷面板视野');await page.getByRole('button',{name:'选择网络 右侧网络',exact:true}).click();await expect(page.locator('.topology-inspector')).toBeVisible();await page.getByRole('button',{name:'放大拓扑',exact:true}).click();await page.getByRole('button',{name:'放大拓扑',exact:true}).click();
 const canvas=page.locator('.topology-canvas');await canvas.evaluate(el=>{el.scrollLeft=el.scrollWidth;el.scrollTop=0});
 for(const name of ['修改网络 右侧网络','在右侧网络内新建网络','为右侧网络添加设备']){
  const before=await viewport(page);await page.getByRole('button',{name,exact:true}).click();await expect(page.getByRole('complementary',{name:'网络列表面板',exact:true})).toBeVisible();expect(await viewport(page)).toEqual(before);await page.getByRole('button',{name:'关闭快捷操作',exact:true}).click();expect(await viewport(page)).toEqual(before);
 }
 await page.locator('.topology-workbench').screenshot({path:'.cache/topology-right-stable-panels.png'});
});

test('总览后拖动右侧合并组不触发二次适应，主动总览仍能清理保留空白',async({page,request})=>{
 await setup(page,request,'总览后稳定落点',true,true);await page.getByRole('button',{name:'重置拓扑缩放',exact:true}).click();const node=page.getByRole('button',{name:'展开 KunTai R722 K2 14台',exact:true}),p=await coords(node),target={x:p.x-416,y:p.y},before=await state(request),view=await viewport(page);await moveTo(page,node,target);await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);expect(await coords(node)).toEqual(target);expect(await viewport(page)).toEqual(view);await page.getByRole('button',{name:'重置拓扑缩放',exact:true}).click();await expect.poll(()=>page.locator('.topology-canvas').evaluate(el=>el.scrollWidth<=el.clientWidth+1&&el.scrollHeight<=el.clientHeight+1)).toBeTruthy();
});


test('保持视野的快捷保存后详情同步更新，不能回写旧网络名称',async({page,request})=>{
 const {right}=await setup(page,request,'快捷保存详情同步');await page.getByRole('button',{name:'选择网络 右侧网络',exact:true}).click();const canvas=page.locator('.topology-canvas');await canvas.evaluate(el=>{el.scrollLeft=el.scrollWidth});const view=await viewport(page);await page.getByRole('button',{name:'修改网络 右侧网络',exact:true}).click();const quick=page.getByRole('region',{name:'快速网络配置',exact:true});await quick.getByLabel('网络名称',{exact:true}).fill('右侧改名');await quick.getByRole('button',{name:'保存网络',exact:true}).click();await expect(quick).toHaveCount(0);const details=page.getByRole('region',{name:'网络详情',exact:true});await expect(details.getByLabel('网络名称',{exact:true})).toHaveValue('右侧改名');await expect(details.getByRole('button',{name:'应用名称与放置位置',exact:true})).toBeDisabled();expect(await viewport(page)).toEqual(view);expect((await state(request)).networks.find(n=>n.id===right.id)?.name).toBe('右侧改名');
});

test('底部设备向上移导致图形高度缩短时保留纵向视野',async({page,request})=>{
 const data=await setup(page,request,'底部设备稳定视野'),mover=data.rightDevices[13];let s=await state(request);const changed=await request.post('/api/networks/'+data.right.id+'/position',{data:{revision:s.revision,deviceId:mover.id,position:{x:208,y:1500}}});expect(changed.ok()).toBeTruthy();s=await changed.json();await page.reload();await page.getByRole('button',{name:'展开 KunTai R722 K2 14台',exact:true}).click();const node=page.locator(`[data-network-node="${mover.id}"]`),p=await coords(node),canvas=page.locator('.topology-canvas');await canvas.evaluate((el,y)=>{el.scrollLeft=el.scrollWidth;el.scrollTop=y-300},p.y);const view=await viewport(page),target={x:p.x,y:p.y-224};expect(view.top).toBeGreaterThan(1000);await moveTo(page,node,target);await expect(page.locator('.topology-drag-hint')).toContainText('保存位置');await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(s.revision+1);expect(await coords(node)).toEqual(target);expect(await viewport(page)).toEqual(view);
});
