import {enterRoom,enterModule,manualCreate} from './navigation';
import {test,expect} from './fixtures';
import {type APIRequestContext,type Page} from '@playwright/test';
import { readFile } from 'node:fs/promises';
import type { Device, State } from '../src/model';

const snapshot = async (request: APIRequestContext): Promise<State> => {
  const response = await request.get('/api/state');
  expect(response.ok()).toBeTruthy();
  return response.json();
};

// Each placement test owns a separate site. API setup supplies fixtures; every
// behavior under test (dragging, editing, exporting) still uses the browser UI.
async function placementFixture(request: APIRequestContext, page: Page, name: string) {
  let state = await snapshot(request);
  const planResponse = await request.post('/api/plans', {
    data: {
      revision: state.revision,
      plan: {
        site_name: name,
        modules: [{ name: '验收模块', type: 'single', rack_count: 2, u_height: 42 }],
        infra: { power: 1, ac: 1, env: 1 },
      },
    },
  });
  expect(planResponse.status()).toBe(201);
  state = await planResponse.json();
  const site = state.sites.find(site => site.name === name)!;
  const module = state.modules.find(module => module.siteId === site.id)!;
  const response = await request.post('/api/import/commit', {
    data: {
      revision: state.revision,
      moduleId: module.id,
      content: `model,quantity,name,asset_prefix\nKunTai R722 K2,2,验收设备,${name}`,
    },
  });
  expect(response.status()).toBe(201);
  state = await response.json();
  const devices = state.devices.filter(device => device.moduleId === module.id);
  const targetRack = state.racks.find(rack => rack.moduleId === module.id && rack.name === 'L02')!;
  await page.goto('/');
  await enterRoom(page,site.name);await enterModule(page,module.name);
  await page.getByRole('button', { name: '立面图',exact:true }).click();
  return { state, devices, targetRack };
}

async function dragToU(page: Page, device: Device, targetU: number) {
  const cabinet = page.locator('.elevation-card').filter({
    has: page.getByRole('button', { name: `${device.name} U${device.startU}`, exact: true }),
  });
  const svg = cabinet.locator('.rack-elevation');
  const bounds = (await svg.boundingBox())!;
  const unitPoint = (u: number) => ({
    x: bounds.x + 130 / 280 * bounds.width,
    y: bounds.y + (36 + (42 - u) * 20 + 10) / 934 * bounds.height,
  });
  const start = unitPoint(device.startU);
  const end = unitPoint(targetU);
  expect(start.y).toBeLessThan(page.viewportSize()!.height);
  expect(end.y).toBeLessThan(page.viewportSize()!.height);
  await page.mouse.move(start.x, start.y);
  await page.mouse.down();
  await page.mouse.move(end.x, end.y, { steps: 12 });
  await page.mouse.up();
}

test('CSV预览不写库、确认导入与台账持久化', async ({ page, request }) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  let before = await snapshot(request);
  const created=await request.post('/api/plans',{data:{revision:before.revision,plan:{project_name:'CSV独立验收项目',site_name:'CSV独立机房',modules:[{name:'CSV独立模块',type:'single',rack_count:5,u_height:42}],infra:{power:1,ac:1,env:1}}}});
  expect(created.ok()).toBeTruthy();before=await created.json();
  const ownProject=before.projects.find(p=>p.name==='CSV独立验收项目')!;
  await page.goto('/');await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(ownProject.id);
  await expect(page.getByRole('heading', { name: '空间总览', exact: true })).toBeVisible();
  await expect(page.locator('.floor-panel')).toHaveCount(0);
  await enterRoom(page,'CSV独立机房');await enterModule(page,'CSV独立模块');
  await expect(page.getByRole('heading',{name:'选择机柜或设施'})).toBeVisible();await page.getByRole('button',{name:'选择机柜 L01',exact:true}).click();await expect(page.getByText('等待第一台设备')).toBeVisible();
  await page.screenshot({ path: '.cache/overview-empty.png', fullPage: true });
  await page.getByRole('button', { name: '导入设备', exact: true }).click();
  await page.getByRole('button', { name: '使用示例清单' }).click();
  await page.getByRole('button', { name: '预览自动排位' }).click();
  await expect(page.getByRole('button', { name: '确认导入 19 台' })).toBeVisible();
  expect((await snapshot(request)).devices).toHaveLength(before.devices.length);
  await expect(page.getByText('型号U高待确认')).toBeVisible();
  await page.getByRole('button', { name: '确认导入 19 台' }).click();
  await expect(page.getByRole('region',{name:'批量导入设备',exact:true})).toHaveCount(0);
  const after = await snapshot(request);
  const imported = after.devices.filter(d => !before.devices.some(old => old.id === d.id));
  expect(imported).toHaveLength(19);
  expect(imported.filter(d => d.rackId)).toHaveLength(18);
  expect(after.revision).toBe(before.revision + 1);
  await page.screenshot({ path: '.cache/overview-populated.png', fullPage: true });
  await page.getByRole('button', { name: '设备台账', exact: true }).click();
  await page.reload();
  await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption(ownProject.id);
  await expect(page.getByRole('heading', { name: '设备台账', exact: true })).toBeVisible();
  await expect(page.locator('tbody tr')).toHaveCount(19);
  await page.getByRole('button', { name: '变更审计', exact: true }).click();
  await expect(page.getByRole('cell', { name: /CSV导入19台/ })).toBeVisible();
  expect(errors).toEqual([]);
});

test('立面拖拽成功、U位冲突回滚且不新增审计', async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 1600 });
  const { state: before, devices } = await placementFixture(request, page, '拖拽验收机房');
  const device = devices[0];
  expect(device.startU).toBe(1);
  await dragToU(page, device, 41);
  await expect.poll(async () => (await snapshot(request)).devices.find(d => d.id === device.id)?.startU).toBe(41);
  const afterMove = await snapshot(request);
  expect(afterMove.revision).toBe(before.revision + 1);
  const moved = afterMove.devices.find(d => d.id === device.id)!;
  await expect(page.getByRole('button', { name: `${device.name} U41`, exact: true })).toBeVisible();
  await dragToU(page, moved, devices[1].startU);
  await expect(page.getByRole('status')).toContainText('U位冲突');
  const afterConflict = await snapshot(request);
  expect(afterConflict.devices.find(d => d.id === device.id)?.startU).toBe(41);
  expect(afterConflict.revision).toBe(afterMove.revision);
  const audits = await (await request.get('/api/audits')).json();
  expect(audits.filter((a: { revision: number }) => a.revision > before.revision)).toHaveLength(1);
  await page.screenshot({ path: '.cache/elevation.png', fullPage: true });
});

test('编辑页面跨柜移动、台账导出与审计快照', async ({ page, request }) => {
  await page.setViewportSize({ width: 1440, height: 1600 });
  const { state: before, devices, targetRack } = await placementFixture(request, page, '跨柜验收机房');
  const device = devices[0];
  await page.getByRole('button', { name: `${device.name} U${device.startU}`, exact: true }).click();
  await page.getByRole('button', { name: '编辑详细信息' }).click();
  const dialog = page.getByRole('region', { name: '编辑设备 / 调整机位', exact: true });
  await expect(dialog).toBeVisible();
  // Explicit label stays stable regardless of the options and remaining U values.
  await expect(dialog.getByLabel('机柜', { exact: true })).toHaveCount(1);
  const rackSelect = dialog.getByRole('combobox', { name: '机柜', exact: true });
  await rackSelect.selectOption(targetRack.id);
  await expect(rackSelect).toHaveValue(targetRack.id);
  await dialog.getByRole('spinbutton', { name: '起始 U 位', exact: true }).fill('1');
  await dialog.getByRole('button', { name: '保存设备', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const after = await snapshot(request);
  const updated = after.devices.find(d => d.id === device.id)!;
  expect(updated.rackId).toBe(targetRack.id);
  expect(updated.startU).toBe(1);
  expect(after.revision).toBe(before.revision + 1);
  await page.getByRole('button', { name: '设备台账', exact: true }).click();
  await page.getByRole('textbox', { name: '搜索设备' }).fill(device.assetTag);
  await expect(page.locator('tbody tr')).toHaveCount(1);
  await expect(page.locator('tbody tr')).toContainText('L02');
  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('link', { name: '导出台账' }).click();
  const exported = await readFile((await (await downloadPromise).path())!, 'utf8');
  expect(exported).toContain(device.assetTag);
  expect(exported.split('\n').find(line => line.includes(device.assetTag))).toContain('L02');
  await page.getByRole('button', { name: '变更审计', exact: true }).click();
  const auditRow = page.getByRole('row').filter({ hasText: `修改设备：${device.assetTag}` });
  await expect(auditRow).toBeVisible();
  const auditDownloadPromise = page.waitForEvent('download');
  await auditRow.getByRole('link', { name: '下载 JSON' }).click();
  const audit = JSON.parse(await readFile((await (await auditDownloadPromise).path())!, 'utf8'));
  expect(audit.before.devices.find((d: Device) => d.id === device.id).rackId).toBe(device.rackId);
  expect(audit.after.devices.find((d: Device) => d.id === device.id).rackId).toBe(targetRack.id);
});

test('标准向导与可复用规划，手机布局', async ({ page, request }) => {
  await page.goto('/');
  await manualCreate(page);
  await page.getByLabel('机房名称', { exact: true }).fill('验收机房');
  await page.getByLabel('机柜数量', { exact: true }).fill('2');
  await page.getByRole('button', { name: '增加模块' }).click();
  const second = page.locator('.module-form').nth(1);
  await second.getByRole('combobox', { name: '排列方式', exact: true }).selectOption('double');
  await second.getByLabel('左列机柜数').fill('2');
  await second.getByLabel('右列机柜数').fill('3');
  await page.getByRole('button', { name: '生成机房基座' }).click();
  await expect(page.getByRole('region',{name:'标准机房创建向导',exact:true})).toHaveCount(0);
  const state = await snapshot(request);
  const site = state.sites.find(site => site.name === '验收机房')!;
  expect(site).toBeTruthy();
  const modules = state.modules.filter(module => module.siteId === site.id);
  expect(modules).toHaveLength(2);
  expect(state.racks.filter(rack => modules.some(module => module.id === rack.moduleId))).toHaveLength(7);
  await enterModule(page,'微模块 2');
  await expect(page.getByText('← 冷通道 / COLD AISLE →')).toBeVisible();
  const plan = await (await request.get('/api/export/plan.json?siteId=' + site.id)).json();
  expect(plan.modules[1].rack_left).toBe(2);
  expect(plan.modules[1].rack_right).toBe(3);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: '.cache/mobile.png', fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
});

test('型号维护、手工新增与引用保护', async ({ page, request }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '型号库', exact: true }).click();
  await page.getByRole('button', { name: '新增型号' }).click();
  await page.getByLabel('型号名称').fill('验收 4U 服务器');
  await page.getByLabel('厂商', { exact: true }).fill('测试厂商');
  await page.getByLabel('设备高度（U）').fill('4');
  await page.getByRole('button', { name: '保存型号' }).click();
  await expect(page.getByRole('region',{name:'新增设备型号',exact:true})).toHaveCount(0);
  await page.getByRole('button', { name: '设备台账', exact: true }).click();
  await page.getByRole('button', { name: '新增设备', exact: true }).click();
  await page.getByLabel('资产编号', { exact: true }).fill('TEST-4U');
  await page.getByRole('combobox', { name: '设备型号', exact: true }).selectOption({ label: '验收 4U 服务器 · 4U' });
  await page.getByRole('button', { name: '保存设备' }).click();
  await expect(page.getByRole('region', {name:'新增设备',exact:true})).toHaveCount(0);
  expect((await snapshot(request)).devices.find(d => d.assetTag === 'TEST-4U')?.rackId).toBeTruthy();
  await page.getByRole('button', { name: '型号库', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: '验收 4U 服务器' });
  await row.getByRole('button', { name: '删除', exact: true }).click();
  await page.getByRole('button', { name: '确认删除' }).click();
  await expect(page.getByRole('alert')).toContainText('型号已被设备引用');
});

test('项目下同一机房总览三个微模块，并编辑右列空调位置', async ({page,request}) => {
  const before=await snapshot(request);
  const defaultProject=before.projects.find(p=>p.id==='project-default')!;
  const room=before.sites.find(s=>s.projectId===defaultProject.id&&s.name==='主机房')!;
  const modules=before.modules.filter(m=>m.siteId===room.id);
  expect(modules).toHaveLength(3);
  expect(modules.filter(m=>m.productModel.includes('SG6600'))).toHaveLength(2);
  expect(modules.filter(m=>m.productModel.includes('C500-40K'))).toHaveLength(1);
  await page.goto('/');
  await expect(page.getByRole('combobox',{name:'选择项目',exact:true})).toHaveValue(defaultProject.id);
  await expect(page.locator('.overview-module')).toHaveCount(0);
  await enterRoom(page,room.name);await expect(page.locator('.overview-module')).toHaveCount(3);
  await page.screenshot({path:'.cache/room-overview-v2.png',fullPage:true});
  const double=modules.find(m=>m.type==='double')!;
  await enterModule(page,double.name);
  await page.getByRole('button',{name:'编辑微模块详细信息',exact:true}).click();
  const dialog=page.getByRole('region',{name:'微模块布局配置',exact:true});
  await dialog.getByRole('button',{name:'新增设施',exact:true}).click();
  await dialog.getByLabel('设施名称',{exact:true}).fill('右列新增列间空调');
  await dialog.getByRole('combobox',{name:'设施所属列',exact:true}).selectOption('R');
  await dialog.getByRole('combobox',{name:'设施列内位置',exact:true}).selectOption('2');
  await dialog.getByRole('button',{name:'新增基础设施',exact:true}).click();
  await expect(dialog.getByRole('status')).toContainText('布局已保存');
  const after=await snapshot(request);
  const ac=after.infra.find(i=>i.name==='右列新增列间空调')!;
  expect(ac.moduleId).toBe(double.id);expect(ac.side).toBe('R');expect(ac.afterRack).toBe(2);
  await dialog.getByRole('button',{name:'返回查看',exact:true}).click();
  await expect(page.getByRole('button',{name:'查看设施 右列新增列间空调',exact:true})).toBeVisible();
  await page.goto('/');await enterRoom(page,room.name);
  await enterModule(page,double.name);
  await expect(page.getByRole('button',{name:'查看设施 右列新增列间空调',exact:true})).toBeVisible();
  const plan=await (await request.get('/api/export/plan.json?siteId='+room.id)).json();
  expect(plan.modules.find((m:any)=>m.type==='double').layout).toContainEqual({name:ac.name,kind:'ac',side:'R',after_rack:2,placement:'inline'});
  const c500=modules.find(m=>m.productModel.includes('C500'))!;
  await page.getByRole('navigation',{name:'位置导航'}).getByRole('button',{name:`返回机房 ${room.name}`,exact:true}).click();await enterModule(page,c500.name);await page.getByRole('button',{name:'选择机柜 L02',exact:true}).click();await page.getByRole('button',{name:'查看 42U 立面',exact:true}).click();
  await expect(page.getByRole('heading',{name:'42U 机柜立面',exact:true})).toBeVisible();
  await expect(page.locator('.elevation-card')).toHaveCount(4);
});

test('创建新项目与机房，项目切换隔离空间', async ({page,request})=>{
  await page.goto('/');
  await manualCreate(page);
  const dialog=page.getByRole('region',{name:'标准机房创建向导',exact:true});
  await dialog.getByRole('combobox',{name:'规划所属项目',exact:true}).selectOption('');
  await dialog.getByLabel('新项目名称',{exact:true}).fill('新建独立项目');
  await dialog.getByLabel('机房名称',{exact:true}).fill('新项目主机房');
  await dialog.getByLabel('微模块型号',{exact:true}).fill('曙光数创 C500（C500-40K）');
  await dialog.getByLabel('点位标识',{exact:true}).fill('独立点位');
  await dialog.getByRole('button',{name:'生成机房基座'}).click();
  await expect(dialog).toHaveCount(0);
  const state=await snapshot(request);const project=state.projects.find(p=>p.name==='新建独立项目')!;
  const room=state.sites.find(s=>s.name==='新项目主机房')!;
  expect(room.projectId).toBe(project.id);
  await expect(page.getByRole('combobox',{name:'选择项目',exact:true})).toHaveValue(project.id);
  await expect(page.locator('.overview-module')).toHaveCount(1);
  await page.getByRole('combobox',{name:'选择项目',exact:true}).selectOption('project-default');
  await enterRoom(page,'主机房');await expect(page.locator('.overview-module')).toHaveCount(3);
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
});
