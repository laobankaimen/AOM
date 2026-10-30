import type {LayoutTile} from './layout';
export interface Rect { left:number; top:number; width:number; height:number }
export function rackUnitAt(clientY:number,rect:Rect){return Math.max(1,Math.min(42,43-Math.ceil(((clientY-rect.top)/rect.height*934-36)/20)))}
export function rackDropUnit(clientX:number,clientY:number,rect:Rect,offset=0):number|null{
 if(rect.width<=0||rect.height<=0)return null;
 const x=(clientX-rect.left)/rect.width*280,y=(clientY-rect.top)/rect.height*934;
 if(x<26||x>266||y<36||y>876)return null;
 return rackUnitAt(clientY,rect)-offset;
}
export interface ReorderRack {id:string;side:string;index:number}
export function reorderRackList(racks:ReorderRack[],id:string,targetSide:string,beforeId:string|null){
 const rows={L:racks.filter(r=>r.side==='L').sort((a,b)=>a.index-b.index).map(r=>r.id),R:racks.filter(r=>r.side==='R').sort((a,b)=>a.index-b.index).map(r=>r.id)};
 if(!racks.some(r=>r.id===id)||!['L','R'].includes(targetSide))return null;
 if(beforeId===id)return rows;
 rows.L=rows.L.filter(v=>v!==id);rows.R=rows.R.filter(v=>v!==id);
 const target=rows[targetSide as 'L'|'R'];const index=beforeId?target.indexOf(beforeId):target.length;
 if(index<0)return null;target.splice(index,0,id);return rows;
}

export interface CabinetDrop {side:string;beforeId:string|null;markerX:number;index:number}
// Every displayed tile is a possible insertion boundary, including facilities.
export function cabinetDropAt(tiles:LayoutTile[],side:string,x:number,movingId:string):CabinetDrop|null {
 if(!Number.isFinite(x)||!['L','R'].includes(side))return null;
 const remaining=tiles.filter(tile=>tile.id!==movingId);
 const index=remaining.findIndex(tile=>x<tile.x+tile.width/2),slot=index<0?remaining.length:index;
 const before=remaining[slot],last=remaining.at(-1);
 return {side,beforeId:before?.id||null,index:remaining.slice(0,slot).filter(t=>t.rack).length+1,
  markerX:before?before.x-6:last?last.x+last.width+6:0};
}
export function insertLayoutItem(rows:{side:string;tiles:LayoutTile[]}[],id:string,side:string,beforeId:string|null){
 const result={L:rows.find(r=>r.side==='L')?.tiles.map(t=>t.id)||[],R:rows.find(r=>r.side==='R')?.tiles.map(t=>t.id)||[]};
 if(!['L','R'].includes(side)||!rows.some(r=>r.tiles.some(t=>t.id===id&&t.rack)))return null;
 const target=side as 'L'|'R';if(beforeId&&!result[target].includes(beforeId))return null;
 if(beforeId===id)return result;
 result.L=result.L.filter(v=>v!==id);result.R=result.R.filter(v=>v!==id);
 const at=beforeId?result[target].indexOf(beforeId):result[target].length;result[target].splice(at,0,id);return result;
}
export function cabinetOrderError(rows:{L:string[];R:string[]},double:boolean):string {
 if(double)return rows.L.length<1||rows.R.length<1?'每列至少保留1柜':rows.L.length>12||rows.R.length>12?'每列最多12柜':'';
 return rows.R.length?'单排只能使用L列':rows.L.length<1||rows.L.length>24?'单排须保留1–24柜':'';
}
