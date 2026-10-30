import type {NetworkSegment} from './model';
export function networkPath(networks:NetworkSegment[],id:string):NetworkSegment[]{const path:NetworkSegment[]=[];const seen=new Set<string>();let n=networks.find(n=>n.id===id);while(n&&!seen.has(n.id)){path.unshift(n);seen.add(n.id);n=networks.find(p=>p.id===n!.parentId)}return path}
export function networkSubtree(networks:NetworkSegment[],id:string):NetworkSegment[]{if(!id)return networks;return networks.filter(n=>networkPath(networks,n.id).some(p=>p.id===id))}
export function canParentNetwork(networks:NetworkSegment[],id:string,parentId:string){return !parentId||!networkPath(networks,parentId).some(n=>n.id===id)}
