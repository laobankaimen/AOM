import {reactive} from 'vue';
import type {DeviceRole} from './model';
export const builtinRoles:DeviceRole[]=[{id:'server',name:'计算服务器',color:'#497cda',baseRole:'server'},{id:'core',name:'核心交换',color:'#8c67cf',baseRole:'core'},{id:'aggregation',name:'汇聚交换',color:'#a786df',baseRole:'aggregation'},{id:'access',name:'业务接入',color:'#1b9c88',baseRole:'access'},{id:'management',name:'管理接入',color:'#51b5a3',baseRole:'management'},{id:'storage',name:'存储设备',color:'#c49745',baseRole:'storage'},{id:'infra',name:'基础设施',color:'#7e8997',baseRole:'infra'}];
export const roleNames=reactive<Record<string,string>>({});
export const roleColors=reactive<Record<string,string>>({});
const bases=reactive<Record<string,string>>({});
export function roleDefinitions(custom:DeviceRole[]=[]){const map=new Map(builtinRoles.map(r=>[r.id,{...r}]));for(const r of custom)map.set(r.id,{...r});return [...map.values()]}
export function syncDeviceRoles(custom:DeviceRole[]=[]){for(const obj of [roleNames,roleColors,bases])for(const key of Object.keys(obj))delete obj[key];for(const r of roleDefinitions(custom)){roleNames[r.id]=r.name;roleColors[r.id]=r.color;bases[r.id]=r.baseRole}}
export function roleBase(role:string){return bases[role]||role}
syncDeviceRoles();
