import {readFileSync} from 'node:fs';import test from 'node:test';import assert from 'node:assert/strict';import ts from 'typescript';
async function load(name){const code=ts.transpileModule(readFileSync(new URL('../src/'+name,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;return import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'))}
const {projectRoom,unprojectRoom,roomMove,defaultRoomPositions,roomOverlaps,cuboidFaces}=await load('roomLayout.ts');const {infrastructureDropAt}=await load('infraDragGeometry.ts');
test('isometric and flat projection round trip, drag delta and snap',()=>{for(const iso of [true,false])for(const p of [{x:0,y:0},{x:200,y:400},{x:10000,y:9000}]){const result=unprojectRoom(projectRoom(p,iso),iso);assert.ok(Math.abs(result.x-p.x)<.00001);assert.ok(Math.abs(result.y-p.y)<.00001)}const start=projectRoom({x:200,y:300}),end=projectRoom({x:270,y:330});assert.deepEqual(roomMove({x:50,y:60},start,end),{x:120,y:90});assert.deepEqual(roomMove({x:0,y:0},{x:0,y:0},{x:-200,y:-200},false),{x:0,y:0});for(const face of Object.values(cuboidFaces(100,200,80,40,40))){assert.ok(!face.includes('NaN'));assert.equal(face.split(' ').length,4)}});
test('automatic room arrangement avoids saved modules and retains saved coordinates',()=>{const modules=[{id:'a',type:'single',roomPosition:{x:60,y:60}},{id:'b',type:'double'},{id:'c',type:'single'}],widths=new Map([['a',200],['b',250],['c',200]]);const p=defaultRoomPositions(modules,widths);assert.deepEqual(p.get('a'),modules[0].roomPosition);for(let i=0;i<modules.length;i++)for(let j=i+1;j<modules.length;j++){const a=modules[i],b=modules[j];assert.equal(roomOverlaps(p.get(a.id),widths.get(a.id),a.type==='double'?180:100,p.get(b.id),widths.get(b.id),b.type==='double'?180:100),false)}});
test('inline infrastructure resolves rack gaps and ordering without changing type',()=>{const tiles=[{id:'ac',x:0,width:76,infra:{id:'ac',afterRack:0}},{id:'r1',x:88,width:108,rack:{index:1}},{id:'power',x:208,width:76,infra:{id:'power',afterRack:1}},{id:'r2',x:296,width:108,rack:{index:2}}];assert.deepEqual(infrastructureDropAt(tiles,'L',205,'ac',2),{side:'L',afterRack:1,beforeId:'power',markerX:202});assert.deepEqual(infrastructureDropAt(tiles,'R',500,'ac',2),{side:'R',afterRack:2,beforeId:'',markerX:410});assert.deepEqual(infrastructureDropAt(tiles,'L',0,'power',2),{side:'L',afterRack:0,beforeId:'ac',markerX:-6});assert.equal(infrastructureDropAt(tiles,'L',290,'ac',2).afterRack,1)});

test('unsnapped dragging follows fractional movements in both views',()=>{for(const iso of [true,false]){const origin={x:100,y:200},start=projectRoom({x:220,y:320},iso),end=projectRoom({x:222.4,y:321.8},iso);const p=roomMove(origin,start,end,iso,false);assert.ok(Math.abs(p.x-102.4)<1e-8);assert.ok(Math.abs(p.y-201.8)<1e-8);const snapped=roomMove(origin,start,end,iso,true);assert.deepEqual(snapped,{x:100,y:200})}});
test('rotation changes footprints and keeps every cabinet within the module',async()=>{const {moduleSize,rotateRoomRect}=await load('roomLayout.ts');const w=300,d=180;for(const angle of [0,90,180,270]){const size=moduleSize(w,d,angle),rect=rotateRoomRect(20,30,70,42,w,d,angle);assert.ok(rect.x>=0&&rect.y>=0);assert.ok(rect.x+rect.width<=size.width);assert.ok(rect.y+rect.depth<=size.depth);if(angle===90){assert.deepEqual(size,{width:180,depth:300});assert.deepEqual(rect,{x:108,y:20,width:42,depth:70})}}});
test('automatic layout respects rotated footprint and saved position',()=>{const modules=[{id:'rotated',type:'single',roomRotation:90,roomPosition:{x:60,y:60}},{id:'next',type:'single'}],widths=new Map([['rotated',800],['next',200]]);const p=defaultRoomPositions(modules,widths);assert.deepEqual(p.get('rotated'),{x:60,y:60});assert.equal(roomOverlaps(p.get('rotated'),100,800,p.get('next'),200,100),false)});

test('browser DOMPoint-like inherited x/y work in flat and isometric dragging',async()=>{
 // Match DOMPoint: x/y live on the prototype; spreading an instance is empty.
 class BrowserPoint {#x;#y;constructor(x,y){this.#x=x;this.#y=y}get x(){return this.#x}get y(){return this.#y}}
 const {unprojectRoom,finiteRoomPoint}=await load('roomLayout.ts');
 const point=new BrowserPoint(37.25,84.5);assert.deepEqual({...point},{});
 assert.deepEqual(unprojectRoom(point,false),{x:37.25,y:84.5});
 assert.deepEqual(finiteRoomPoint(point),{x:37.25,y:84.5});
 for(const iso of [false,true]){
  const start=new BrowserPoint(40,70),end=new BrowserPoint(47,76);
  const p=roomMove({x:100,y:200},start,end,iso,false);
  assert.ok(Number.isFinite(p.x)&&Number.isFinite(p.y));
  const visualStart=projectRoom({x:100,y:200},iso),visualEnd=projectRoom(p,iso);
  assert.ok(Math.abs(visualEnd.x-visualStart.x-7)<1e-9);
  assert.ok(Math.abs(visualEnd.y-visualStart.y-6)<1e-9);
 }
});
test('pointer conversion rejects invalid matrices and preserves subpixel travel at every zoom',async()=>{
 const {transformRoomPointer,finiteRoomPoint,finiteRoomMatrix}=await load('roomLayout.ts');
 for(const scale of [.5,.75,1,1.5,2]){
  const inverse={a:1/scale,b:0,c:0,d:1/scale,e:-30/scale,f:-80/scale};
  const start=transformRoomPointer({x:150,y:230},inverse),end=transformRoomPointer({x:150.25,y:230.5},inverse);
  assert.ok(Math.abs(end.x-start.x-.25/scale)<1e-9);
  assert.ok(Math.abs(end.y-start.y-.5/scale)<1e-9);
  const scrolled=transformRoomPointer({x:150,y:230},inverse,{x:20,y:40});
  assert.ok(Math.abs(scrolled.x-start.x-20/scale)<1e-9);assert.ok(Math.abs(scrolled.y-start.y-40/scale)<1e-9);
 }
 assert.equal(finiteRoomPoint({x:NaN,y:1}),null);assert.equal(finiteRoomPoint({x:1,y:Infinity}),null);
 assert.equal(finiteRoomMatrix({a:NaN,b:0,c:0,d:1,e:0,f:0}),false);
 assert.equal(transformRoomPointer({x:1,y:2},{a:1,b:0,c:0,d:NaN,e:0,f:0}),null);
 assert.equal(transformRoomPointer({x:Infinity,y:2},{a:1,b:0,c:0,d:1,e:0,f:0}),null);
});
