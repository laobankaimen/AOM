import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
const {outputFiles}=await build({entryPoints:['src/devicePlacement.ts'],bundle:true,write:false,format:'esm',platform:'node'});
const {roomRacks,freeRackUnits}=await import('data:text/javascript;base64,'+Buffer.from(outputFiles[0].text).toString('base64'));

test('rack choices share the room, sort by module/row/index and exclude other rooms',()=>{
 const state={modules:[{id:'a',siteId:'room'},{id:'b',siteId:'room'},{id:'other',siteId:'other-room'}],racks:[{id:'b2',moduleId:'b',side:'L',index:2},{id:'outside',moduleId:'other',side:'L',index:1},{id:'b1',moduleId:'b',side:'L',index:1},{id:'a1',moduleId:'a',side:'L',index:1}]};
 assert.deepEqual(roomRacks(state,'a').map(r=>r.id),['a1','b1','b2']);assert.deepEqual(roomRacks(state,'b'),roomRacks(state,'a'));assert.deepEqual(roomRacks(state,'missing'),[]);
});
test('placement finds contiguous free units in the selected rack, excluding the moved device',()=>{
 const device={id:'mover',modelId:'two',rackId:'r',startU:1},state={models:[{id:'two',uHeight:2},{id:'one',uHeight:1}],racks:[{id:'r',capacity:8}],devices:[device,{id:'blocker',modelId:'one',rackId:'r',startU:4},{id:'other-rack',modelId:'two',rackId:'elsewhere',startU:6}]};
 assert.deepEqual(freeRackUnits(state,device,'r'),[1,2,5,6,7]);assert.deepEqual(freeRackUnits(state,{...device,modelId:'unconfirmed'},'r'),[]);assert.deepEqual(freeRackUnits(state,device,'missing'),[]);
 state.devices.push({id:'full',modelId:'all',rackId:'r',startU:1});state.models.push({id:'all',uHeight:8});assert.deepEqual(freeRackUnits(state,device,'r'),[]);
});
