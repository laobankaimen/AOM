import {test,expect} from './fixtures';
import type {APIRequestContext,Page} from '@playwright/test';
import type {State} from '../src/model';
import {navigationHash} from '../src/navigationState';
const state=async(api:APIRequestContext):Promise<State>=>(await api.get('/api/state')).json();
async function setup(page:Page,api:APIRequestContext,name:string){
 let s=await state(api);const planned=await api.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:name,modules:[{name:'拖动模块',type:'single',rack_count:2,u_height:42}],infra:{power:0,ac:0,env:0}}}});expect(planned.ok()).toBeTruthy();s=await planned.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await api.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,4,拖动设备,EMPTY-'+project.id}});expect(imported.ok()).toBeTruthy();s=await imported.json();const devices=s.devices.filter(d=>d.moduleId===module.id),[lower,a,b,mover]=devices;
 const created=await api.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,siteId:room.id,name:'管理网络2',purpose:'management',color:'#9478b5',position:{x:180,y:22},deviceIds:devices.map(d=>d.id),layers:[{id:'access',name:'接入层'},{id:'compute',name:'计算层'}],layersVisible:true,deviceLayers:{[lower.id]:'compute',[a.id]:'access',[b.id]:'access',[mover.id]:'access'},devicePositions:{[lower.id]:{x:0,y:112},[a.id]:{x:0,y:0},[b.id]:{x:208,y:0},[mover.id]:{x:416,y:112}}}}});expect(created.ok()).toBeTruthy();s=await created.json();const network=s.networks.find(n=>n.projectId===project.id)!;
 await page.goto('/'+navigationHash({view:'topology',project:project.id,room:'',module:'',rack:'',network:'',filterRoom:room.id,presentation:'detail'}));await expect(page.locator('.topology-canvas')).toBeVisible();return {devices,network,mover,lower,a};
}
const transform=(page:Page,id:string)=>page.locator(`[data-network-node="${id}"]`).evaluate(el=>{const matrix=(el as SVGGElement).transform.baseVal.consolidate()!.matrix;return {x:Math.round(matrix.e*1000)/1000,y:Math.round(matrix.f*1000)/1000}});
async function dragTo(page:Page,id:string,target:{x:number;y:number}){
 const points=await page.locator(`[data-network-node="${id}"]`).evaluate((el,target)=>{const matrix=(el as SVGGElement).ownerSVGElement!.getScreenCTM()!,own=(el as SVGGElement).transform.baseVal.consolidate()!.matrix,start=new DOMPoint(own.e+40,own.f+24).matrixTransform(matrix),end=new DOMPoint(target.x+40,target.y+24).matrixTransform(matrix);return {start:{x:start.x,y:start.y},end:{x:end.x,y:end.y}}},target);
 await page.mouse.move(points.start.x,points.start.y);await page.mouse.down();await page.mouse.move(points.end.x,points.end.y,{steps:12});
}
function expectSavedPosition(point:{x:number;y:number}|undefined){expect(point?.x).toBeCloseTo(0,3);expect(point?.y).toBeCloseTo(112,3)}


test('同一层内拖到框内左侧空白，松手和刷新后保留落点，不受下一层旧坐标阻挡',async({page,request})=>{
 const {devices,network,mover,lower,a}=await setup(page,request,'左侧空白落点');const anchor=await transform(page,a.id),lowerBefore=await transform(page,lower.id);
 const target={x:anchor.x,y:anchor.y+112},before=await state(request),audits=await(await request.get('/api/audits')).json();
 await dragTo(page,mover.id,target);await expect(page.locator('.topology-drag-hint')).toContainText('保存位置');expect(await transform(page,mover.id)).toEqual(target);await page.mouse.up();
 await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);expectSavedPosition((await state(request)).networks.find(n=>n.id===network.id)?.devicePositions?.[mover.id]);await expect.poll(()=>transform(page,mover.id)).toEqual(target);expect(await transform(page,lower.id)).toEqual(lowerBefore);
 await page.reload();await expect.poll(()=>transform(page,mover.id)).toEqual(target);expect(await transform(page,lower.id)).toEqual(lowerBefore);const saved=await state(request),savedNetwork=saved.networks.find(n=>n.id===network.id)!;expect(saved.devices.filter(d=>devices.some(old=>old.id===d.id))).toEqual(devices);expect(savedNetwork.deviceIds).toEqual(network.deviceIds);expect(savedNetwork.deviceLayers).toEqual(network.deviceLayers);expect(saved.links).toEqual(before.links);expect(await(await request.get('/api/audits')).json()).toEqual(audits);await page.locator('.topology-workbench').screenshot({path:'.cache/topology-left-empty-drop.png'});
});

test('框内空白落点在缩放和滚动后仍准确，Esc取消不保存',async({page,request})=>{
 const {network,mover,a}=await setup(page,request,'缩放左侧落点');await page.getByRole('button',{name:'放大拓扑',exact:true}).click();await page.getByRole('button',{name:'放大拓扑',exact:true}).click();const canvas=page.locator('.topology-canvas');await canvas.evaluate(el=>{el.scrollLeft=180;el.scrollTop=20});const anchor=await transform(page,a.id),target={x:anchor.x,y:anchor.y+112},node=page.getByRole('button',{name:'查看拓扑设备 '+mover.name,exact:true}),before=await state(request),original=await transform(page,mover.id);

 await dragTo(page,mover.id,target);await page.keyboard.press('Escape');await page.mouse.up();expect(await transform(page,mover.id)).toEqual(original);expect((await state(request)).revision).toBe(before.revision);await dragTo(page,mover.id,target);await page.mouse.up();await expect.poll(async()=>(await state(request)).revision).toBe(before.revision+1);await expect.poll(()=>transform(page,mover.id)).toEqual(target);expectSavedPosition((await state(request)).networks.find(n=>n.id===network.id)?.devicePositions?.[mover.id]);
});
