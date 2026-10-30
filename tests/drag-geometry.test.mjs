import {readFileSync} from 'node:fs';import assert from 'node:assert/strict';import test from 'node:test';import ts from 'typescript';
const js=ts.transpileModule(readFileSync(new URL('../src/dragGeometry.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
const {rackUnitAt,rackDropUnit,reorderRackList}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
test('cross-cabinet target uses destination bounds and preserves grab offset at different scales',()=>{for(const scale of [.55,.9,1.2]){const rect={left:500,top:150,width:280*scale,height:934*scale};const x=rect.left+140*scale,y=rect.top+(36+32*20+10)*scale;assert.equal(rackUnitAt(y,rect),10);assert.equal(rackDropUnit(x,y,rect,1),9);assert.equal(rackDropUnit(rect.left-1,y,rect),null);assert.equal(rackDropUnit(x,rect.top+20*scale,rect),null);assert.equal(rackDropUnit(x,rect.top+900*scale,rect),null)}});
test('reorder inserts instead of renaming or duplicating racks, and supports cross-row moves',()=>{const racks=[{id:'a',side:'L',index:1},{id:'b',side:'L',index:2},{id:'c',side:'L',index:3},{id:'d',side:'R',index:1}];assert.deepEqual(reorderRackList(racks,'a','L',null),{L:['b','c','a'],R:['d']});assert.deepEqual(reorderRackList(racks,'b','L','a'),{L:['b','a','c'],R:['d']});assert.deepEqual(reorderRackList(racks,'a','R','d'),{L:['b','c'],R:['a','d']});assert.deepEqual(reorderRackList(racks,'b','L','b'),{L:['a','b','c'],R:['d']});assert.equal(reorderRackList(racks,'a','L','missing'),null);assert.equal(reorderRackList(racks,'missing','L',null),null);assert.equal(racks[0].index,1)});
const {cabinetDropAt,cabinetOrderError,insertLayoutItem}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
test('cabinet insertion line resolves every mixed-row boundary, including infrastructure and row ends',()=>{
 const rack=(id,index,x)=>({id,x,width:108,rack:{id,side:'L',index}});
 const tiles=[{id:'power',x:0,width:76,infra:{}},rack('a',1,88),{id:'ac',x:208,width:76,infra:{}},rack('b',2,296),rack('c',3,416),{id:'env',x:536,width:76,infra:{}}];
 assert.deepEqual(cabinetDropAt(tiles,'L',0,'c'),{side:'L',beforeId:'power',markerX:-6,index:1});
 assert.deepEqual(cabinetDropAt(tiles,'L',250,'c'),{side:'L',beforeId:'b',markerX:290,index:2});
 assert.deepEqual(cabinetDropAt(tiles,'L',600,'a'),{side:'L',beforeId:null,markerX:618,index:3});
 assert.deepEqual(cabinetDropAt(tiles,'L',250,'b'),{side:'L',beforeId:'c',markerX:410,index:2});
 assert.deepEqual(cabinetDropAt(tiles,'L',600,'other-row-rack'),{side:'L',beforeId:null,markerX:618,index:4});
 assert.deepEqual(cabinetDropAt(tiles,'L',220,'c'),{side:'L',beforeId:'ac',markerX:202,index:2});
 assert.deepEqual(cabinetDropAt(tiles,'L',550,'a'),{side:'L',beforeId:'env',markerX:530,index:3});
 assert.equal(cabinetDropAt(tiles,'L',NaN,'a'),null);
 assert.equal(cabinetDropAt(tiles,'R',Infinity,'a'),null);
 assert.equal(cabinetDropAt(tiles,'invalid',100,'a'),null);
});
test('cabinet cross-row constraints preserve at least one rack and respect row capacities',()=>{
 assert.equal(cabinetOrderError({L:['a'],R:['b']},true),'');
 assert.match(cabinetOrderError({L:[],R:['a','b']},true),/至少/);
 assert.match(cabinetOrderError({L:['a'],R:Array(13).fill('r')},true),/最多/);
 assert.equal(cabinetOrderError({L:Array(24).fill('r'),R:[]},false),'');
 assert.match(cabinetOrderError({L:['a'],R:['b']},false),/只能/);
});

test('mixed insertion preserves every other rack and facility in order, including same rack index moves',()=>{
 const rack=id=>({id,rack:{id}}),infra=id=>({id,infra:{id}});
 const rows=[{side:'L',tiles:[rack('a'),infra('power'),infra('ac'),rack('b'),rack('c')]},{side:'R',tiles:[rack('r'),infra('ups')]}];
 assert.deepEqual(insertLayoutItem(rows,'c','L','power'),{L:['a','c','power','ac','b'],R:['r','ups']});
 assert.deepEqual(insertLayoutItem(rows,'b','L','ac'),{L:['a','power','b','ac','c'],R:['r','ups']});
 assert.deepEqual(insertLayoutItem(rows,'c','R','ups'),{L:['a','power','ac','b'],R:['r','c','ups']});
 assert.deepEqual(insertLayoutItem(rows,'a','L','power'),{L:['a','power','ac','b','c'],R:['r','ups']});
 assert.equal(insertLayoutItem(rows,'a','R','power'),null);
 assert.deepEqual(rows[0].tiles.map(t=>t.id),['a','power','ac','b','c']);
});
