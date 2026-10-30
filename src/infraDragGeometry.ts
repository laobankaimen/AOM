import type {LayoutTile} from './layout';
export interface InfraDrop {side:string;afterRack:number;beforeId:string;markerX:number}
export function infrastructureDropAt(tiles:LayoutTile[],side:string,x:number,movingId:string,rackCount:number):InfraDrop {
 const remaining=tiles.filter(t=>t.id!==movingId);
 const before=remaining.find(t=>x<t.x+t.width/2);
 if(!before){const last=remaining.at(-1);return {side,afterRack:rackCount,beforeId:'',markerX:last?last.x+last.width+6:0}}
 return {side,afterRack:before.rack?before.rack.index-1:before.infra!.afterRack,beforeId:before.infra?.id||'',markerX:before.x-6};
}
