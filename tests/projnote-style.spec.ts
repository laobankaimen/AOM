import {test,expect} from './fixtures';
import type {APIRequestContext,Page} from '@playwright/test';
import type {State} from '../src/model';
import {navigationHash} from '../src/navigationState';
import {enterRoom,enterModule} from './navigation';

const snapshot=async(api:APIRequestContext):Promise<State>=>(await api.get('/api/state')).json();
async function fixture(page:Page,api:APIRequestContext,name='数据中心运维'){
 let s=await snapshot(api);
 const planned=await api.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'主机房',modules:[{name:'业务计算模块',type:'double',rack_count:8,rack_left:4,rack_right:4,u_height:42}],infra:{power:1,ac:1,env:1}}}});
 expect(planned.ok()).toBeTruthy();s=await planned.json();
 const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await api.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,12,业务计算服务器,STYLE-SERVER-'+project.id+'\nRZ2805-48YS8CQ,4,业务接入交换机,STYLE-SW-'+project.id}});
 expect(imported.ok()).toBeTruthy();s=await imported.json();
 const devices=s.devices.filter(d=>d.moduleId===module.id);
 for(const [index,name] of ['数据网络','管理网络'].entries()){
  const network=await api.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,siteId:room.id,name,purpose:index?'management':'business',color:index?'#4f958e':'#6287b8',deviceIds:devices.slice(index*8,index*8+8).map(d=>d.id),layersVisible:true}}});
  expect(network.ok()).toBeTruthy();s=await network.json();
 }
 await page.goto('/'+navigationHash({view:'overview',project:project.id,room:'',module:'',rack:'',network:''}));
 return {project,room,module,devices};
}

// These are the ProjNote design contract, rather than snapshots of arbitrary CSS.
test('ProjNote桌面设计规范：共享配色、字体、壳层尺寸及紧凑事实与机房行',async({page,request})=>{
 const {room}=await fixture(page,request);
 const styles=await page.evaluate(()=>{
  const css=(selector:string)=>getComputedStyle(document.querySelector(selector)!);
  return {background:css('body').backgroundColor,font:css('body').fontFamily,text:css('body').color,sidebar:css('.sidebar').backgroundColor,sidebarWidth:css('.sidebar').width,topbarHeight:css('.topbar').height,button:css('.heading-actions .primary').backgroundColor,buttonShadow:css('.heading-actions .primary').boxShadow,title:css('.page-heading h1').fontSize};
 });
 expect(styles).toMatchObject({background:'rgb(243, 246, 250)',text:'rgb(36, 51, 74)',sidebar:'rgb(35, 51, 72)',sidebarWidth:'210px',topbarHeight:'72px',button:'rgb(36, 101, 206)',buttonShadow:'none',title:'25px'});
 expect(styles.font).toContain('Noto Sans CJK SC');
 await expect(page.locator('.page-heading .eyebrow,.page-heading .heading-badge')).toHaveCount(0);
 const metrics=page.locator('.home-metrics');expect((await metrics.boundingBox())!.height).toBeLessThan(65);
 expect(await metrics.locator('div').first().evaluate(el=>getComputedStyle(el).backgroundColor)).toBe('rgba(0, 0, 0, 0)');
 for(const viewport of [{width:1280,height:800},{width:1440,height:1000},{width:1920,height:1080}]){
  await page.setViewportSize(viewport);const row=page.getByRole('link',{name:'进入机房 '+room.name,exact:true});expect((await row.boundingBox())!.height).toBeLessThanOrEqual(132);await expect(row).toContainText('8 机柜');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.screenshot({path:`.cache/projnote-overview-${viewport.width}.png`,fullPage:true});
 }
 await page.locator('.home-metrics button').click();await expect(page.locator('.table-footer')).toContainText('16 台设备');
});

test('ProjNote风格走查：台账、型号、模板、审计、机房、模块、拓扑和详情编辑',async({page,request})=>{
 const {room,module,devices}=await fixture(page,request,'运维界面走查'),errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 for(const [name,file] of [['设备台账','assets'],['型号库','models'],['布局模板','templates'],['变更审计','audit']] as const){
  await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name,exact:true}).click();await expect(page.getByRole('heading',{name,level:1,exact:true})).toBeVisible();await page.screenshot({path:'.cache/projnote-'+file+'.png',fullPage:true});
 }
 await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'空间总览',exact:true}).click();await enterRoom(page,room.name);await page.screenshot({path:'.cache/projnote-room.png',fullPage:true});await enterModule(page,module.name);await page.screenshot({path:'.cache/projnote-module.png',fullPage:true});
 await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'网络拓扑',exact:true}).click();await expect(page.locator('.topology-canvas')).toBeVisible();await page.screenshot({path:'.cache/projnote-topology.png',fullPage:true});
 await page.getByRole('button',{name:'选择网络 数据网络',exact:true}).click();await page.getByRole('button',{name:'修改网络 数据网络',exact:true}).click();const toolbar=await page.locator('.topology-command-header').boundingBox(),panel=await page.getByRole('complementary',{name:'网络列表面板',exact:true}).boundingBox();expect(panel!.y).toBeGreaterThanOrEqual(toolbar!.y+toolbar!.height);await page.screenshot({path:'.cache/projnote-topology-editor.png',fullPage:true});await expect(page.getByRole('button',{name:'保存网络',exact:true})).toBeInViewport();
 await page.setViewportSize({width:1280,height:800});const quick=page.getByRole('region',{name:'快速网络配置',exact:true});await quick.locator('.selection-tools summary').click();await expect(quick.getByRole('button',{name:'保存网络',exact:true})).toBeInViewport();await quick.locator('.quick-network-fields').evaluate(el=>el.scrollTop=el.scrollHeight);await expect(quick.getByRole('button',{name:'保存网络',exact:true})).toBeInViewport();await page.getByRole('button',{name:'关闭快捷操作',exact:true}).click();await page.setViewportSize({width:1440,height:1000});
 await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'设备台账',exact:true}).click();await page.getByRole('button',{name:'新增设备',exact:true}).click();await expect(page.locator('.editor-page')).toBeVisible();expect((await page.locator('.editor-page').boundingBox())!.y).toBe(72);await page.screenshot({path:'.cache/projnote-device-editor.png',fullPage:true});await page.getByRole('button',{name:'返回查看',exact:true}).click();
 expect(errors).toEqual([]);expect((await snapshot(request)).devices.filter(d=>devices.some(old=>old.id===d.id))).toEqual(devices);
});

test('登录和改密独立纸面界面使用相同字体与配色，修改密码不会叠加业务页面',async({page})=>{
 await page.goto('/');await page.getByRole('button',{name:'用户菜单',exact:true}).click();await page.getByRole('menuitem',{name:'修改登录密码',exact:true}).click();await expect(page.getByRole('heading',{name:'修改密码',exact:true})).toBeVisible();await expect(page.locator('.app-shell')).toHaveCount(0);expect(await page.locator('.auth-screen').evaluate(el=>getComputedStyle(el).backgroundImage)).toBe('none');await page.screenshot({path:'.cache/projnote-password.png',fullPage:true});await page.getByRole('button',{name:'返回工作空间',exact:true}).click();await page.context().clearCookies();await page.reload();await expect(page.getByRole('heading',{name:'登录工作空间',exact:true})).toBeVisible();await page.screenshot({path:'.cache/projnote-login.png',fullPage:true});
});
