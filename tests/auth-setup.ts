import {request} from '@playwright/test';
import {readFile} from 'node:fs/promises';
export default async function setup(){const api=await request.newContext({baseURL:'http://127.0.0.1:18080'});try{const status=await(await api.get('/api/auth/status')).json();if(status.setupRequired){const token=(await readFile('.cache/e2e-setup-token','utf8')).trim();const response=await api.post('/api/auth/setup',{data:{token,username:'testadmin',password:'Only-for-e2e-tests-2026!'}});if(response.status()!==201)throw new Error('无法初始化临时测试账号：'+await response.text())}}finally{await api.dispose()}}
