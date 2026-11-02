import {computed,onBeforeUnmount,onMounted,ref,type Ref} from 'vue';
export function fitDiagram(width:number,height:number,availableWidth:number,availableHeight:number,zoom=100){
 const scale=Math.min(Math.max(1,availableWidth-24)/Math.max(1,width),Math.max(1,availableHeight-24)/Math.max(1,height))*zoom/100;
 return {width:Math.max(1,width*scale),height:Math.max(1,height*scale)};
}
/** Scale the drawing, not its data/layout. Observe both browser zoom and panel resize. */
export function useDiagramViewport(container:Ref<HTMLElement|undefined>,content:()=>{width:number;height:number},zoom:()=>number,locked:()=>boolean=()=>false){
 const viewport=ref({width:1000,height:600});let observer:ResizeObserver|undefined;
 function measure(){if(locked())return;const el=container.value;if(el&&el.clientWidth&&el.clientHeight)viewport.value={width:el.clientWidth,height:el.clientHeight}}
 let pan:{pointer:number;x:number;y:number;left:number;top:number}|null=null;
 function down(event:PointerEvent){const el=container.value;if(!el||event.button!==0||locked()||!(event.target as Element).closest('svg')||(event.target as Element).closest('.rack-hit'))return;event.preventDefault();el.setPointerCapture(event.pointerId);pan={pointer:event.pointerId,x:event.clientX,y:event.clientY,left:el.scrollLeft,top:el.scrollTop}}
 function move(event:PointerEvent){const el=container.value;if(el&&pan?.pointer===event.pointerId){el.scrollLeft=pan.left-(event.clientX-pan.x);el.scrollTop=pan.top-(event.clientY-pan.y)}}
 function up(){const el=container.value;if(el&&pan&&el.hasPointerCapture(pan.pointer))el.releasePointerCapture(pan.pointer);pan=null}
 onMounted(()=>{container.value?.addEventListener('pointerdown',down);container.value?.addEventListener('pointermove',move);container.value?.addEventListener('pointerup',up);container.value?.addEventListener('pointercancel',up);observer=new ResizeObserver(measure);if(container.value)observer.observe(container.value);window.addEventListener('resize',measure);measure()});
 onBeforeUnmount(()=>{up();container.value?.removeEventListener('pointerdown',down);container.value?.removeEventListener('pointermove',move);container.value?.removeEventListener('pointerup',up);container.value?.removeEventListener('pointercancel',up);observer?.disconnect();window.removeEventListener('resize',measure)});
 const size=computed(()=>{const c=content();return fitDiagram(c.width,c.height,viewport.value.width,viewport.value.height,zoom())});
 const style=computed(()=>({width:`${size.value.width}px`,height:`${size.value.height}px`,flexShrink:0}));
 function resetScroll(){if(container.value){container.value.scrollLeft=0;container.value.scrollTop=0}measure()}
 return {style,viewport,resetScroll,measure};
}
