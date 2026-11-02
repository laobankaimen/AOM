import type {Cookie} from '@playwright/test';
import {test as base,expect} from '@playwright/test';
// These credentials exist only in the temporary database started by the test runner.
export const testAccount={username:'testadmin',password:'Only-for-e2e-tests-2026!'};
// Reuse a valid test session within each worker; repeated logins are not the
// behavior under test and can exhaust the real server login rate limit.
let sessionCookies:Cookie[]=[];
export const test=base.extend({
 page:async({page},use)=>{if(sessionCookies.length)await page.context().addCookies(sessionCookies);const status=await(await page.context().request.get('/api/auth/status')).json();if(!status.authenticated){const response=await page.context().request.post('/api/auth/login',{data:testAccount});expect(response.ok()).toBeTruthy();sessionCookies=await page.context().cookies()}await use(page)},
 request:async({page},use)=>{await use(page.context().request)},
});
export {expect};
