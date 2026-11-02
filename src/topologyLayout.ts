export interface LayoutItem {id:string;role:string;layerId?:string;position?:{x:number;y:number}}
export interface LayoutGroup {id:string;items:LayoutItem[];layers?:{id:string;name:string}[];position?:{x:number;y:number}}
export const topologyLevels=['核心层','汇聚层','接入层','计算 / 存储','其他设备'];
export function topologyLevel(role:string){return ({core:0,aggregation:1,access:2,management:2,server:3,storage:3,infra:4} as Record<string,number>)[role]??4}
const valid=(p:LayoutItem['position'])=>!!p&&Number.isFinite(p.x)&&Number.isFinite(p.y)&&p.x>=0&&p.y>=0&&p.x<=10000&&p.y<=10000;
/** Pack only populated role bands. Saved positions stay local to their network. */
export function compactTopologyLayout(groups:LayoutGroup[],availableWidth:number,nodeWidth=184,nodeHeight=98,maxColumns=3){
 const gapX=24,gapY=36,padding=20,header=88;
 const usable=Math.max(nodeWidth+padding*2+40,Number.isFinite(availableWidth)?availableWidth:1000);
 const populated=groups.filter(g=>g.items.length);
 const lanes=populated.map(g=>{
  const maxPerRole=Math.max(1,...[0,1,2,3,4].map(r=>g.items.filter(n=>topologyLevel(n.role)===r).length));
  const columns=Math.max(1,Math.min(maxColumns,maxPerRole,Math.floor((usable-80)/(nodeWidth+gapX))));
  const local=new Map<string,{x:number;y:number}>();
  const occupied:{x:number;y:number}[]=[];
  for(const n of g.items)if(valid(n.position)){const p={x:n.position!.x,y:n.position!.y};let blocker;while((blocker=occupied.find(q=>p.x<q.x+nodeWidth+16&&p.x+nodeWidth+16>q.x&&p.y<q.y+nodeHeight+16&&p.y+nodeHeight+16>q.y))){p.y=blocker.y+nodeHeight+36}local.set(n.id,p);occupied.push(p)}
  const overlaps=(p:{x:number;y:number})=>occupied.some(q=>p.x<q.x+nodeWidth+8&&p.x+nodeWidth+8>q.x&&p.y<q.y+nodeHeight+8&&p.y+nodeHeight+8>q.y);
  let y=0;const bands:{id:string;name:string;y:number;top:number;height:number}[]=[];
  const layers=g.layers?.length?g.layers:topologyLevels.map((name,i)=>({id:String(i),name}));
  for(let role=0;role<layers.length;role++){
   const layer=layers[role],items=g.items.filter(n=>g.layers?.length?n.layerId===layer.id:topologyLevel(n.role)===role);
   if(!items.length&&!g.layers?.length)continue;
   const bandTop=y,automatic=items.filter(n=>!local.has(n.id));let slot=0,maxY=y;
   if(g.layers?.length)for(const item of items){const p=local.get(item.id);if(p&&p.y<bandTop){p.y=bandTop;let blocker;while((blocker=occupied.find(q=>q!==p&&p.x<q.x+nodeWidth+16&&p.x+nodeWidth+16>q.x&&p.y<q.y+nodeHeight+16&&p.y+nodeHeight+16>q.y))){p.y=blocker.y+nodeHeight+36}}}
   for(const item of automatic){let p:{x:number;y:number};do{p={x:slot%columns*(nodeWidth+gapX),y:y+Math.floor(slot/columns)*(nodeHeight+gapY)};slot++}while(overlaps(p));local.set(item.id,p);occupied.push(p);maxY=Math.max(maxY,p.y)}
   for(const item of items){const p=local.get(item.id);if(p)maxY=Math.max(maxY,p.y)}
   y=maxY+(items.length?nodeHeight+gapY:30);
   bands.push({id:layer.id,name:layer.name,y:bandTop-10,top:bandTop-24,height:y-bandTop});
  }
  const width=Math.max(nodeWidth+padding*2,...[...local.values()].map(p=>p.x+nodeWidth+padding*2));
  const height=Math.max(header+nodeHeight+padding, g.layers?.length?header+y:0,...[...local.values()].map(p=>p.y+header+nodeHeight+padding));
  return {id:g.id,x:0,y:0,nodeX:0,nodeY:0,width,height,bands,local};
 });
 let x=20,y=22,rowHeight=0;
 for(const lane of lanes){if(x>20&&x+lane.width+20>usable){x=20;y+=rowHeight+24;rowHeight=0}lane.x=x;lane.y=y;lane.nodeX=x+padding;lane.nodeY=y+header;x+=lane.width+24;rowHeight=Math.max(rowHeight,lane.height)}
 // Manual network locations override their own slot without moving neighbours.
 for(const lane of lanes){const saved=groups.find(g=>g.id===lane.id)?.position;if(valid(saved)){const dx=saved!.x-lane.x,dy=saved!.y-lane.y;lane.x+=dx;lane.y+=dy;lane.nodeX+=dx;lane.nodeY+=dy}}
 // Saved rectangles can collide after expansion or viewport changes. Resolve only
 // intersecting boxes, keep every already-separated box unchanged.
 const placed:typeof lanes=[];
 for(const lane of lanes){let obstacle;while((obstacle=placed.find(q=>lane.x<q.x+q.width+20&&lane.x+lane.width+20>q.x&&lane.y<q.y+q.height+20&&lane.y+lane.height+20>q.y))){const dy=obstacle.y+obstacle.height+40-lane.y;lane.y+=dy;lane.nodeY+=dy}placed.push(lane)}
 const nodes=new Map<string,{x:number;y:number;groupId:string}>();
 for(const lane of lanes)for(const [id,p] of lane.local)nodes.set(id,{x:lane.nodeX+p.x,y:lane.nodeY+p.y,groupId:lane.id});
 return {lanes,nodes,width:Math.max(280,...lanes.map(l=>l.x+l.width+20)),height:Math.max(180,...lanes.map(l=>l.y+l.height+22))};
}
