import {layerOf} from './networkLayers';
import type {Device,Model,NetworkSegment,DeviceLink,CanvasPoint} from './model';
export interface TopologyNode {id:string;name:string;role:string;networkId:string;networkIds:string[];deviceIds:string[];groupKey:string;grouped:boolean;layerId:string;device?:Device;position?:CanvasPoint}
export function projectTopology(devices:Device[],models:Model[],networks:NetworkSegment[],allNetworks:NetworkSegment[],links:DeviceLink[],merge:boolean,expanded:ReadonlySet<string>){
 const buckets=new Map<string,{key:string;name:string;role:string;networkId:string;networkIds:string[];layerId:string;devices:Device[]}>();
 for(const d of devices){
  const model=models.find(m=>m.id===d.modelId),memberships=allNetworks.filter(n=>n.deviceIds.includes(d.id)).map(n=>n.id).sort();
  const networkId=networks.find(n=>n.deviceIds.includes(d.id))?.id||'';
  const layerId=layerOf(networks.find(n=>n.id===networkId),d.id,model?.role||'server');
  const key=JSON.stringify([d.modelId,memberships,networkId,layerId]);
  if(!buckets.has(key))buckets.set(key,{key,name:model?.name||d.name,role:model?.role||'server',networkId,networkIds:memberships,layerId,devices:[]});
  buckets.get(key)!.devices.push(d);
 }
 const nodes:TopologyNode[]=[],owner=new Map<string,string>();
 for(const bucket of buckets.values()){
  const saved=(d:Device)=>bucket.networkId?networks.find(n=>n.id===bucket.networkId)?.devicePositions?.[d.id]:d.topologyPosition;
  if(merge&&bucket.devices.length>1&&!expanded.has(bucket.key)){
   const positions=bucket.devices.map(saved).filter((p):p is CanvasPoint=>!!p),id='group:'+bucket.key;
   nodes.push({id,name:bucket.name,role:bucket.role,networkId:bucket.networkId,networkIds:bucket.networkIds,deviceIds:bucket.devices.map(d=>d.id),groupKey:bucket.key,layerId:bucket.layerId,grouped:true,position:positions.length?{x:Math.min(...positions.map(p=>p.x)),y:Math.min(...positions.map(p=>p.y))}:undefined});
   for(const d of bucket.devices)owner.set(d.id,id);
  }else for(const d of bucket.devices){nodes.push({id:d.id,name:d.name,role:bucket.role,networkId:bucket.networkId,networkIds:bucket.networkIds,deviceIds:[d.id],groupKey:bucket.key,layerId:bucket.layerId,grouped:false,device:d,position:saved(d)});owner.set(d.id,d.id)}
 }
 const edges=new Map<string,{id:string;from:string;to:string;links:DeviceLink[]}>(),internal=new Map<string,number>();
 for(const link of links){const from=owner.get(link.sourceId),to=owner.get(link.targetId);if(!from||!to)continue;if(from===to){internal.set(from,(internal.get(from)||0)+1);continue}
  const aggregate=from.startsWith('group:')||to.startsWith('group:');const key=aggregate?JSON.stringify([from,to].sort()):link.id;
  if(!edges.has(key))edges.set(key,{id:key,from,to,links:[]});edges.get(key)!.links.push(link);
 }
 return {nodes,edges:[...edges.values()],buckets:[...buckets.values()],internal};
}
