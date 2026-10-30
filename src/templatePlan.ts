import type {State,ModulePlan,Module,Infra,Rack} from './model';
export function captureModulePlan(state:State,id:string):ModulePlan{
 const m=state.modules.find(m=>m.id===id);if(!m)throw new Error('微模块不存在');
 return {name:m.name,type:m.type,point:m.point,product_model:m.productModel,rack_count:m.rackLeft,rack_left:m.rackLeft,rack_right:m.rackRight,u_height:42,rack_names:Object.fromEntries(['L','R'].map(side=>[side,state.racks.filter(r=>r.moduleId===id&&r.side===side).sort((a,b)=>a.index-b.index).map(r=>r.name)])),layout:state.infra.filter(i=>i.moduleId===id).map(i=>({name:i.name,kind:i.kind,side:i.side,after_rack:i.afterRack,placement:i.placement,...(i.order?{order:i.order}:{})}))};
}
export function previewPlan(plan:ModulePlan){
 const bounded=(v:number|undefined,max:number)=>Math.min(max,Math.max(1,Math.floor(Number(v)||1)));
 const m:Module={id:'preview',siteId:'preview',name:plan.name,productModel:plan.product_model||'',point:plan.point||'',type:plan.type==='double'?'double':'single',rackLeft:plan.type==='double'?bounded(plan.rack_left,12):bounded(plan.rack_count,24),rackRight:plan.type==='double'?bounded(plan.rack_right,12):0};
 const racks:Rack[]=[];for(const side of ['L','R'])for(let i=1;i<=(side==='L'?m.rackLeft:m.rackRight);i++)racks.push({id:side+i,name:plan.rack_names?.[side]?.[i-1]||side+String(i).padStart(2,'0'),moduleId:m.id,side,index:i,capacity:42});
 let layout=plan.layout;if(!layout){layout=[];for(const [kind,count] of Object.entries(plan.infra||{power:0,ac:0,env:0}))for(let i=0;i<Math.min(8,Math.max(0,count));i++)layout.push({kind,name:kind==='power'?'供电':kind==='ac'?'空调':'环境',side:m.type==='double'&&i%2?'R':'L',after_rack:kind==='env'?1:kind==='ac'?Math.floor((m.type==='double'&&i%2?m.rackRight:m.rackLeft)/2):0,placement:kind==='env'?'integrated':'inline'})}
 const infra:Infra[]=layout.slice(0,48).map((i,index)=>({id:'infra'+index,moduleId:m.id,name:i.name,kind:i.kind,side:i.side,afterRack:i.after_rack,placement:i.placement,...(i.order?{order:i.order}:{})}));return {module:m,racks,infra};
}
export function readTemplateFile(input:unknown):import('./model').ModuleTemplate{
 const doc=input as {format?:unknown;version?:unknown;template?:any};const t=doc?.template,p=t?.plan;
 if(doc?.format!=='aom-module-template'||doc.version!==1||!p||typeof p!=='object'||typeof t.name!=='string'||typeof p.name!=='string'||!['single','double'].includes(p.type)||p.u_height!==42)throw new Error('模板格式无效，请导入AOM导出的布局模板');
 const count=(n:unknown,max:number)=>typeof n==='number'&&Number.isInteger(n)&&n>=1&&n<=max;
 if(p.type==='single'?!count(p.rack_count,24):!count(p.rack_left,12)||!count(p.rack_right,12))throw new Error('模板机柜数量超出范围');
 if(p.layout!=null&&(!Array.isArray(p.layout)||p.layout.length>48||p.layout.some((i:any)=>!i||typeof i.name!=='string'||typeof i.kind!=='string'||!['L','R'].includes(i.side)||!['inline','integrated'].includes(i.placement)||!Number.isInteger(i.after_rack))))throw new Error('模板设施布局无效');
 if(p.rack_names!=null&&(typeof p.rack_names!=='object'||Object.entries(p.rack_names).some(([side,names])=>!['L','R'].includes(side)||!Array.isArray(names)||names.length>24||names.some(n=>typeof n!=='string'))))throw new Error('模板机柜名称列表无效');
 if(p.infra!=null&&(typeof p.infra!=='object'||['power','ac','env'].some(k=>!Number.isInteger(p.infra[k])||p.infra[k]<0||p.infra[k]>8)))throw new Error('模板设施数量无效');
 const plan=JSON.parse(JSON.stringify(p));delete plan.room_position;delete plan.room_rotation;
 return {id:'',name:t.name+' 副本',description:typeof t.description==='string'?t.description:'',updatedAt:'',plan};
}
