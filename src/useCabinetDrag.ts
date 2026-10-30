import {ref,onMounted,onBeforeUnmount} from 'vue';
import {cabinetDropAt,cabinetOrderError,insertLayoutItem,type CabinetDrop} from './dragGeometry';
import type {Rack} from './model';
import type {LayoutTile} from './layout';

export function useCabinetDrag(getRacks:()=>Rack[],rows:()=>{side:string;tiles:LayoutTile[]}[],save:(id:string,side:string,beforeId:string)=>void,enabled:()=>boolean){
 const active=ref<{id:string;x:number;y:number;moved:boolean;pointer:number;capture:Element}|null>(null);
 const destination=ref<(CabinetDrop&{error:string})|null>(null);
 let suppressClick=false,frame=0,last={x:0,y:0};
 let scope:SVGSVGElement|null=null;
 function orderError(next:{L:string[];R:string[]}){const ids=new Set(getRacks().map(r=>r.id));return cabinetOrderError({L:next.L.filter(id=>ids.has(id)),R:next.R.filter(id=>ids.has(id))},rows().some(row=>row.side==='R'))}
 function target(x:number,y:number){
  const a=active.value,rowElement=document.elementFromPoint(x,y)?.closest('[data-cabinet-row]');
  if(!a||!rowElement||!scope?.contains(rowElement)){destination.value=null;return}
  const row=rows().find(row=>row.side===rowElement.getAttribute('data-cabinet-row')),matrix=scope.getScreenCTM();
  if(!row||!matrix){destination.value=null;return}
  const local=new DOMPoint(x,y).matrixTransform(matrix.inverse());
  const drop=cabinetDropAt(row.tiles,row.side,local.x-80,a.id);
  const next=drop&&insertLayoutItem(rows(),a.id,drop.side,drop.beforeId);
  destination.value=drop&&next?{...drop,error:orderError(next)}:null;
 }
 function tick(){
  if(!active.value?.moved)return;
  const scroller=scope?.closest<HTMLElement>('.floor-scroll,.overview-module-map');
  if(scroller){const b=scroller.getBoundingClientRect();if(last.y>=b.top&&last.y<=b.bottom)scroller.scrollLeft+=last.x<b.left+40?-12:last.x>b.right-40?12:0}
  target(last.x,last.y);frame=requestAnimationFrame(tick);
 }
 function down(e:PointerEvent,rack:Rack){
  if(e.button!==0||active.value||!enabled())return;
  e.preventDefault();scope=(e.currentTarget as Element).closest('svg');
  const capture=e.currentTarget as Element;capture.setPointerCapture(e.pointerId);
  active.value={id:rack.id,x:e.clientX,y:e.clientY,moved:false,pointer:e.pointerId,capture};
  last={x:e.clientX,y:e.clientY};suppressClick=false;
 }
 function move(e:PointerEvent){
  const a=active.value;if(!a||e.pointerId!==a.pointer)return;
  last={x:e.clientX,y:e.clientY};
  if(!a.moved&&Math.hypot(e.clientX-a.x,e.clientY-a.y)>6){a.moved=true;frame=requestAnimationFrame(tick)}
  if(a.moved)target(e.clientX,e.clientY);
 }
 function clear(){
  cancelAnimationFrame(frame);const a=active.value;active.value=null;destination.value=null;
  if(a?.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer);
 }
 function cancel(){if(active.value)suppressClick=active.value.moved;clear()}
 function saveIfChanged(id:string,side:string,beforeId:string|null){
  if(!enabled())return;
  const current=rows(),next=insertLayoutItem(current,id,side,beforeId);
  if(!next||orderError(next))return;
  const old=(side:string)=>current.find(r=>r.side===side)?.tiles.map(t=>t.id)||[];
  if(JSON.stringify([old('L'),old('R')])!==JSON.stringify([next.L,next.R]))save(id,side,beforeId||'');
 }
 function up(e:PointerEvent){
  const a=active.value;if(!a||e.pointerId!==a.pointer)return;
  target(e.clientX,e.clientY);const dest=destination.value;suppressClick=a.moved;clear();
  if(a.moved&&dest&&!dest.error)saveIfChanged(a.id,dest.side,dest.beforeId);
 }
 function clicked(fn:()=>void){if(suppressClick){suppressClick=false;return}fn()}
 function key(e:KeyboardEvent,rack:Rack){
  if(!e.altKey||!enabled()||!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key))return;
  e.preventDefault();
  if(e.key==='ArrowUp'||e.key==='ArrowDown'){
   const side=e.key==='ArrowUp'?'L':'R';
   if(side===rack.side||!rows().some(row=>row.side===side))return;
   const other=getRacks().filter(r=>r.side===side).sort((a,b)=>a.index-b.index);
   saveIfChanged(rack.id,side,other[rack.index-1]?.id||null);return;
  }
  const row=rows().find(r=>r.side===rack.side)?.tiles||[],index=row.findIndex(t=>t.id===rack.id);
  if(index<0||e.key==='ArrowLeft'&&index===0||e.key==='ArrowRight'&&index===row.length-1)return;
  saveIfChanged(rack.id,rack.side,e.key==='ArrowLeft'?row[index-1]!.id:row[index+2]?.id||null);
 }
 const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')cancel()};
 onMounted(()=>window.addEventListener('keydown',escape));
 onBeforeUnmount(()=>{window.removeEventListener('keydown',escape);clear()});
 return {active,destination,down,move,up,cancel,clicked,key};
}
