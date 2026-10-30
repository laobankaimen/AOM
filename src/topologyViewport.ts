import {computed,nextTick,onMounted,onBeforeUnmount,ref,watch,type Ref} from 'vue';
export const canvasPadding=40;
export function topologyFitZoom(width:number,height:number,viewportWidth:number,viewportHeight:number){return Math.max(10,Math.min(200,Math.floor(Math.min(Math.max(1,viewportWidth-canvasPadding*2)/Math.max(1,width),Math.max(1,viewportHeight-canvasPadding*2)/Math.max(1,height))*100)))}
export function anchoredScroll(scroll:number,anchor:number,before:number,after:number){return (scroll+anchor-canvasPadding)/before*after+canvasPadding-anchor}
export function useTopologyViewport(canvas:Ref<HTMLElement|undefined>,bounds:()=>{width:number;height:number},zoom:()=>number,update:(zoom:number)=>void,locked:()=>boolean){
 const fitting=ref(false);let observer:ResizeObserver|undefined;
 const style=computed(()=>({width:bounds().width*zoom()/100+'px',height:bounds().height*zoom()/100+'px',flexShrink:0}));
 async function change(next:number,x?:number,y?:number){if(locked())return;const el=canvas.value;if(!el)return;fitting.value=false;const old=zoom()/100,value=Math.max(10,Math.min(400,next));const ax=x??el.clientWidth/2,ay=y??el.clientHeight/2,left=el.scrollLeft,top=el.scrollTop;update(value);await nextTick();el.scrollLeft=anchoredScroll(left,ax,old,value/100);el.scrollTop=anchoredScroll(top,ay,old,value/100)}
 async function fit(){if(locked())return;const el=canvas.value;if(!el||!el.clientWidth||!el.clientHeight)return;fitting.value=true;update(topologyFitZoom(bounds().width,bounds().height,el.clientWidth,el.clientHeight));await nextTick();el.scrollLeft=0;el.scrollTop=0}
 async function actual(){if(locked())return;fitting.value=false;update(100);await nextTick();if(canvas.value){canvas.value.scrollLeft=0;canvas.value.scrollTop=0}}
 function wheel(e:WheelEvent){if(locked())return;e.preventDefault();const el=canvas.value;if(!el)return;if(e.ctrlKey||e.metaKey){const box=el.getBoundingClientRect();void change(Math.round(zoom()*Math.exp(-e.deltaY*.002)),e.clientX-box.left,e.clientY-box.top)}else{el.scrollLeft+=e.shiftKey?e.deltaY:e.deltaX;el.scrollTop+=e.shiftKey?0:e.deltaY}}
 onMounted(()=>{if(canvas.value){canvas.value.addEventListener('wheel',wheel,{passive:false});observer=new ResizeObserver(()=>{if(fitting.value&&!locked())void fit()});observer.observe(canvas.value)}});
 onBeforeUnmount(()=>{observer?.disconnect();canvas.value?.removeEventListener('wheel',wheel)});
 watch(()=>[bounds().width,bounds().height],()=>{if(fitting.value&&!locked())void fit()});
 return {style,fit,actual,change,fitting};
}
