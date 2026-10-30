import {test,expect} from './fixtures';
import {enterRoom,enterModule} from './navigation';
import {rowLayout} from '../src/layout';
import {previewPlan} from '../src/templatePlan';
import type {Page,APIRequestContext,Locator} from '@playwright/test';
import type {State} from '../src/model';
const snapshot=async(request:APIRequestContext):Promise<State>=>(await request.get('/api/state')).json();
async function setup(page:Page,request:APIRequestContext,name:string){
 let s=await snapshot(request);const planned=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'插入机房',modules:[{name:'插入模块',type:'single',rack_count:5,u_height:42,layout:[{name:'供电',kind:'power',side:'L',after_rack:2,placement:'inline'},{name:'环境监测',kind:'env',side:'L',after_rack:2,placement:'inline',order:1},{name:'空调',kind:'ac',side:'L',after_rack:4,placement:'inline'},{name:'柜内环境',kind:'env',side:'L',after_rack:1,placement:'integrated'}]}],infra:{power:0,ac:0,env:0}}}});expect(planned.ok()).toBe(true);s=await planned.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,2,插入设备,INSERT-'+project.id}});expect(imported.ok()).toBe(true);s=await imported.json();await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);await enterModule(page,module.name);return {s,module};
}
async function dragTo(page:Page,source:Locator,target:Locator,fraction:number){await source.scrollIntoViewIfNeeded();const a=(await source.boundingBox())!,b=(await target.boundingBox())!;await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(b.x+b.width*fraction,b.y+b.height/2,{steps:12})}
test('机柜插入设施前、设施之间及设施后，落点预览与保存顺序一致',async({page,request})=>{
 const {s:initial,module}=await setup(page,request,'机柜混排插入验收');const racks=initial.racks.filter(r=>r.moduleId===module.id).sort((a,b)=>a.index-b.index),moving=racks[4],power=initial.infra.find(i=>i.moduleId===module.id&&i.kind==='power')!,env=initial.infra.find(i=>i.moduleId===module.id&&i.placement==='inline'&&i.kind==='env')!,ac=initial.infra.find(i=>i.moduleId===module.id&&i.kind==='ac')!;
 const original=rowLayout(module,initial.racks,initial.infra,'L').map(t=>t.id),remaining=original.filter(id=>id!==moving.id),source=page.locator(`[data-cabinet-id="${moving.id}"]`);
 const cases=[{target:power,fraction:.2,before:power.id},{target:power,fraction:.8,before:env.id},{target:ac,fraction:.8,before:''}];
 for(const c of cases){const s=await snapshot(request),target=page.locator(`[data-infrastructure-id="${c.target.id}"]`);await dragTo(page,source,target,c.fraction);await expect(page.locator('.cabinet-drop-indicator')).toHaveAttribute('data-before-id',c.before);expect((await snapshot(request)).revision).toBe(s.revision);await page.mouse.up();await expect.poll(async()=>(await snapshot(request)).revision).toBe(s.revision+1);
  const after=await snapshot(request),expected=[...remaining];expected.splice(c.before?expected.indexOf(c.before):expected.length,0,moving.id);expect(rowLayout(after.modules.find(m=>m.id===module.id)!,after.racks,after.infra,'L').map(t=>t.id)).toEqual(expected);expect(after.devices).toEqual(initial.devices);expect(after.links).toEqual(initial.links);
  const visible=await page.locator('.floor-svg [data-cabinet-row="L"]>.rack-hit').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('data-cabinet-id')||n.getAttribute('data-infrastructure-id')));expect(visible).toEqual(expected);
 }
 const after=await snapshot(request);await page.reload();await expect(source).toBeVisible();expect(await page.locator('.floor-svg [data-cabinet-row="L"]>.rack-hit').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('data-cabinet-id')||n.getAttribute('data-infrastructure-id')))).toEqual(original);expect((await snapshot(request)).revision).toBe(after.revision);
});
test('布局模板采用同一混排插入规则并保存复用顺序',async({page,request})=>{
 await setup(page,request,'模板混排插入验收');await page.locator('.action-menu summary').filter({hasText:'模块更多操作'}).click();await page.getByRole('button',{name:'保存当前布局为模板',exact:true}).click();const editor=page.getByRole('region',{name:'编辑布局模板',exact:true});await editor.getByLabel('模板名称',{exact:true}).fill('混排插入模板');
 await dragTo(page,editor.getByRole('button',{name:'选择机柜 L05',exact:true}),editor.getByRole('button',{name:'查看设施 供电',exact:true}),.2);await expect(editor.locator('.cabinet-drop-indicator')).toContainText('供电前');await page.mouse.up();await editor.getByRole('button',{name:'保存模板',exact:true}).click();await expect(editor).toHaveCount(0);
 const s=await snapshot(request),template=s.templates.find(t=>t.name==='混排插入模板')!,preview=previewPlan(template.plan);expect(rowLayout(preview.module,preview.racks,preview.infra,'L').map(t=>t.name)).toEqual(['L01','L02','L05','供电','环境监测','L03','L04','空调']);expect(preview.infra.find(i=>i.name==='柜内环境')?.afterRack).toBe(1);
});
