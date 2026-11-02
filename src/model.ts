export interface Project { id: string; name: string }
export interface RoomEntrance {side:string;offset:number}
export interface Site { entrance?:RoomEntrance; id: string; name: string; projectId: string }
export interface Module {roomRotation?:number; roomPosition?:CanvasPoint; productModel: string; point: string; id: string; siteId: string; name: string; type: 'single' | 'double'; rackLeft: number; rackRight: number }
export interface Rack { id: string; moduleId: string; name: string; side: string; index: number; capacity: number }
export interface Infra { order?:number; side: string; afterRack: number; placement: string; id: string; moduleId: string; kind: string; name: string }
export interface Model { id: string; name: string; vendor: string; role: string; uHeight: number; ports: string }
export interface Device {topologyPosition?:CanvasPoint; id: string; assetTag: string; name: string; modelId: string; moduleId: string; rackId: string; startU: number; ip: string; sn: string; status: string; note: string; source: string }
export interface State {templates:ModuleTemplate[]; networks: NetworkSegment[]; links: DeviceLink[]; projects: Project[]; schemaVersion: number; revision: number; sites: Site[]; modules: Module[]; racks: Rack[]; infra: Infra[]; models: Model[]; devices: Device[] }
export interface Audit { id: number; time: string; action: string; summary: string; revision: number }
export interface Preview { total: number; placed: number; pending: number; revision: number; rows: { device: Device; reason: string; line: number }[] }
export const roleNames: Record<string, string> = { server: '计算服务器', core: '核心交换', aggregation: '汇聚交换', access: '业务接入', management: '管理接入', storage: '存储设备', infra: '基础设施' };
export const roleColors: Record<string, string> = { server: '#497cda', core: '#8c67cf', aggregation: '#a786df', access: '#1b9c88', management: '#51b5a3', storage: '#c49745', infra: '#7e8997' };
export const statuses: Record<string, string> = { pending: '待上架', installed: '已上架', maintenance: '维护中' };
export const blankDevice = (): Device => ({ id: '', assetTag: '', name: '', modelId: '', moduleId: '', rackId: '', startU: 0, ip: '', sn: '', status: 'pending', note: '', source: 'manual' });
export const blankModel = (): Model => ({ id: '', name: '', vendor: '', role: 'server', uHeight: 2, ports: '' });
export async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const response = await fetch('/api' + path, { method, headers: body ? { 'Content-Type': 'application/json' } : {}, body: body ? JSON.stringify(body) : undefined });
  const data = await response.json();
  if(response.status===401)window.dispatchEvent(new Event('aom:unauthorized'));
  if (!response.ok) throw Object.assign(new Error(data.message || '请求失败'), { status: response.status });
  return data;
}
export function download(name: string, text: string, mime = 'application/json') { const url = URL.createObjectURL(new Blob([text], { type: mime })); const a = document.createElement('a'); a.href = url; a.download = name; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); }
export const sampleCSV = 'model,quantity,name,asset_prefix,ip,sn,note\nKunTai R722 K2,12,示例计算节点,DEMO-COMPUTE,,,仅用于验证导入流程\nKunTai R624 K2,4,示例存储节点,DEMO-STORAGE,,,示例数据\nRZ2805-48YS8CQ,2,示例业务接入,DEMO-NET,,,示例数据\nRZ18001-1,1,示例核心设备,DEMO-CORE,,,U高待确认，保留待上架\n';

export const infraNames: Record<string,string> = {ac:"空调",power:"供电",env:"环境监测",ups:"UPS",battery:"电池"};
export const infraColors: Record<string,string> = {ac:"#47a4cf",power:"#cf9e4e",env:"#9a7ac7",ups:"#c88e59",battery:"#759a73"};
export const moduleProducts = ["曙光数创 C2000（SG6600）", "曙光数创 C500（C500-40K）"];

export interface CanvasPoint {x:number;y:number}
export interface NetworkLayer {id:string;name:string}
export interface NetworkSegment {layers?:NetworkLayer[];deviceLayers?:Record<string,string>;parentId?:string;position?:CanvasPoint;devicePositions?:Record<string,CanvasPoint>;id:string;projectId:string;name:string;purpose:string;vlan:number;cidr:string;gateway:string;color:string;deviceIds:string[];note:string}
export interface DeviceLink {id:string;projectId:string;sourceId:string;targetId:string;sourcePort:string;targetPort:string;networkIds:string[];kind:'physical'|'logical';medium:string;speedGbps:number;status:'pending'|'confirmed';origin:'manual'|'inferred';note:string}
export const networkPurposes: Record<string,string> = {business:'业务网络',management:'管理网络',storage:'存储网络',interconnect:'互联网络',other:'其他网络'};

export interface InfraSlot {order?:number;kind:string;name:string;side:string;after_rack:number;placement:string}
export interface ModulePlan {room_rotation?:number;room_position?:CanvasPoint;name:string;type:string;rack_count?:number;rack_left?:number;rack_right?:number;u_height:number;point?:string;product_model?:string;rack_names?:Record<string,string[]>;infra?:{power:number;ac:number;env:number};layout?:InfraSlot[]|null}
export interface ModuleTemplate {id:string;name:string;description:string;plan:ModulePlan;updatedAt:string}
