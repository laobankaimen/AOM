import type {State,NetworkSegment} from './model';import {networkPath,networkSubtree} from './networkTree';
export function networkRoomIds(state:State,network:NetworkSegment):string[]{
 if(network.roomScope==='project')return state.sites.filter(r=>r.projectId===network.projectId).map(r=>r.id);
 const ids=new Set<string>();
 for(const n of networkSubtree(state.networks,network.id)){
  if(n.siteId)ids.add(n.siteId);
  for(const id of n.deviceIds){const d=state.devices.find(d=>d.id===id),m=state.modules.find(m=>m.id===d?.moduleId);if(m)ids.add(m.siteId)}
 }
 if(!ids.size){for(const parent of networkPath(state.networks,network.id).reverse().slice(1)){if(parent.siteId){ids.add(parent.siteId);break}if(parent.roomScope==='project')return state.sites.filter(r=>r.projectId===network.projectId).map(r=>r.id)}}
 if(!ids.size){const rooms=state.sites.filter(r=>r.projectId===network.projectId);if(rooms.length===1)ids.add(rooms[0].id)}
 return [...ids];
}
export function networksForRoom(state:State,networks:NetworkSegment[],roomId:string){
 if(!roomId)return networks;const visible=new Set<string>();
 for(const n of networks){const rooms=networkRoomIds(state,n);if(!rooms.length||rooms.includes(roomId))for(const parent of networkPath(networks,n.id))visible.add(parent.id)}
 return networks.filter(n=>visible.has(n.id));
}
export function networkRoomLabel(state:State,n:NetworkSegment){const ids=networkRoomIds(state,n);return n.roomScope==='project'||ids.length>1?'跨机房':ids.length===1?state.sites.find(r=>r.id===ids[0])?.name||'机房已移除':'待指定机房'}
