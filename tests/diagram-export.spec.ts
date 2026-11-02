import {test,expect} from './fixtures';
import {enterRoom,enterModule} from './navigation';
import {readFile} from 'node:fs/promises';
import type {Page} from '@playwright/test';
import type {State} from '../src/model';
async function exported(page:Page,format:'PNG 图片'|'PDF（单页）'|'SVG（矢量）'){
 await page.locator('.diagram-export summary').click();
 const pending=page.waitForEvent('download');await page.getByRole('button',{name:'导出 '+format,exact:true}).click();
 const download=await pending;expect(await download.failure()).toBeNull();const data=await readFile((await download.path())!);return {data,name:download.suggestedFilename()};
}
test('机房平面立体、模块与机柜图导出PNG、单页PDF及完整SVG',async({page,request})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 let s:State=await (await request.get('/api/state')).json();
 const response=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'图形导出验收',site_name:'中文机房',modules:[{name:'导出模块',type:'double',rack_left:3,rack_right:3,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(response.ok()).toBe(true);s=await response.json();
 const project=s.projects.find(p=>p.name==='图形导出验收')!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await enterRoom(page,room.name);
 const png=await exported(page,'PNG 图片');expect(png.name).toBe('中文机房立体布局图.png');expect(png.data.subarray(0,8).toString('hex')).toBe('89504e470d0a1a0a');expect(png.data.readUInt32BE(16)).toBeGreaterThan(800);expect(png.data.readUInt32BE(20)).toBeGreaterThan(300);
 const pdf=await exported(page,'PDF（单页）');expect(pdf.name.endsWith('.pdf')).toBe(true);expect(pdf.data.subarray(0,8).toString()).toBe('%PDF-1.4');expect(pdf.data.toString('latin1')).toContain('/Count 1');expect(pdf.data.toString('latin1')).toContain('/DCTDecode');
 await page.getByRole('button',{name:'平面布局',exact:true}).click();const flat=await exported(page,'SVG（矢量）');expect(flat.name).toBe('中文机房平面布局图.svg');expect(flat.data.toString()).toContain('导出模块');expect(flat.data.toString()).toContain('图形导出验收');
 await enterModule(page,module.name);const floor=await exported(page,'SVG（矢量）');for(const r of s.racks.filter(r=>r.moduleId===module.id))expect(floor.data.toString()).toContain(r.name);
 await page.getByRole('button',{name:'立面图',exact:true}).click();const racks=await exported(page,'SVG（矢量）');expect((racks.data.toString().match(/data-drop-rack=/g)||[]).length).toBe(6);expect(racks.data.toString()).toContain('当前显示 6 个机柜');
 expect((await (await request.get('/api/state')).json()).revision).toBe(s.revision);expect(errors).toEqual([]);
});
test('拓扑导出保留筛选范围并包含滚动区域外设备',async({page,request})=>{
 let s:State=await (await request.get('/api/state')).json();
 const planned=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:'拓扑导出验收',site_name:'导出机房',modules:[{name:'导出模块',type:'single',rack_count:2,u_height:42}],infra:{power:0,ac:0,env:0}}}});expect(planned.ok()).toBe(true);s=await planned.json();
 const project=s.projects.find(p=>p.name==='拓扑导出验收')!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const response=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,4,导出服务器,EXPORT'}});expect(response.ok()).toBe(true);s=await response.json();
 const devices=s.devices.filter(d=>d.moduleId===module.id);
 const net=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,name:'导出业务网',purpose:'business',color:'#497cda',deviceIds:devices.map(d=>d.id),devicePositions:{[devices[3].id]:{x:1900,y:100}}}}});expect(net.ok()).toBe(true);s=await net.json();
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'网络拓扑',exact:true}).click();
 await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption('detail');const svg=await exported(page,'SVG（矢量）');expect(svg.data.toString()).toContain(devices[3].name);expect(svg.data.toString()).toContain('导出业务网');expect(svg.data.toString()).toContain('全部网络');
 await page.getByRole('textbox',{name:'搜索拓扑设备',exact:true}).fill(devices[0].assetTag);const filtered=await exported(page,'SVG（矢量）');expect(filtered.data.toString()).toContain('搜索：'+devices[0].assetTag);expect(filtered.data.toString()).not.toContain(devices[3].name);
 await page.getByRole('textbox',{name:'搜索拓扑设备',exact:true}).fill('');await page.getByRole('combobox',{name:'拓扑展示方式',exact:true}).selectOption('auto');const overview=await exported(page,'PNG 图片');expect(overview.data.subarray(0,8).toString('hex')).toBe('89504e470d0a1a0a');
 expect((await (await request.get('/api/state')).json()).revision).toBe(s.revision);
});
