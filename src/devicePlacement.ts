import type {Device,State} from './model';

/** One room shares its pending devices and rack choices across all modules. */
export function roomRacks(state:State,moduleId:string){
 const room=state.modules.find(m=>m.id===moduleId)?.siteId;
 if(!room)return [];
 const modules=state.modules.filter(m=>m.siteId===room),order=new Map(modules.map((m,index)=>[m.id,index]));
 return state.racks.filter(r=>order.has(r.moduleId)).sort((a,b)=>order.get(a.moduleId)!-order.get(b.moduleId)!||a.side.localeCompare(b.side)||a.index-b.index);
}
export function freeRackUnits(state:State,device:Device,rackId:string){
 const rack=state.racks.find(r=>r.id===rackId),height=state.models.find(m=>m.id===device.modelId)?.uHeight||0;
 if(!rack||!height)return [];
 const occupied=new Set<number>();
 for(const other of state.devices.filter(d=>d.rackId===rack.id&&d.id!==device.id)){
  const size=state.models.find(m=>m.id===other.modelId)?.uHeight||0;
  for(let u=other.startU;u<other.startU+size;u++)occupied.add(u);
 }
 return Array.from({length:Math.max(0,rack.capacity-height+1)},(_,index)=>index+1).filter(u=>Array.from({length:height},(_,index)=>u+index).every(u=>!occupied.has(u)));
}
