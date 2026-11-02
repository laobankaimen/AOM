import {expect,type Page} from '@playwright/test';
export async function enterRoom(page:Page,name:string){await page.getByRole('link',{name:`进入机房 ${name}`,exact:true}).click();await expect(page.getByRole('heading',{name:'微模块总览',exact:true})).toBeVisible()}
export async function enterModule(page:Page,name:string){const map=page.getByRole('group',{name:'机房微模块位置图',exact:true});if(await map.count()){await map.getByRole('button',{name:`选择布局模块 ${name}`,exact:true}).click();await page.getByRole('button',{name:'进入模块',exact:true}).click()}else{await page.getByRole('button',{name:`进入微模块 ${name}`,exact:true}).click()}await expect(page.getByRole('group',{name:`${name}俯视图`,exact:true})).toBeVisible()}

export async function manualCreate(page:Page){await page.locator('.heading-actions .action-menu summary').click();await page.getByRole('button',{name:'手动创建机房',exact:true}).click()}

export async function openNetworkPanel(page:Page){const button=page.getByRole('button',{name:'网络与分层',exact:true});if(await button.getAttribute('aria-expanded')!=='true')await button.click()}
export async function closeNetworkPanel(page:Page){const button=page.getByRole('button',{name:'网络与分层',exact:true});if(await button.getAttribute('aria-expanded')==='true')await button.click()}
export async function openCanvasMenu(page:Page){await closeNetworkPanel(page);const details=page.locator('.canvas-controls .action-menu');if(await details.getAttribute('open')===null)await details.locator('summary').click()}
