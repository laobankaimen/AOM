import {test as base,expect} from '@playwright/test';
// These credentials exist only in the temporary database started by the test runner.
export const testAccount={username:'testadmin',password:'Only-for-e2e-tests-2026!'};
export const test=base.extend({
 page:async({page},use)=>{const response=await page.context().request.post('/api/auth/login',{data:testAccount});expect(response.ok()).toBeTruthy();await use(page)},
 request:async({page},use)=>{await use(page.context().request)},
});
export {expect};
