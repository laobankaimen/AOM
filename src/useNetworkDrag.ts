import {computed,ref,shallowRef,onMounted,onBeforeUnmount,onDeactivated,watch,type Ref} from 'vue';
import {finiteRoomMatrix,transformRoomPointer} from './roomLayout';
interface Lane {id:string;x:number;y:number}
interface Point {x:number;y:number}
export function useNetworkDrag(svg:Ref<SVGSVGElement|undefined>,canvas:Ref<HTMLElement|undefined>,lanes:()=>Lane[],enabled:()=>boolean,save:(id:string,position:Point)=>void){
 const active=shallowRef<{id:string;pointer:number;capture:Element;matrix:DOMMatrix;start:Point;origin:Point;screen:Point;scroll:Point;moved:boolean}|null>(null);
 const preview=ref<{id:string;position:Point}|null>(null),pending=ref<{id:string;position:Point}|null>(null);
 let frame=0,last:Point|null=null;
 const dragging=computed(()=>!!active.value?.moved);
 function offset(id:string){const p=preview.value?.id===id?preview.value:pending.value?.id===id?pending.value:null;const lane=lanes().find(l=>l.id===id);return p&&lane?{x:p.position.x-lane.x,y:p.position.y-lane.y}:{x:0,y:0}}
 function down(e:PointerEvent,id:string){
  if(e.button!==0||!enabled()||active.value||pending.value||!id||id==='unassigned')return;
  const lane=lanes().find(l=>l.id===id),matrix=svg.value?.getScreenCTM()?.inverse();if(!lane||!matrix||!finiteRoomMatrix(matrix))return;
  const point=transformRoomPointer({x:e.clientX,y:e.clientY},matrix);if(!point)return;
  e.preventDefault();e.stopPropagation();const capture=e.currentTarget as Element;capture.setPointerCapture(e.pointerId);
  active.value={id,pointer:e.pointerId,capture,matrix,start:point,origin:{x:lane.x,y:lane.y},screen:{x:e.clientX,y:e.clientY},scroll:{x:canvas.value?.scrollLeft||0,y:canvas.value?.scrollTop||0},moved:false};
  last={x:e.clientX,y:e.clientY};
 }
 function paint(){
  frame=0;const a=active.value;if(!a||!last)return;
  const c=canvas.value;
  if(a.moved&&c){const r=c.getBoundingClientRect();if(last.x>=r.left&&last.x<=r.right&&last.y>=r.top&&last.y<=r.bottom){c.scrollLeft+=last.x<r.left+30?-10:last.x>r.right-30?10:0;c.scrollTop+=last.y<r.top+30?-10:last.y>r.bottom-30?10:0}}
  const p=transformRoomPointer(last,a.matrix,{x:(c?.scrollLeft||0)-a.scroll.x,y:(c?.scrollTop||0)-a.scroll.y});if(!p){cancel();return}
  if(!a.moved&&Math.hypot(last.x-a.screen.x,last.y-a.screen.y)<5)return;
  if(!a.moved)active.value={...a,moved:true};
  preview.value={id:a.id,position:{x:Math.max(0,Math.min(10000,Math.round((a.origin.x+p.x-a.start.x)*100)/100)),y:Math.max(0,Math.min(10000,Math.round((a.origin.y+p.y-a.start.y)*100)/100))}};
  frame=requestAnimationFrame(paint);
 }
 function move(e:PointerEvent){if(active.value?.pointer!==e.pointerId)return;last={x:e.clientX,y:e.clientY};if(!frame)frame=requestAnimationFrame(paint)}
 function clear(){cancelAnimationFrame(frame);frame=0;const a=active.value;active.value=null;preview.value=null;last=null;if(a?.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer)}
 function cancel(){clear()}
 function up(e:PointerEvent){
  const a=active.value;if(!a||e.pointerId!==a.pointer)return;
  last={x:e.clientX,y:e.clientY};cancelAnimationFrame(frame);paint();const p=preview.value,rect=canvas.value?.getBoundingClientRect();
  const inside=rect&&e.clientX>=rect.left&&e.clientX<=rect.right&&e.clientY>=rect.top&&e.clientY<=rect.bottom;
  const changed=p&&(p.position.x!==a.origin.x||p.position.y!==a.origin.y);clear();
  if(inside&&changed&&enabled()){pending.value=p;save(a.id,p.position)}
 }
 function key(e:KeyboardEvent,id:string){
  if(!enabled()||pending.value||active.value||!e.altKey||!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key))return;
  const lane=lanes().find(l=>l.id===id);if(!lane||!id||id==='unassigned')return;e.preventDefault();
  const position={x:Math.max(0,Math.min(10000,lane.x+(e.key==='ArrowLeft'?-12:e.key==='ArrowRight'?12:0))),y:Math.max(0,Math.min(10000,lane.y+(e.key==='ArrowUp'?-12:e.key==='ArrowDown'?12:0)))};
  if(position.x===lane.x&&position.y===lane.y)return;pending.value={id,position};save(id,position);
 }
 watch(enabled,value=>{if(value&&!active.value)pending.value=null});
 const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')cancel()};onMounted(()=>window.addEventListener('keydown',escape));onBeforeUnmount(()=>{window.removeEventListener('keydown',escape);cancel()});onDeactivated(cancel);
 return {active,dragging,offset,down,move,up,cancel,key};
}
