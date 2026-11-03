import {roleBase} from './deviceRoles';
import type {NetworkLayer,NetworkSegment} from './model';
export const defaultNetworkLayers:NetworkLayer[]=[{id:'core',name:'核心层'},{id:'aggregation',name:'汇聚层'},{id:'access',name:'接入层'},{id:'compute',name:'计算 / 存储'},{id:'other',name:'其他设备'}];
export function layersOf(n?:NetworkSegment){return n?.layers?.length?n.layers:defaultNetworkLayers}
export function layerOf(n:NetworkSegment|undefined,deviceId:string,role:string){const list=layersOf(n),override=n?.deviceLayers?.[deviceId];if(override&&list.some(l=>l.id===override))return override;const fallback=({core:'core',aggregation:'aggregation',access:'access',management:'access',server:'compute',storage:'compute'} as Record<string,string>)[roleBase(role)]||'other';return list.some(l=>l.id===fallback)?fallback:list[list.length-1].id}

export function networkLayersVisible(n?:NetworkSegment){return n?.layersVisible??!!n?.layers?.length}
