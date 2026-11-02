import {openNetworkPanel,closeNetworkPanel,openCanvasMenu} from './navigation';
import {test,expect} from './fixtures';
import {type Page} from '@playwright/test';

async function checkSearch(page:Page,label:string){
 const input=page.getByRole('textbox',{name:label,exact:true});const box=input.locator('..');
 await expect(input).toBeVisible();const before=await box.boundingBox();
 await input.click();await expect(input).toBeFocused();
 await expect.poll(()=>box.evaluate(el=>getComputedStyle(el).borderTopColor)).toBe('rgb(37, 99, 235)');
 const style=await input.evaluate(el=>{const input=getComputedStyle(el),wrapper=getComputedStyle(el.parentElement!);return {outline:input.outlineStyle,shadow:input.boxShadow,border:input.borderTopWidth,wrapperBorder:wrapper.borderTopColor,wrapperShadow:wrapper.boxShadow,fontSize:input.fontSize,color:input.color}});
 expect(style.outline).toBe('none');expect(style.shadow).toBe('none');expect(style.border).toBe('0px');expect(style.wrapperBorder).toBe('rgb(37, 99, 235)');expect(style.wrapperShadow).not.toBe('none');expect(style.color).toBe('rgb(15, 23, 42)');
 expect(await box.boundingBox()).toEqual(before);
 await input.press('Tab');await expect(input).not.toBeFocused();await expect.poll(()=>box.evaluate(el=>getComputedStyle(el).boxShadow)).toBe('none');
 await page.keyboard.press('Shift+Tab');await expect(input).toBeFocused();expect(await input.evaluate(el=>getComputedStyle(el).outlineStyle)).toBe('none');
 const rect=await input.boundingBox(),outer=await box.boundingBox();expect(rect!.x).toBeGreaterThan(outer!.x);expect(rect!.x+rect!.width).toBeLessThanOrEqual(outer!.x+outer!.width);return box;
}

test('搜索框鼠标和键盘聚焦仅显示外框，核心文字与主操作清晰',async({page})=>{
 await page.goto('/');await page.getByRole('link',{name:'设备台账',exact:true}).click();await checkSearch(page,'搜索设备');await page.screenshot({path:'.cache/theme-assets-focus.png',fullPage:true});
 const primary=page.getByRole('button',{name:'新增设备',exact:true});const colors=await primary.evaluate(el=>({color:getComputedStyle(el).color,background:getComputedStyle(el).backgroundColor}));expect(colors).toEqual({color:'rgb(255, 255, 255)',background:'rgb(29, 78, 216)'});
 await page.getByRole('link',{name:'型号库',exact:true}).click();await checkSearch(page,'搜索型号');
 await page.setViewportSize({width:390,height:844});await checkSearch(page,'搜索型号');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();await page.screenshot({path:'.cache/theme-search-mobile.png',fullPage:true});
});

test('拓扑搜索框焦点与普通表单键盘焦点均保留',async({page,request})=>{
 let s=await(await request.get('/api/state')).json();const result=await request.post('/api/devices',{data:{revision:s.revision,autoPlace:true,device:{assetTag:'THEME-FOCUS-DEVICE',name:'配色验收设备',modelId:'model-r722',moduleId:s.modules[0].id,status:'pending'}}});expect(result.ok()).toBeTruthy();
 await page.goto('/');await page.getByRole('link',{name:'网络拓扑',exact:true}).click();await checkSearch(page,'搜索拓扑设备');await page.screenshot({path:'.cache/theme-topology-focus.png',fullPage:true});
 await openNetworkPanel(page);await page.locator('.action-menu summary').filter({hasText:'拓扑更多操作'}).click();await page.getByRole('button',{name:'新建设备',exact:true}).click();const field=page.getByRole('textbox',{name:'资产编号',exact:true});await field.focus();await field.press('Tab');await page.keyboard.press('Shift+Tab');await expect(field).toBeFocused();expect(await field.evaluate(el=>getComputedStyle(el).outlineStyle)).toBe('solid');
});
