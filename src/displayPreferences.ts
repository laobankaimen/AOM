import {computed,reactive,watch} from 'vue';
const defaults={sidebar:true,stats:true,labels:true,details:true,tools:true,networks:true};
const key='aom.display-preferences';
function load(){try{const saved=JSON.parse(localStorage.getItem(key)||'{}');return Object.fromEntries(Object.entries(defaults).map(([name,value])=>[name,typeof saved[name]==='boolean'?saved[name]:value])) as typeof defaults}catch{return {...defaults}}}
export const displayPreferences=reactive(load());
watch(displayPreferences,value=>{try{localStorage.setItem(key,JSON.stringify(value))}catch{}},{deep:true});
export const displayClasses=computed(()=>Object.fromEntries(Object.entries(displayPreferences).map(([key,value])=>['hide-'+key,!value])));
export function resetDisplay(){Object.assign(displayPreferences,defaults)}
let previous:typeof defaults|undefined;
export function focusDisplay(){if(focusedDisplay.value){Object.assign(displayPreferences,previous||defaults);previous=undefined}else{previous={...displayPreferences};for(const key of Object.keys(defaults) as (keyof typeof defaults)[])displayPreferences[key]=false}}
export const focusedDisplay=computed(()=>Object.values(displayPreferences).every(v=>!v));
