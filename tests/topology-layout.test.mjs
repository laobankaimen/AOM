import {readFileSync} from 'node:fs';import test from 'node:test';import assert from 'node:assert/strict';import ts from 'typescript';
import {build} from 'esbuild';
const compiled=await build({entryPoints:['src/topologyLayout.ts'],bundle:true,write:false,format:'esm',platform:'node'});const js=compiled.outputFiles[0].text;
const {compactTopologyLayout:layout}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
const items=(count,prefix='s',role='server')=>Array.from({length:count},(_,i)=>({id:prefix+i,role}));
function noOverlap(result,w=184,h=98){const nodes=[...result.nodes.values()];for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++){const a=nodes[i],b=nodes[j];assert.ok(a.x+w<=b.x||b.x+w<=a.x||a.y+h<=b.y||b.y+h<=a.y,`overlap ${i},${j}`)}}
test('server-only networks start at their header, empty parents reserve no lane',()=>{
 const r=layout([{id:'empty',items:[]},{id:'data',items:items(3)}],1100);
 assert.equal(r.lanes.length,1);assert.equal(r.nodes.get('s0').y,r.lanes[0].nodeY);assert.ok(r.height<300);assert.equal(r.lanes[0].bands.length,1);assert.equal(r.nodes.size,3);noOverlap(r);
});
test('multiple networks and large device sets wrap without a giant horizontal row',()=>{
 for(const width of [360,800,1400]){
  const r=layout([{id:'a',items:items(19)},{id:'b',items:items(3,'b')},{id:'c',items:items(2,'c','access')}],width);
  assert.ok(r.width<=width);assert.equal(r.nodes.size,24);assert.ok(r.lanes[0].width<=640);noOverlap(r);
  for(const n of r.nodes.values()){const lane=r.lanes.find(l=>l.id===n.groupId);assert.ok(n.x>=lane.nodeX&&n.y>=lane.nodeY);assert.ok(n.x+184<=lane.x+lane.width&&n.y+98<=lane.y+lane.height)}
 }
});
test('saved positions remain stable and auto nodes avoid them; clearing saved coordinates compacts again',()=>{
 const group={id:'n',items:[{id:'saved',role:'server',position:{x:0,y:0}},...items(5),{id:'far',role:'access',position:{x:2400,y:1100}}]};
 const r=layout([group],900);assert.equal(r.nodes.get('far').x-r.lanes[0].nodeX,2400);assert.equal(r.nodes.get('far').y-r.lanes[0].nodeY,1100);noOverlap(r);
 const compact=layout([{...group,items:group.items.map(({position,...n})=>n)}],900);assert.ok(compact.width<900);assert.ok(compact.height<700);noOverlap(compact);
 assert.equal(group.items[0].position.x,0);
});
test('invalid coordinates fall back to automatic finite geometry and summary node sizes are supported',()=>{
 const r=layout([{id:'n',items:[{id:'bad',role:'core',position:{x:NaN,y:3}},...items(6)]}],800,160,76);
 for(const n of r.nodes.values())assert.ok(Number.isFinite(n.x)&&Number.isFinite(n.y));noOverlap(r,160,76);
});
test('network positions move the entire group without changing internal offsets or other lanes',()=>{
 const groups=[{id:'a',items:items(3)},{id:'b',items:items(2,'b')}];const initial=layout(groups,1200);
 const positioned=layout([{...groups[0],position:{x:620,y:300}},groups[1]],1200);
 const before=initial.lanes.find(l=>l.id==='a'),after=positioned.lanes.find(l=>l.id==='a');assert.equal(after.x,620);assert.equal(after.y,300);
 for(const item of groups[0].items){const a=initial.nodes.get(item.id),b=positioned.nodes.get(item.id);assert.equal(b.x-after.nodeX,a.x-before.nodeX);assert.equal(b.y-after.nodeY,a.y-before.nodeY)}
 assert.deepEqual(initial.lanes.find(l=>l.id==='b'),positioned.lanes.find(l=>l.id==='b'));
 assert.ok(positioned.width>=after.x+after.width);assert.ok(positioned.height>=after.y+after.height);
 const invalid=layout([{...groups[0],position:{x:NaN,y:3}},groups[1]],1200);assert.deepEqual(initial,invalid);
});
test('custom layers have ordered drop areas, including empty layers, and override model roles',()=>{
 const r=layout([{id:'n',layers:[{id:'edge',name:'边界层'},{id:'app',name:'应用层'},{id:'empty',name:'预留层'}],items:[{id:'a',role:'server',layerId:'edge'},{id:'b',role:'core',layerId:'app'}]}],900);
 assert.deepEqual(r.lanes[0].bands.map(b=>b.id),['edge','app','empty']);assert.ok(r.nodes.get('a').y<r.nodes.get('b').y);for(const b of r.lanes[0].bands)assert.ok(b.height>0);noOverlap(r);
});
test('saved device coordinates cannot lift a lower-layer device into a higher layer',()=>{
 const r=layout([{id:'n',layers:[{id:'core',name:'核心'},{id:'compute',name:'计算'}],items:[{id:'core',role:'core',layerId:'core',position:{x:0,y:0}},{id:'server',role:'server',layerId:'compute',position:{x:240,y:0}}]}],900);
 const lane=r.lanes[0],band=lane.bands.find(b=>b.id==='compute');assert.ok(r.nodes.get('server').y>=lane.nodeY+band.top+24);assert.ok(r.nodes.get('server').y>r.nodes.get('core').y);noOverlap(r);
});

test('a real empty network keeps a compact actionable lane without a device-sized blank area',()=>{
 const r=layout([{id:'empty',preserveEmpty:true,items:[]}],1200,184,76);assert.equal(r.lanes.length,1);assert.equal(r.lanes[0].height,130);assert.equal(r.lanes[0].bands.length,0);assert.equal(r.nodes.size,0);
});
