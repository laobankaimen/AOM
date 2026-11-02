import {build} from 'esbuild';import assert from 'node:assert/strict';import test from 'node:test';
const {outputFiles}=await build({stdin:{contents:"export * from './src/topologyRouting';export * from './src/topologyLayout';export * from './src/roomCamera';",resolveDir:process.cwd()},bundle:true,write:false,format:'esm',platform:'node'});
const {routeConnection,compactTopologyLayout,fillCamera}=await import('data:text/javascript;base64,'+Buffer.from(outputFiles[0].text).toString('base64'));
function hit(a,b,r){return a.x===b.x?a.x>r.x&&a.x<r.x+r.width&&Math.max(a.y,b.y)>r.y&&Math.min(a.y,b.y)<r.y+r.height:a.y>r.y&&a.y<r.y+r.height&&Math.max(a.x,b.x)>r.x&&Math.min(a.x,b.x)<r.x+r.width}
test('orthogonal connections avoid intervening rows and expose finite labels',()=>{
 const boxes=Array.from({length:24},(_,i)=>({id:String(i),x:20+i%6*234,y:80+Math.floor(i/6)*136,width:184,height:76}));
 for(const [from,to]of [[0,23],[0,12],[1,20],[4,17],[5,6],[0,5]]){const r=routeConnection(boxes[from],boxes[to],boxes,from);assert.equal(r.collisions,0);for(let i=1;i<r.points.length;i++){const a=r.points[i-1],b=r.points[i];assert.ok(a.x===b.x||a.y===b.y);for(const box of boxes){if(box.id===String(from)&&i===1||box.id===String(to)&&i===r.points.length-1)continue;assert.ok(!hit(a,b,box),`${from} -> ${to} crossed ${box.id}`)}}assert.ok(Number.isFinite(r.label.x)&&Number.isFinite(r.label.y))}
});
test('overlapping historical network and device coordinates are separated on display',()=>{
 const items=[{id:'a',role:'server',position:{x:20,y:20}},{id:'b',role:'server',position:{x:20,y:20}}];const groups=[{id:'n1',position:{x:200,y:100},items},{id:'n2',position:{x:250,y:120},items:[{id:'c',role:'server'}]}];const layout=compactTopologyLayout(groups,1200);const [a,b]=layout.lanes;assert.ok(a.x+a.width<=b.x||b.x+b.width<=a.x||a.y+a.height<=b.y||b.y+b.height<=a.y);const one=layout.nodes.get('a'),two=layout.nodes.get('b');assert.ok(one.y+98<=two.y);assert.equal(groups[1].position.y,120);
});
test('room camera fills wide and tall viewports without distorting units',()=>{
 for(const [w,h]of [[1800,600],[390,700],[900,900]]){const box={x:-400,y:30,width:600,height:500},camera=fillCamera(box,w,h,100);assert.ok(Math.abs(camera.width/camera.height-w/h)<1e-8);assert.ok(camera.x<=box.x&&camera.y<=box.y);assert.ok(camera.x+camera.width>=box.x+box.width-1e-8);const zoom=fillCamera(box,w,h,200);assert.equal(zoom.width,camera.width/2)}
});
