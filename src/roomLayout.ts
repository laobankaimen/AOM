import type {CanvasPoint,Module} from './model';
export const ISO_X=Math.sqrt(3)/2;
export function projectRoom(p:CanvasPoint,isometric=true,z=0):CanvasPoint{return isometric?{x:(p.x-p.y)*ISO_X,y:(p.x+p.y)*.5-z}:{x:p.x,y:p.y-z*.35}}
// DOMPoint coordinates are prototype accessors. Object spread would discard
// them, so projection boundaries always read x/y explicitly.
export function unprojectRoom(p:CanvasPoint,isometric=true):CanvasPoint{return isometric?{x:p.x/(2*ISO_X)+p.y,y:p.y-p.x/(2*ISO_X)}:{x:p.x,y:p.y}}
export function finiteRoomPoint(p:CanvasPoint|null|undefined):CanvasPoint|null{return p&&Number.isFinite(p.x)&&Number.isFinite(p.y)?{x:p.x,y:p.y}:null}
export interface RoomMatrix {a:number;b:number;c:number;d:number;e:number;f:number}
export function finiteRoomMatrix(matrix:RoomMatrix|null|undefined):boolean{return !!matrix&&[matrix.a,matrix.b,matrix.c,matrix.d,matrix.e,matrix.f].every(Number.isFinite)}
export function transformRoomPointer(point:CanvasPoint,matrix:RoomMatrix,scrollDelta:CanvasPoint={x:0,y:0}):CanvasPoint|null{
 if(!finiteRoomPoint(point)||!finiteRoomMatrix(matrix)||!finiteRoomPoint(scrollDelta))return null;
 const x=point.x+scrollDelta.x,y=point.y+scrollDelta.y;
 return finiteRoomPoint({x:matrix.a*x+matrix.c*y+matrix.e,y:matrix.b*x+matrix.d*y+matrix.f});
}
export function clampRoom(p:CanvasPoint):CanvasPoint{return {x:Math.max(0,Math.min(10000,p.x)),y:Math.max(0,Math.min(10000,p.y))}}
export function snapRoom(p:CanvasPoint):CanvasPoint{return clampRoom({x:Math.round(p.x/10)*10,y:Math.round(p.y/10)*10})}
export function roomMove(origin:CanvasPoint,start:CanvasPoint,end:CanvasPoint,isometric=true,snap=true):CanvasPoint{const a=unprojectRoom(start,isometric),b=unprojectRoom(end,isometric);const p={x:origin.x+b.x-a.x,y:origin.y+b.y-a.y};return snap?snapRoom(p):clampRoom(p)}
export function roomOverlaps(a:CanvasPoint,aw:number,ad:number,b:CanvasPoint,bw:number,bd:number){return a.x<b.x+bw&&a.x+aw>b.x&&a.y<b.y+bd&&a.y+ad>b.y}
export function moduleSize(width:number,depth:number,rotation=0){return rotation===90||rotation===270?{width:depth,depth:width}:{width,depth}}
export function rotateRoomRect(x:number,y:number,width:number,depth:number,moduleWidth:number,moduleDepth:number,rotation=0){
 switch(rotation){case 90:return {x:moduleDepth-y-depth,y:x,width:depth,depth:width};case 180:return {x:moduleWidth-x-width,y:moduleDepth-y-depth,width,depth};case 270:return {x:y,y:moduleWidth-x-width,width:depth,depth:width};default:return {x,y,width,depth}}
}
export function defaultRoomPositions(modules:Module[],widths:Map<string,number>):Map<string,CanvasPoint>{
 const size=(m:Module)=>moduleSize(widths.get(m.id)||250,m.type==='double'?180:100,m.roomRotation||0);
 const maxWidth=Math.max(250,...modules.map(m=>size(m).width)),maxDepth=Math.max(180,...modules.map(m=>size(m).depth));const result=new Map<string,CanvasPoint>();const occupied:{position:CanvasPoint;width:number;depth:number}[]=[];
 for(const m of modules)if(m.roomPosition){result.set(m.id,{...m.roomPosition});occupied.push({position:m.roomPosition,...size(m)})}
 let slot=0;for(const m of modules){if(result.has(m.id))continue;const {width,depth}=size(m);let p:CanvasPoint;
 do{p={x:60+(slot%2)*(maxWidth+150),y:60+Math.floor(slot/2)*(maxDepth+120)};slot++}while(occupied.some(o=>roomOverlaps(p,width+30,depth+30,o.position,o.width+30,o.depth+30))&&slot<10000);
 result.set(m.id,p);occupied.push({position:p,width,depth})
 }return result
}
export function cuboidFaces(x:number,y:number,w:number,d:number,h:number,isometric=true){const a=projectRoom({x,y},isometric,h),b=projectRoom({x:x+w,y},isometric,h),c=projectRoom({x:x+w,y:y+d},isometric,h),e=projectRoom({x,y:y+d},isometric,h),bc=projectRoom({x:x+w,y},isometric),cc=projectRoom({x:x+w,y:y+d},isometric),ec=projectRoom({x,y:y+d},isometric);const str=(pts:CanvasPoint[])=>pts.map(p=>`${p.x},${p.y}`).join(' ');return {top:str([a,b,c,e]),right:str([b,bc,cc,c]),front:str([e,c,cc,ec])}}
