import {computed,ref,shallowRef,onMounted,onBeforeUnmount,onDeactivated,watch,type Ref} from 'vue';
import {finiteRoomMatrix,transformRoomPointer} from './roomLayout';
interface Lane {parentId?:string;childrenY?:number;id:string;x:number;y:number;width?:number;height?:number}
interface Point {x:number;y:number}
export function useNetworkDrag(svg:Ref<SVGSVGElement|undefined>,canvas:Ref<HTMLElement|undefined>,lanes:()=>Lane[],enabled:()=>boolean,save:(id:string,position:Point)=>void,notice:(text:string)=>void=()=>{}){
 const active=shallowRef<{id:string;pointer:number;capture:Element;matrix:DOMMatrix;start:Point;origin:Point;screen:Point;scroll:Point;moved:boolean}|null>(null);
 const preview=ref<{id:string;position:Point}|null>(null),pending=ref<{id:string;position:Point}|null>(null);
 let suppressClick=false;let frame=0,last:Point|null=null;
 const invalid=computed(()=>!!preview.value&&!!overlaps(preview.value.id,preview.value.position));
 const dragging=computed(()=>!!active.value?.moved);
 function offset(id:string){const p=preview.value||pending.value;if(!p)return {x:0,y:0};let lane=lanes().find(l=>l.id===id);const seen=new Set<string>();while(lane&&!seen.has(lane.id)){if(lane.id===p.id)return {x:p.position.x-lane.x,y:p.position.y-lane.y};seen.add(lane.id);lane=lanes().find(l=>l.id===lane!.parentId)}return {x:0,y:0}}
 function overlaps(id:string,p:Point){const lane=lanes().find(l=>l.id===id);return lane?.width&&lane.height&&lanes().some(l=>l.id!==id&&(l.parentId||'')===(lane.parentId||'')&&l.width&&l.height&&p.x<l.x+l.width+16&&p.x+lane.width!+16>l.x&&p.y<l.y+l.height+16&&p.y+lane.height!+16>l.y)}
 function constrain(id:string,p:Point){const lane=lanes().find(l=>l.id===id),parent=lanes().find(l=>l.id===lane?.parentId);return {x:Math.max(parent?parent.x+20:0,Math.min(10000,p.x)),y:Math.max(parent?parent.childrenY||parent.y+88:0,Math.min(10000,p.y))}}
 function persist(id:string,position:Point){const lane=lanes().find(l=>l.id===id),parent=lanes().find(l=>l.id===lane?.parentId);save(id,parent?{x:position.x-parent.x,y:position.y-parent.y}:position)}
 function consumeClick(){const value=suppressClick;suppressClick=false;return value}
 function down(e:PointerEvent,id:string){suppressClick=false;
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
  preview.value={id:a.id,position:constrain(a.id,{x:Math.round((a.origin.x+p.x-a.start.x)*100)/100,y:Math.round((a.origin.y+p.y-a.start.y)*100)/100})};
  frame=requestAnimationFrame(paint);
 }
 function move(e:PointerEvent){if(active.value?.pointer!==e.pointerId)return;last={x:e.clientX,y:e.clientY};if(!frame)frame=requestAnimationFrame(paint)}
 function clear(){cancelAnimationFrame(frame);frame=0;const a=active.value;active.value=null;preview.value=null;last=null;if(a?.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer)}
 function cancel(){clear()}
 function up(e:PointerEvent){
  const a=active.value;if(!a||e.pointerId!==a.pointer)return;
  last={x:e.clientX,y:e.clientY};cancelAnimationFrame(frame);paint();const p=preview.value,rect=canvas.value?.getBoundingClientRect();
  const inside=rect&&e.clientX>=rect.left&&e.clientX<=rect.right&&e.clientY>=rect.top&&e.clientY<=rect.bottom;
  suppressClick=a.moved;const changed=p&&(p.position.x!==a.origin.x||p.position.y!==a.origin.y);clear();
  if(inside&&changed&&enabled()){if(overlaps(a.id,p.position)){notice('同级网络不能重叠，本次移动已取消；可拖到空白位置');return}pending.value=p;persist(a.id,p.position)}
 }
 function key(e:KeyboardEvent,id:string){
  if(!enabled()||pending.value||active.value||!e.altKey||!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key))return;
  const lane=lanes().find(l=>l.id===id);if(!lane||!id||id==='unassigned')return;e.preventDefault();
  const position=constrain(id,{x:lane.x+(e.key==='ArrowLeft'?-12:e.key==='ArrowRight'?12:0),y:lane.y+(e.key==='ArrowUp'?-12:e.key==='ArrowDown'?12:0)});
  if(position.x===lane.x&&position.y===lane.y)return;if(overlaps(id,position)){notice('位置与其他网络重叠，未移动');return}pending.value={id,position};persist(id,position);
 }
 watch(enabled,value=>{if(value&&!active.value)pending.value=null});
 const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')cancel()};onMounted(()=>window.addEventListener('keydown',escape));onBeforeUnmount(()=>{window.removeEventListener('keydown',escape);cancel()});onDeactivated(cancel);
 return {consumeClick,active,dragging,invalid,offset,down,move,up,cancel,key};
}
