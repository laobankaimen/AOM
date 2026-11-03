import {reactive} from 'vue';
import type {NetworkSegment} from './model';
import {networkLayersVisible} from './networkLayers';

const key='aom.network-layer-display';
function load():Record<string,boolean>{
 try{const value=JSON.parse(localStorage.getItem(key)||'{}');return Object.fromEntries(Object.entries(value).filter((entry):entry is [string,boolean]=>typeof entry[1]==='boolean'))}
 catch{return {}}
}
const choices=reactive(load());
export function layerDisplayVisible(network?:NetworkSegment){return !!network&&(choices[network.id]??networkLayersVisible(network))}
export function setLayerDisplayVisible(id:string,shown:boolean){
 choices[id]=shown;
 try{localStorage.setItem(key,JSON.stringify(choices))}catch{}
}
