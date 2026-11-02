import type {State,NetworkSegment,Device} from './model';
import {layersOf,layerOf} from './networkLayers';
/** ELK's layered example supplies crossing minimization; semantic display layers
 * remain authoritative. Network and layer rows are deliberately kept separate. */
export async function arrangeWithELK(state:State,devices:Device[],networks:NetworkSegment[]){
 if(devices.length>1000)throw new Error('一次自动排列最多1000台设备，请先按网络或机房缩小范围');
 const {default:ELK}=await import('elkjs/lib/elk.bundled.js');const elk=new ELK();
 const ids=new Set(devices.map(d=>d.id));
 const graph=await elk.layout({id:'root',layoutOptions:{'elk.algorithm':'layered','elk.direction':'DOWN','elk.edgeRouting':'ORTHOGONAL','elk.spacing.nodeNode':'50','elk.layered.spacing.nodeNodeBetweenLayers':'80'},children:devices.map(d=>({id:d.id,width:184,height:76})),edges:state.links.filter(l=>ids.has(l.sourceId)&&ids.has(l.targetId)).map(l=>({id:l.id,sources:[l.sourceId],targets:[l.targetId]}))});
 const order=new Map((graph.children||[]).map(n=>[n.id,{x:n.x||0,y:n.y||0}]));
 const owners=new Map(devices.map(d=>[d.id,networks.find(n=>n.deviceIds.includes(d.id))?.id||'']));
 const result:{networkId:string;position:{x:number;y:number}|null;positions:{deviceId:string;position:{x:number;y:number}}[]}[]=[];
 let networkX=20;
 for(const networkId of [...networks.map(n=>n.id),'']){
  const members=devices.filter(d=>owners.get(d.id)===networkId);if(!members.length)continue;const network=networks.find(n=>n.id===networkId);
  let y=0,maxWidth=224;const positions:typeof result[number]['positions']=[];
  for(const layer of layersOf(network)){
   const row=members.filter(d=>layerOf(network,d.id,state.models.find(m=>m.id===d.modelId)?.role||'server')===layer.id).sort((a,b)=>(order.get(a.id)?.x||0)-(order.get(b.id)?.x||0)||a.name.localeCompare(b.name));
   if(!row.length)continue;
   const columns=Math.max(5,Math.ceil(row.length/60));
   row.forEach((d,i)=>positions.push({deviceId:d.id,position:{x:i%columns*234,y:y+Math.floor(i/columns)*136}}));
   y+=Math.ceil(row.length/columns)*136+40;maxWidth=Math.max(maxWidth,Math.min(row.length,columns)*234+20);
  }
  if(networkX>10000||positions.some(p=>p.position.x>10000||p.position.y>10000))throw new Error('布局范围过大，请缩小网络范围后排列');
  result.push({networkId,position:networkId?{x:networkX,y:22}:null,positions});networkX+=maxWidth+80;
 }
 return result;
}
