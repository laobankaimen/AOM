import { ref,computed,onMounted,onBeforeUnmount } from 'vue';
import type { Device, Model } from './model';
import {rackUnitAt,rackDropUnit} from './dragGeometry';
export function useDeviceDrag(devices:()=>Device[],models:()=>Model[],onMove:(id:string,rack:string,u:number)=>void,onSelect:(id:string)=>void,enabled:()=>boolean){
 const active=ref<{id:string;pointer:number;x:number;y:number;offset:number;moved:boolean;capture:Element}|null>(null);
 const target=ref<{rackId:string;startU:number;valid:boolean;reason:string}|null>(null);let frame=0,last={x:0,y:0},container:HTMLElement|null=null;
 const height=computed(()=>{const d=devices().find(d=>d.id===active.value?.id);return models().find(m=>m.id===d?.modelId)?.uHeight||0});
 function locate(x:number,y:number){
  const svg=document.elementFromPoint(x,y)?.closest<SVGSVGElement>('svg[data-drop-rack]');
  if(!svg||!container?.contains(svg)||!active.value){target.value=null;return}
  const u=rackDropUnit(x,y,svg.getBoundingClientRect(),active.value.offset);if(u===null){target.value=null;return}
  const rackId=svg.dataset.dropRack!;let reason='';if(!height.value)reason='型号高度待确认';else if(u<1||u+height.value-1>42)reason='超出1–42U';else if(devices().some(d=>d.id!==active.value!.id&&d.rackId===rackId&&u<d.startU+(models().find(m=>m.id===d.modelId)?.uHeight||0)&&d.startU<u+height.value))reason='目标U位已占用';
  target.value={rackId,startU:u,valid:!reason,reason};
 }
 function tick(){if(!active.value?.moved)return;const b=container?.getBoundingClientRect();if(container&&b){if(last.y>=b.top&&last.y<=b.bottom){container.scrollLeft+=last.x<b.left+48?-14:last.x>b.right-48?14:0;container.scrollTop+=last.y<b.top+40?-12:last.y>b.bottom-40?12:0}}
 locate(last.x,last.y);frame=requestAnimationFrame(tick)}
 function start(e:PointerEvent,d:Device){if(e.button!==0||!enabled())return;e.preventDefault();const el=e.currentTarget as Element;const svg=el.closest<SVGSVGElement>('svg[data-drop-rack]');container=el.closest<HTMLElement>('.elevation-scroll');if(!svg||!container)return;el.setPointerCapture(e.pointerId);active.value={id:d.id,pointer:e.pointerId,x:e.clientX,y:e.clientY,offset:rackUnitAt(e.clientY,svg.getBoundingClientRect())-d.startU,moved:false,capture:el};last={x:e.clientX,y:e.clientY}}
 function move(e:PointerEvent){if(!active.value)return;last={x:e.clientX,y:e.clientY};if(!active.value.moved&&Math.hypot(e.clientX-active.value.x,e.clientY-active.value.y)>5){active.value.moved=true;frame=requestAnimationFrame(tick)};if(active.value.moved)locate(e.clientX,e.clientY)}
 function cancel(){cancelAnimationFrame(frame);const a=active.value;if(a?.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer);active.value=null;target.value=null}
 function end(e:PointerEvent){const a=active.value;if(!a)return;locate(e.clientX,e.clientY);const dest=target.value;cancel();if(a.moved){if(dest){const old=devices().find(d=>d.id===a.id);if(old?.rackId!==dest.rackId||old?.startU!==dest.startU)onMove(a.id,dest.rackId,dest.startU)}}else onSelect(a.id)}
 function key(e:KeyboardEvent){if(e.key==='Escape'&&active.value){e.preventDefault();cancel()}}
 onMounted(()=>window.addEventListener('keydown',key));onBeforeUnmount(()=>{window.removeEventListener('keydown',key);cancel()});return {active,target,height,start,move,end,cancel,key};
}
