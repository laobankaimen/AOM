import test from 'node:test';import assert from 'node:assert/strict';import {build} from 'esbuild';
const compiled=await build({stdin:{contents:"export * from './src/deviceRoles';export * from './src/networkLayers';export * from './src/topologyLayout';export * from './src/topologyProjection';",resolveDir:process.cwd()},bundle:true,write:false,format:'esm',platform:'node'});
const {syncDeviceRoles,roleNames,roleColors,roleBase,layerOf,topologyLevel,projectTopology}=await import('data:text/javascript;base64,'+Buffer.from(compiled.outputFiles[0].text).toString('base64'));
test('custom roles rename labels and colors without losing topology classification or grouping safeguards',()=>{
 syncDeviceRoles([{id:'server',name:'应用节点',color:'#123456',baseRole:'server'},{id:'custom-core',name:'园区核心',color:'#654321',baseRole:'core'}]);
 assert.equal(roleNames.server,'应用节点');assert.equal(roleColors['custom-core'],'#654321');assert.equal(roleBase('custom-core'),'core');assert.equal(topologyLevel('custom-core'),0);assert.equal(layerOf(undefined,'d','custom-core'),'core');
 const model={id:'m',name:'core',role:'custom-core'},devices=['a','b'].map(id=>({id,name:id,modelId:'m'}));const result=projectTopology(devices,[model],[],[],[],true,new Set());assert.equal(result.nodes.length,2);
 syncDeviceRoles();assert.equal(roleNames['custom-core'],undefined);assert.equal(roleNames.server,'计算服务器');
});
