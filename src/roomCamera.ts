import {computed,ref,watch,onMounted,onBeforeUnmount,type Ref} from 'vue';
export interface ViewBounds {x:number;y:number;width:number;height:number}
export function fillCamera(bounds:ViewBounds,width:number,height:number,zoom:number,pan={x:0,y:0}){
 const scale=Math.min(width/Math.max(1,bounds.width),height/Math.max(1,bounds.height))*zoom/100;
 const w=width/Math.max(.001,scale),h=height/Math.max(.001,scale);
 return {x:bounds.x+(bounds.width-w)/2+pan.x,y:bounds.y+(bounds.height-h)/2+pan.y,width:w,height:h};
}
export function useRoomCamera(canvas:Ref<HTMLElement|undefined>,bounds:()=>ViewBounds,zoom:()=>number,locked:()=>boolean,setZoom?:(value:number)=>void){
 const size=ref({width:1000,height:600}),pan=ref({x:0,y:0});let observer:ResizeObserver|undefined,bound:HTMLElement|undefined;
 function measure(){if(!locked()&&canvas.value?.clientWidth&&canvas.value?.clientHeight)size.value={width:canvas.value.clientWidth,height:canvas.value.clientHeight}}
 const view=computed(()=>fillCamera(bounds(),size.value.width,size.value.height,zoom(),pan.value));
 function change(value:number,client?:{x:number;y:number}){
  if(locked()||!setZoom||!Number.isFinite(value))return;
  const next=Math.max(25,Math.min(400,Math.round(value))),rect=canvas.value?.getBoundingClientRect();
  const fx=rect&&client?(client.x-rect.left)/rect.width:.5,fy=rect&&client?(client.y-rect.top)/rect.height:.5;
  const before=view.value,after=fillCamera(bounds(),size.value.width,size.value.height,next,pan.value);
  pan.value={x:pan.value.x+before.x+fx*before.width-after.x-fx*after.width,y:pan.value.y+before.y+fy*before.height-after.y-fy*after.height};setZoom(next);
 }
 function wheel(event:WheelEvent){if(locked()||!event.deltaY||!setZoom)return;event.preventDefault();const delta=event.deltaY*(event.deltaMode===1?16:event.deltaMode===2?size.value.height:1);change(zoom()*Math.exp(-Math.max(-400,Math.min(400,delta))*.0015),{x:event.clientX,y:event.clientY})}
 function bind(){if(bound===canvas.value){if(bound){observer?.observe(bound);measure()}return;}if(bound){bound.removeEventListener('wheel',wheel);observer?.unobserve(bound)}bound=canvas.value;if(bound){bound.addEventListener('wheel',wheel,{passive:false});observer?.observe(bound);measure()}}
 onMounted(()=>{observer=new ResizeObserver(measure);bind()});watch(canvas,bind,{flush:'post'});
 onBeforeUnmount(()=>{bound?.removeEventListener('wheel',wheel);observer?.disconnect()});
 function reset(){pan.value={x:0,y:0};measure()}
 return {view,pan,size,reset,change};
}
