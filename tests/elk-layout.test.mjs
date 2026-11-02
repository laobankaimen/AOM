import {build} from 'esbuild';import test from 'node:test';import assert from 'node:assert/strict';
import {mkdirSync} from 'node:fs';import {pathToFileURL} from 'node:url';import path from 'node:path';
mkdirSync('.cache',{recursive:true});const output=path.resolve('.cache/elk-layout-test.mjs');await build({entryPoints:['src/elkLayout.ts'],bundle:true,outfile:output,format:'esm',platform:'node',packages:'external'});
const {arrangeWithELK}=await import(pathToFileURL(output).href);
test('ELK arrangement preserves custom layers and returns finite non-overlapping network positions',async()=>{
 const devices=Array.from({length:20},(_,i)=>({id:'d'+i,name:'Device '+i,modelId:i<2?'switch':'server'}));const networks=[{id:'n1',deviceIds:devices.slice(0,12).map(d=>d.id),layers:[{id:'a',name:'边界'},{id:'b',name:'业务'}],deviceLayers:{d0:'a',d1:'a'}},{id:'n2',deviceIds:devices.slice(12).map(d=>d.id)}];const state={models:[{id:'switch',role:'core'},{id:'server',role:'server'}],links:devices.slice(2).map((d,i)=>({id:'l'+i,sourceId:'d0',targetId:d.id}))};
 const groups=await arrangeWithELK(state,devices,networks);assert.equal(groups.length,2);assert.equal(groups.reduce((n,g)=>n+g.positions.length,0),20);assert.ok(groups[1].position.x>groups[0].position.x);const points=groups[0].positions;assert.ok(points.find(p=>p.deviceId==='d0').position.y<points.find(p=>p.deviceId==='d2').position.y);for(const g of groups)for(const p of g.positions)assert.ok(Number.isFinite(p.position.x)&&p.position.x>=0&&p.position.y>=0);
});
