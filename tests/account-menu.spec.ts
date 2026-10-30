import {test,expect} from './fixtures';
import {readFileSync} from 'node:fs';
const packageInfo=JSON.parse(readFileSync(new URL('../package.json',import.meta.url),'utf8'));
test('右上角用户菜单集中账户操作，系统信息展示版本及开发者公司',async({page})=>{
 await page.goto('/');await expect(page.getByRole('heading',{name:'空间总览',exact:true})).toBeVisible();await expect(page.locator('.account-controls')).toHaveCount(0);
 const trigger=page.locator('.topbar-right').getByRole('button',{name:'用户菜单',exact:true});await trigger.click();const menu=page.getByRole('menu',{name:'账户操作',exact:true});await expect(menu).toContainText('testadmin');await expect(menu.getByRole('menuitem')).toHaveCount(3);
 await menu.getByRole('menuitem',{name:'系统信息',exact:true}).click();const dialog=page.getByRole('dialog',{name:'系统信息',exact:true});await expect(dialog).toBeVisible();await expect(dialog.locator('[data-system-version]')).toHaveText(packageInfo.version);await expect(dialog.locator('dd')).toHaveText([packageInfo.version,'zhoutao','zkhx']);await dialog.getByRole('button',{name:'关闭',exact:true}).click();await expect(trigger).toBeFocused();
 await trigger.press('ArrowDown');await expect(menu).toBeVisible();await expect(menu.getByRole('menuitem',{name:'修改登录密码',exact:true})).toBeFocused();await page.keyboard.press('ArrowDown');await expect(menu.getByRole('menuitem',{name:'系统信息',exact:true})).toBeFocused();await page.keyboard.press('Escape');await expect(menu).toHaveCount(0);await expect(trigger).toBeFocused();
 await trigger.click();await page.getByRole('heading',{name:'空间总览',exact:true}).click();await expect(menu).toHaveCount(0);
 await page.setViewportSize({width:390,height:844});await trigger.click();await expect(menu).toBeVisible();const box=(await menu.boundingBox())!;expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(390);await page.screenshot({path:'.cache/user-menu-mobile.png',fullPage:true});
});
