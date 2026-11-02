import {computed,ref,onMounted,onBeforeUnmount,type Ref} from 'vue';
export interface ViewBounds {x:number;y:number;width:number;height:number}
export function fillCamera(bounds:ViewBounds,width:number,height:number,zoom:number,pan={x:0,y:0}){
 const scale=Math.min(width/Math.max(1,bounds.width),height/Math.max(1,bounds.height))*zoom/100;
 const w=width/Math.max(.001,scale),h=height/Math.max(.001,scale);
 return {x:bounds.x+(bounds.width-w)/2+pan.x,y:bounds.y+(bounds.height-h)/2+pan.y,width:w,height:h};
}
export function useRoomCamera(canvas:Ref<HTMLElement|undefined>,bounds:()=>ViewBounds,zoom:()=>number,locked:()=>boolean){
 const size=ref({width:1000,height:600}),pan=ref({x:0,y:0});let observer:ResizeObserver|undefined;
 function measure(){if(!locked()&&canvas.value?.clientWidth&&canvas.value?.clientHeight)size.value={width:canvas.value.clientWidth,height:canvas.value.clientHeight}}
 onMounted(()=>{observer=new ResizeObserver(measure);if(canvas.value)observer.observe(canvas.value);measure()});onBeforeUnmount(()=>observer?.disconnect());
 const view=computed(()=>fillCamera(bounds(),size.value.width,size.value.height,zoom(),pan.value));
 function reset(){pan.value={x:0,y:0};measure()}
 return {view,pan,size,reset};
}
