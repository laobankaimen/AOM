import {openNetworkPanel,closeNetworkPanel,openCanvasMenu} from './navigation';
import {test,expect} from './fixtures';
import type {State} from '../src/model';
import type {Page,APIRequestContext} from '@playwright/test';
async function setup(page:Page,request:APIRequestContext,name:string){
 let s:State=await(await request.get('/api/state')).json();
 const planned=await request.post('/api/plans',{data:{revision:s.revision,plan:{project_name:name,site_name:'导航机房',modules:[{name:'导航模块',type:'single',rack_count:2,u_height:42}],infra:{power:1,ac:1,env:1}}}});expect(planned.ok()).toBe(true);s=await planned.json();const project=s.projects.find(p=>p.name===name)!,room=s.sites.find(r=>r.projectId===project.id)!,module=s.modules.find(m=>m.siteId===room.id)!;
 const imported=await request.post('/api/import/commit',{data:{revision:s.revision,moduleId:module.id,content:'model,quantity,name,asset_prefix\nKunTai R722 K2,1,导航服务器,NAV-'+project.id}});expect(imported.ok()).toBe(true);s=await imported.json();const device=s.devices.find(d=>d.moduleId===module.id)!;
 for(const [network,parent] of [['业务网',''],['计算子网','业务网']] as const){const response=await request.post('/api/networks',{data:{revision:s.revision,network:{projectId:project.id,name:network,parentId:s.networks.find(n=>n.projectId===project.id&&n.name===parent)?.id||'',purpose:'business',color:'#497cda',deviceIds:parent?[device.id]:[]}}});expect(response.ok()).toBe(true);s=await response.json()}
 await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(project.id);await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'网络拓扑',exact:true}).click();
 return {s,project,device,room,module,child:s.networks.find(n=>n.projectId===project.id&&n.name==='计算子网')!};
}
async function child(page:Page){await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 业务网',exact:true}).click();await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 计算子网',exact:true}).click()}
test('顶部统一网络路径，项目与侧栏回到总拓扑，编辑返回保持网络',async({page,request})=>{
 const {s,project}=await setup(page,request,'网络导航验收');const trail=page.getByRole('navigation',{name:'网络层级'}),nav=page.getByRole('navigation',{name:'主导航'});
 await child(page);await page.screenshot({path:'.cache/unified-topology-navigation.png',fullPage:true});await expect(page.locator('.topbar').getByRole('navigation',{name:'网络层级'})).toHaveCount(1);await expect(page.locator('.topology-workspace .network-breadcrumbs')).toHaveCount(0);await expect(trail).toContainText('项目拓扑业务网计算子网');
 await trail.getByRole('link',{name:'业务网',exact:true}).click();await openNetworkPanel(page);await expect(page.getByRole('button',{name:'筛选网络 计算子网',exact:true})).toBeVisible();
 await openNetworkPanel(page);await page.getByRole('button',{name:'筛选网络 计算子网',exact:true}).click();await openNetworkPanel(page);await page.getByRole('button',{name:'编辑当前网络',exact:true}).click();await page.getByRole('button',{name:'高级编辑',exact:true}).click();
 await expect(trail).toContainText('计算子网');await expect(trail).toContainText('编辑网络');await page.getByRole('button',{name:'返回查看',exact:true}).click();await expect(trail.getByRole('link',{name:'计算子网',exact:true})).toHaveAttribute('aria-current','page');
 await page.getByRole('textbox',{name:'搜索拓扑设备',exact:true}).fill('不存在的设备');await trail.getByRole('link',{name:`返回项目 ${project.name}`,exact:true}).click();
 await expect(page.locator('.topology-workspace')).toBeVisible();await expect(page.locator('.project-home')).toHaveCount(0);await expect(page.getByRole('textbox',{name:'搜索拓扑设备',exact:true})).toHaveValue('');await expect(trail).not.toContainText('计算子网');
 await child(page);await nav.getByRole('link',{name:'网络拓扑',exact:true}).click();await expect(trail).not.toContainText('计算子网');await openNetworkPanel(page);await expect(page.getByRole('button',{name:'筛选网络 业务网',exact:true})).toBeVisible();
 expect((await(await request.get('/api/state')).json()).revision).toBe(s.revision);
});
test('拓扑设备编辑和机房定位可返回原网络，浏览器返回前进与刷新保留层级',async({page,request})=>{
 const {s,project,child:network}=await setup(page,request,'导航历史验收');const trail=page.getByRole('navigation',{name:'网络层级'});
 await child(page);await expect(page).toHaveURL(new RegExp('network='+network.id));
 await page.getByRole('button',{name:'查看拓扑设备 导航服务器',exact:true}).click();await page.getByRole('button',{name:'编辑设备详情',exact:true}).click();await expect(trail).toContainText('计算子网');await expect(page.getByRole('navigation',{name:'位置导航'})).not.toContainText('导航机房');await page.getByRole('button',{name:'返回查看',exact:true}).click();
 await expect(trail).toContainText('计算子网');await page.getByRole('button',{name:'在机房中定位',exact:false}).click();await expect(page.getByRole('heading',{name:'42U 机柜立面',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'返回原网络拓扑',exact:false}).click();await expect(trail).toContainText('计算子网');await expect(page.locator('.topology-inspector')).toContainText('导航服务器');
 await trail.getByRole('link',{name:`返回项目 ${project.name}`,exact:true}).click();await expect(page).not.toHaveURL(/network=/);
 await page.goBack();await expect(trail).toContainText('计算子网');await page.goForward();await expect(trail).not.toContainText('计算子网');
 await child(page);await expect(page).toHaveURL(new RegExp('network='+network.id));await page.reload();await expect(trail).toContainText('计算子网');await expect(page.getByRole('combobox',{name:'选择项目',exact:true})).toHaveValue(project.id);
 await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(s.projects.find(p=>p.id!==project.id)!.id);await expect(page.getByRole('heading',{name:'网络拓扑',level:1,exact:true})).toBeVisible();await expect(trail).not.toContainText('计算子网');
 expect((await(await request.get('/api/state')).json()).revision).toBe(s.revision);
});

test('项目切换与非空间编辑保持工作区，保存时导航不能打断',async({page,request})=>{
 const {s,project}=await setup(page,request,'导航保存范围验收');
 await openNetworkPanel(page);await page.getByRole('button',{name:'新增网络分区',exact:true}).click();const quick=page.getByRole('region',{name:'快速网络配置',exact:true});await quick.getByLabel('网络名称',{exact:true}).fill('保存中网络');
 let release!:()=>void;const gate=new Promise<void>(resolve=>release=resolve);await page.route('**/api/networks',async route=>{if(route.request().method()==='POST'){await gate}await route.continue()});
 await quick.getByRole('button',{name:'创建网络',exact:true}).click();await expect(page.getByRole('combobox',{name:'选择项目',exact:true})).toBeDisabled();await expect(page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'空间总览',exact:true})).toBeDisabled();await expect(page.getByRole('link',{name:`返回项目 ${project.name}`,exact:true})).toBeDisabled();
 await page.goBack();await expect(quick).toBeVisible();await expect(page).toHaveURL(/#topology/);
 release();await expect(quick).toHaveCount(0);await page.unroute('**/api/networks');await expect(page.getByRole('navigation',{name:'网络层级'})).toContainText('保存中网络');
 await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'型号库',exact:true}).click();await page.getByRole('button',{name:'新增型号',exact:true}).click();
 const crumbs=page.getByRole('navigation',{name:'位置导航'});await expect(crumbs).toContainText('型号库');await expect(crumbs).not.toContainText('导航模块');await crumbs.getByRole('link',{name:'返回型号库',exact:true}).click();await expect(page.getByRole('heading',{name:'型号库',level:1,exact:true})).toBeVisible();await expect(page.locator('.editor-page')).toHaveCount(0);
 await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(s.projects.find(p=>p.id!==project.id)!.id);await expect(page.getByRole('heading',{name:'型号库',level:1,exact:true})).toBeVisible();
 await page.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'网络拓扑',exact:true}).click();await expect(page.getByRole('navigation',{name:'网络层级'})).not.toContainText('保存中网络');
});
