import {readFileSync} from 'node:fs';import assert from 'node:assert/strict';import test from 'node:test';import ts from 'typescript';
import {build} from 'esbuild';
const compiled=await build({entryPoints:['src/topologyProjection.ts'],bundle:true,write:false,format:'esm',platform:'node'});const js=compiled.outputFiles[0].text;
const {projectTopology}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
test('same-model devices merge visually, expansion retains every real device and connection',()=>{
 const models=[{id:'sw',role:'access',name:'接入交换机'},{id:'srv',role:'server',name:'计算服务器'}];
 const devices=[{id:'a',modelId:'sw',name:'A'},{id:'b',modelId:'sw',name:'B'},...Array.from({length:66},(_,i)=>({id:'s'+i,modelId:'srv',name:'服务器'+i}))];
 const networks=[{id:'data',deviceIds:devices.map(d=>d.id)}],links=devices.slice(2).flatMap(d=>['a','b'].map(from=>({id:from+d.id,sourceId:from,targetId:d.id,kind:'logical',status:'pending'})));
 const result=projectTopology(devices,models,networks,networks,links,true,new Set());
 assert.equal(result.nodes.length,2);assert.equal(result.nodes.find(g=>g.role==='server').deviceIds.length,66);assert.equal(result.nodes.find(g=>g.role==='access').deviceIds.length,2);assert.equal(result.edges.length,1);assert.equal(result.edges[0].links.length,132);
 const expanded=projectTopology(devices,models,networks,networks,links,true,new Set([result.nodes.find(g=>g.role==='server').groupKey]));assert.equal(expanded.nodes.length,67);assert.equal(expanded.edges.reduce((n,e)=>n+e.links.length,0),132);assert.equal(expanded.nodes.flatMap(n=>n.deviceIds).length,68);
 const all=projectTopology(devices,models,networks,networks,links,false,new Set());assert.equal(all.nodes.length,68);assert.equal(all.edges.length,132);
});
test('hidden other-network membership prevents false grouping in a filtered view',()=>{
 const models=[{id:'m',role:'server',name:'Server'}],devices=[{id:'1',modelId:'m',name:'one'},{id:'2',modelId:'m',name:'two'}];
 const networks=[{id:'business',deviceIds:['1','2']},{id:'management',deviceIds:['2']}];
 const filtered=projectTopology(devices,models,[networks[0]],networks,[],true,new Set());assert.equal(filtered.nodes.length,2);assert.deepEqual(filtered.nodes.find(n=>n.id==='2').networkIds,['business','management']);
});
test('collapsed groups preserve real physical/logical links and internal connection counts',()=>{
 const devices=[{id:'1',modelId:'m',name:'one'},{id:'2',modelId:'m',name:'two'},{id:'3',modelId:'n',name:'switch'}],models=[{id:'m',role:'server',name:'server'},{id:'n',role:'access',name:'switch'}];
 const links=[{id:'a',sourceId:'1',targetId:'2',kind:'physical',status:'confirmed'},{id:'b',sourceId:'1',targetId:'3',kind:'logical',status:'pending'},{id:'c',sourceId:'2',targetId:'3',kind:'physical',status:'confirmed'},{id:'d',sourceId:'1',targetId:'outside',kind:'logical',status:'pending'}];
 const result=projectTopology(devices,models,[],[],links,true,new Set());assert.equal(result.nodes.length,2);const group=result.nodes.find(n=>n.grouped);assert.equal(result.internal.get(group.id),1);assert.deepEqual(result.edges[0].links.map(l=>l.id),['b','c']);assert.equal(result.nodes.flatMap(n=>n.deviceIds).length,3);
});
test('same-model devices assigned to different display layers do not merge',()=>{
 const devices=[{id:'a',name:'A',modelId:'m'},{id:'b',name:'B',modelId:'m'}],models=[{id:'m',name:'Server',role:'server'}],network={id:'n',deviceIds:['a','b'],layers:[{id:'edge',name:'边界层'},{id:'app',name:'应用层'}],deviceLayers:{a:'edge',b:'app'}};
 const result=projectTopology(devices,models,[network],[network],[],true,new Set());assert.equal(result.nodes.length,2);assert.equal(result.nodes.find(n=>n.id==='a').layerId,'edge');assert.equal(result.nodes.find(n=>n.id==='b').layerId,'app');
 const fallback=projectTopology(devices,models,[{...network,deviceLayers:{}}],[network],[],true,new Set());assert.equal(fallback.nodes.length,1);assert.equal(fallback.nodes[0].layerId,'app');
});
