<script setup lang="ts">
import {computed,ref,onBeforeUnmount,onMounted,onDeactivated,nextTick,watch} from 'vue';
import type {State,DeviceLink,CanvasPoint,NetworkSegment} from './model';
import {roleNames,roleColors} from './model';
import {useNetworkDrag} from './useNetworkDrag';
import {useTopologyViewport} from './topologyViewport';
import {compactTopologyLayout,nestedTopologyLayout} from './topologyLayout';
import {routeConnection} from './topologyRouting';
import {layersOf} from './networkLayers';import {layerDisplayVisible} from './networkLayerDisplay';
import {projectTopology,type TopologyNode} from './topologyProjection';
import {finiteRoomMatrix,transformRoomPointer} from './roomLayout';
const props=defineProps<{state:State;devices:State['devices'];networks:State['networks'];links:DeviceLink[];selectedDevice:string;selectedLink:string;zoom:number;networkId:string;highlightNetwork?:string;preview?:NetworkSegment;busy?:boolean;merge?:boolean;showLayers?:boolean;showEmptyLayers?:boolean;searchMatches?:string[]}>();
const emit=defineEmits<{addMembers:[networkId:string];network:[id:string];createNetwork:[parentId:string];editNetwork:[id:string];layers:[networkId:string];device:[id:string];link:[id:string];linkGroup:[ids:string[]];position:[deviceId:string,networkId:string,position:CanvasPoint];positions:[networkId:string,positions:{deviceId:string;position:CanvasPoint}[]];members:[deviceIds:string[],sourceId:string,targetId:string,copy:boolean];networkPosition:[id:string,position:CanvasPoint];connect:[sourceId:string,targetId:string];notice:[text:string];layer:[networkId:string,deviceIds:string[],layerId:string];'update:zoom':[zoom:number]}>();
const svg=ref<SVGSVGElement>(),canvas=ref<HTMLElement>(),canvasWidth=ref(1000),showLabels=ref(false),expanded=ref(new Set<string>());
const projection=computed(()=>projectTopology(props.devices,props.state.models,props.networks,props.state.networks,props.links,!!props.merge,expanded.value));
const connectionPick=ref<string[]>([]);
const expandedBuckets=computed(()=>props.merge?projection.value.buckets.filter(b=>b.devices.length>1&&expanded.value.has(b.key)):[]);
const groups=computed(()=>{
 const visible=props.networks.map(n=>n.id===props.preview?.id?props.preview:n);if(props.preview&&!visible.some(n=>n.id===props.preview?.id))visible.push(props.preview);
 const list=visible.map(n=>({id:n.id,name:n.name,color:n.color,network:n,items:projection.value.nodes.filter(d=>d.networkId===n.id)}));
 list.push({id:'unassigned',name:'未划分网络',color:'#8b9ba8',network:undefined as unknown as State['networks'][number],items:projection.value.nodes.filter(d=>!d.networkId)});
 return list.filter(g=>g.items.length||g.network);
});
const layerTones=['#6287b8','#4f958e','#9478b5','#af8b55','#72849c'];
const layerDashes=['8 5','2 4','10 4 2 4','5 4','12 5'];
function layerStyle(network:NetworkSegment|undefined,id:string){const index=Math.max(0,layersOf(network).findIndex(layer=>layer.id===id));return {color:layerTones[index%layerTones.length],dash:layerDashes[index%layerDashes.length]}}
const baseGeometry=computed(()=>{
 const layout=nestedTopologyLayout(groups.value.map(g=>({id:g.id,preserveEmpty:!!g.network,parentId:g.network?.parentId,positionSpace:g.network?.positionSpace,position:g.network?.position,layers:g.network?layersOf(g.network).filter(l=>props.showEmptyLayers||g.items.some(n=>n.layerId===l.id)):[{id:'all',name:''}],items:g.items.map(n=>({id:n.id,role:g.network?n.role:'server',layerId:g.network?n.layerId:'all',position:n.position}))})),canvasWidth.value,184,76,6);
 return {...layout,lanes:layout.lanes.map(l=>({...groups.value.find(g=>g.id===l.id)!,...l}))};
});
type Drag={node:TopologyNode;x:number;y:number;origin:CanvasPoint;start:CanvasPoint;screen:CanvasPoint;scroll:CanvasPoint;matrix:DOMMatrix;pointer:number;capture:Element;moved:boolean};
const pendingNode=ref<{id:string;x:number;y:number}|null>(null);
const pan=ref<{pointer:number;capture:Element;x:number;y:number;left:number;top:number}|null>(null);
const dragging=ref<Drag|null>(null),layerTarget=ref<{networkId:string;id:string;name:string}|null>(null),dropTarget=ref<{id:string;name:string}|null>(null),copyMembers=ref(false),wire=ref<{source:string;point:CanvasPoint;pointer:number;capture:Element;target:string}|null>(null),pickSource=ref('');
let suppressClick=false,frame=0,lastClient={x:0,y:0},externalTarget:Element|null=null;
const networkDrag=useNetworkDrag(svg,canvas,()=>baseGeometry.value.lanes,()=>!props.busy&&!props.preview&&!dragging.value&&!wire.value&&!pan.value,(id,position)=>emit('networkPosition',id,position),text=>emit('notice',text));
const geometry=computed(()=>{
 const base=baseGeometry.value;
 const nodes=new Map([...base.nodes].map(([id,p])=>{const d=dragging.value?.node.id===id?dragging.value:pendingNode.value?.id===id?pendingNode.value:null,o=networkDrag.offset(p.groupId),node=projection.value.nodes.find(n=>n.id===id)!;return [id,{...node,x:d?.x??p.x+o.x,y:d?.y??p.y+o.y}]}));
 const lanes=base.lanes.map(l=>{const o=networkDrag.offset(l.id);return {...l,x:l.x+o.x,y:l.y+o.y,nodeX:l.nodeX+o.x,nodeY:l.nodeY+o.y,childrenY:l.childrenY?l.childrenY+o.y:0}});for(const lane of [...lanes].reverse()){const parent=lanes.find(p=>p.id===lane.parentId);if(parent){parent.width=Math.max(parent.width,lane.x+lane.width-parent.x+20);parent.height=Math.max(parent.height,lane.y+lane.height-parent.y+20)}}return {...base,nodes,lanes};
});
const fitted=useTopologyViewport(canvas,()=>baseGeometry.value,()=>props.zoom,value=>emit('update:zoom',value),()=>!!dragging.value||!!networkDrag.active.value||!!wire.value||!!pan.value);
defineExpose({fit:fitted.fit,actual:fitted.actual,zoomBy:(delta:number)=>fitted.change(props.zoom+delta),revealNetwork});
let observer:ResizeObserver|undefined;
function revealNetwork(id:string){void nextTick(()=>{
 const el=canvas.value,header=el?.querySelector<SVGRectElement>(`[data-network-handle="${CSS.escape(id)}"]>rect`);if(!el||!header)return;
 const outer=el.getBoundingClientRect(),box=header.getBoundingClientRect(),right=box.left+Math.min(box.width,240);
 const dx=box.left<outer.left+16?box.left-outer.left-16:right>outer.right-16?right-outer.right+16:0,dy=box.top<outer.top+16?box.top-outer.top-16:box.bottom>outer.bottom-16?box.bottom-outer.bottom+16:0;
 el.scrollBy({left:dx,top:dy,behavior:'instant'});
})}
function revealSelected(){void nextTick(()=>requestAnimationFrame(()=>{const el=canvas.value,node=el?.querySelector<SVGGElement>('.topology-node[aria-pressed="true"]');if(!el||!node||dragging.value||wire.value)return;const outer=el.getBoundingClientRect(),box=node.getBoundingClientRect();const dx=box.left<outer.left+16?box.left-outer.left-16:box.right>outer.right-16?box.right-outer.right+16:0,dy=box.top<outer.top+16?box.top-outer.top-16:box.bottom>outer.bottom-16?box.bottom-outer.bottom+16:0;el.scrollBy(dx,dy)}))}
watch(()=>props.selectedDevice,revealSelected);
onMounted(()=>{observer=new ResizeObserver(()=>{if(!dragging.value&&!networkDrag.active.value&&!wire.value){canvasWidth.value=canvas.value?.closest('.topology-workspace')?.clientWidth||1000;revealSelected()}});const workspace=canvas.value?.closest('.topology-workspace');if(workspace)observer.observe(workspace)});
function point(x:number,y:number){const matrix=svg.value?.getScreenCTM()?.inverse();return matrix&&finiteRoomMatrix(matrix)?transformRoomPointer({x,y},matrix):null}
function inside(x:number,y:number){const b=canvas.value?.getBoundingClientRect();return !!b&&x>=b.left&&x<=b.right&&y>=b.top&&y<=b.bottom}
function dropAt(x:number,y:number){
 layerTarget.value=null;
 const element=document.elementFromPoint(x,y)?.closest('[data-network-drop]');externalTarget?.classList.remove('is-device-drop-target');externalTarget=null;
 const id=element?.getAttribute('data-network-drop');if(id){const n=props.state.networks.find(n=>n.id===id);if(n){externalTarget=element!;element!.classList.add('is-device-drop-target');return {id:n.id,name:n.name}}}
 if(!inside(x,y))return null;const p=point(x,y);if(!p)return null;
 const lane=[...geometry.value.lanes].sort((a,b)=>b.depth-a.depth).find(l=>p.x>=l.x&&p.x<=l.x+l.width&&p.y>=l.y&&p.y<=l.y+l.height);
 if(lane?.network&&dragging.value?.node.networkId===lane.id){const band=lane.bands.find(b=>p.y>=lane.nodeY+b.top&&p.y<=lane.nodeY+b.top+b.height);if(band&&band.id!==dragging.value.node.layerId)layerTarget.value={networkId:lane.id,id:band.id,name:band.name}}
 return lane?.network?{id:lane.network.id,name:lane.name}:null;
}
function start(e:PointerEvent,node:TopologyNode){
 if(e.button!==0||props.busy||props.preview||dragging.value||networkDrag.active.value||wire.value||pan.value)return;
 const n=geometry.value.nodes.get(node.id),matrix=svg.value?.getScreenCTM()?.inverse();if(!n||!matrix||!finiteRoomMatrix(matrix))return;
 const p=transformRoomPointer({x:e.clientX,y:e.clientY},matrix);if(!p)return;
 e.preventDefault();const capture=e.currentTarget as Element;capture.setPointerCapture(e.pointerId);
 dragging.value={node,x:n.x,y:n.y,origin:{x:n.x,y:n.y},start:p,screen:{x:e.clientX,y:e.clientY},scroll:{x:canvas.value?.scrollLeft||0,y:canvas.value?.scrollTop||0},matrix,pointer:e.pointerId,capture,moved:false};lastClient={x:e.clientX,y:e.clientY};suppressClick=false;
}
function updatePosition(){
 const a=dragging.value;if(!a)return;const p=transformRoomPointer(lastClient,a.matrix,{x:(canvas.value?.scrollLeft||0)-a.scroll.x,y:(canvas.value?.scrollTop||0)-a.scroll.y});if(!p)return;
 a.x=Math.max(0,p.x-a.start.x+a.origin.x);a.y=Math.max(0,p.y-a.start.y+a.origin.y);const target=dropAt(lastClient.x,lastClient.y);dropTarget.value=target&&target.id!==a.node.networkId?target:null;
}
function tick(){if(!dragging.value?.moved)return;const el=canvas.value;if(el){const b=el.getBoundingClientRect();if(lastClient.y>=b.top&&lastClient.y<=b.bottom){el.scrollLeft+=lastClient.x<b.left+35?-10:lastClient.x>b.right-35?10:0;el.scrollTop+=lastClient.y<b.top+35?-10:lastClient.y>b.bottom-35?10:0}}updatePosition();frame=requestAnimationFrame(tick)}
function startPan(e:PointerEvent){
 if(![0,1].includes(e.button)||props.busy||dragging.value||networkDrag.active.value||wire.value||(e.target as Element).closest('.topology-node,.topology-edge,.network-drag-handle'))return;
 const c=canvas.value;if(!c||(c.scrollWidth<=c.clientWidth&&c.scrollHeight<=c.clientHeight))return;e.preventDefault();const capture=e.currentTarget as Element;capture.setPointerCapture(e.pointerId);pan.value={pointer:e.pointerId,capture,x:e.clientX,y:e.clientY,left:c.scrollLeft,top:c.scrollTop};
}
function clearPan(){const p=pan.value;pan.value=null;if(p?.capture.hasPointerCapture(p.pointer))p.capture.releasePointerCapture(p.pointer)}
function move(e:PointerEvent){
 if(pan.value?.pointer===e.pointerId){if(canvas.value){canvas.value.scrollLeft=pan.value.left+pan.value.x-e.clientX;canvas.value.scrollTop=pan.value.top+pan.value.y-e.clientY}return}
 if(wire.value?.pointer===e.pointerId){const p=point(e.clientX,e.clientY);if(p){wire.value.point=p;wire.value.target=deviceAt(e.clientX,e.clientY)}return}
 const a=dragging.value;if(!a||e.pointerId!==a.pointer)return;lastClient={x:e.clientX,y:e.clientY};copyMembers.value=e.shiftKey;
 if(!a.moved&&Math.hypot(e.clientX-a.screen.x,e.clientY-a.screen.y)>5){a.moved=true;frame=requestAnimationFrame(tick)}if(a.moved)updatePosition();
}
function clear(){cancelAnimationFrame(frame);const a=dragging.value;dragging.value=null;dropTarget.value=null;layerTarget.value=null;externalTarget?.classList.remove('is-device-drop-target');externalTarget=null;if(a?.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer);canvasWidth.value=canvas.value?.closest('.topology-workspace')?.clientWidth||1000}
function cancel(){if(dragging.value?.moved)suppressClick=true;clear();clearWire();clearPan();pickSource.value=''}
function saveNodePosition(node:TopologyNode,p:CanvasPoint){
 const position={x:Math.max(0,Math.min(10000,p.x)),y:Math.max(0,Math.min(10000,p.y))};
 if(!node.grouped){emit('position',node.id,node.networkId,position);return}
 // Move all hidden members atomically while preserving their expanded offsets.
 const group=groups.value.find(g=>g.id===(node.networkId||'unassigned'))!;
 const devices=props.devices.filter(d=>node.deviceIds.includes(d.id));
 const expandedLayout=compactTopologyLayout([{id:group.id,items:devices.map(d=>({id:d.id,role:node.role,position:group.network?.devicePositions?.[d.id]||(!node.networkId?d.topologyPosition:undefined)}))}],Math.min(canvasWidth.value,680),184,76);
 const locals=expandedLayout.lanes[0].local,minX=Math.min(...[...locals.values()].map(p=>p.x)),minY=Math.min(...[...locals.values()].map(p=>p.y));
 const positions=devices.map(d=>({deviceId:d.id,position:{x:position.x+locals.get(d.id)!.x-minX,y:position.y+locals.get(d.id)!.y-minY}}));
 emit('positions',node.networkId,positions);
}
function up(e:PointerEvent){
 if(pan.value?.pointer===e.pointerId){clearPan();return}
 if(wire.value?.pointer===e.pointerId){const source=wire.value.source,target=deviceAt(e.clientX,e.clientY);clearWire();if(target&&target!==source)emit('connect',source,target);return}
 const a=dragging.value;if(!a||e.pointerId!==a.pointer)return;lastClient={x:e.clientX,y:e.clientY};updatePosition();const target=dropTarget.value,layer=layerTarget.value,valid=inside(e.clientX,e.clientY)||!!target;suppressClick=a.moved;
 const lane=baseGeometry.value.lanes.find(l=>l.id===(a.node.networkId||'unassigned'));clear();
 if(!a.moved||!valid||props.busy)return;
 pendingNode.value={id:a.node.id,x:a.x,y:a.y};
 if(layer)emit('layer',layer.networkId,a.node.deviceIds,layer.id);
 else if(target)emit('members',a.node.deviceIds,a.node.networkId,target.id,e.shiftKey);
 else if(lane&&(Math.abs(a.x-a.origin.x)>.1||Math.abs(a.y-a.origin.y)>.1))saveNodePosition(a.node,{x:a.x-lane.nodeX,y:a.y-lane.nodeY});
}
function expand(node:TopologyNode){
 if(geometry.value.nodes.size+node.deviceIds.length-1>300){emit('notice','展开后超过300台，请先按机房、网络或名称缩小范围');return}
 expanded.value=new Set([...expanded.value,node.groupKey]);
}
function collapse(key:string){const next=new Set(expanded.value);next.delete(key);expanded.value=next}
function clickNode(node:TopologyNode,event?:MouseEvent){if(event&&(event.ctrlKey||event.metaKey)&&!node.grouped){const ids=connectionPick.value.length?connectionPick.value:props.selectedDevice?[props.selectedDevice]:[];connectionPick.value=ids.includes(node.id)?ids.filter(id=>id!==node.id):[...ids.slice(-1),node.id];return}connectionPick.value=[];if(props.busy)return;if(suppressClick){suppressClick=false;return}if(pickSource.value){if(node.grouped){expand(node);return}const source=pickSource.value;pickSource.value='';if(source!==node.id)emit('connect',source,node.id);return}if(node.grouped)expand(node);else emit('device',node.id)}
function keyboard(e:KeyboardEvent,node:TopologyNode){if(e.key==='Enter'||e.key===' '){e.preventDefault();clickNode(node)}else if(e.altKey&&['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key)&&!props.busy){e.preventDefault();const n=geometry.value.nodes.get(node.id),lane=baseGeometry.value.lanes.find(l=>l.id===(node.networkId||'unassigned'));if(n&&lane)saveNodePosition(node,{x:n.x-lane.nodeX+(e.key==='ArrowLeft'?-24:e.key==='ArrowRight'?24:0),y:n.y-lane.nodeY+(e.key==='ArrowUp'?-24:e.key==='ArrowDown'?24:0)})}}
function deviceAt(x:number,y:number){if(!inside(x,y))return '';const id=document.elementFromPoint(x,y)?.closest('[data-network-node]')?.getAttribute('data-network-node');return id&&!projection.value.nodes.find(n=>n.id===id)?.grouped?id||'':''}
function startWire(e:PointerEvent,node:TopologyNode){if(props.busy||node.grouped||e.button!==0||dragging.value||networkDrag.active.value)return;e.stopPropagation();e.preventDefault();const p=point(e.clientX,e.clientY);if(!p)return;const capture=e.currentTarget as Element;capture.setPointerCapture(e.pointerId);wire.value={source:node.id,point:p,pointer:e.pointerId,capture,target:''}}
function clearWire(){const w=wire.value;wire.value=null;if(w?.capture.hasPointerCapture(w.pointer))w.capture.releasePointerCapture(w.pointer)}
const wirePath=computed(()=>{const w=wire.value,n=w&&geometry.value.nodes.get(w.source);return w&&n?`M${n.x+184} ${n.y+38} Q${(n.x+184+w.point.x)/2} ${Math.min(n.y+38,w.point.y)-30} ${w.point.x} ${w.point.y}`:''});
const dragHint=computed(()=>dragging.value?.moved?(layerTarget.value?`松开移到「${layerTarget.value.name}」 · ${dragging.value.node.deviceIds.length}台`:dropTarget.value?`松开${copyMembers.value?'加入':'移入'}「${dropTarget.value.name}」 · ${dragging.value.node.deviceIds.length}台${copyMembers.value?' · 保留原网络':''}`:inside(lastClient.x,lastClient.y)?'松开保存位置 · Esc取消':'移出画布，松开取消'):wire.value?'拖到另一台设备建立待确认连接 · Esc取消':pickSource.value?'点击目标设备建立连接 · Esc取消':'');
const active=(l:DeviceLink)=>l.id===props.selectedLink||l.sourceId===props.selectedDevice||l.targetId===props.selectedDevice;
const edges=computed(()=>{
 const boxes=[...geometry.value.nodes.values()].map(n=>({id:n.id,x:n.x,y:n.y,width:184,height:76}));const labels:{x:number;y:number;width:number;height:number}[]=[];
 return projection.value.edges.flatMap((e,index)=>{const a=boxes.find(n=>n.id===e.from),b=boxes.find(n=>n.id===e.to);if(!a||!b)return [];const route=routeConnection(a,b,boxes,index,labels,e.links.length>1?128:88,geometry.value.lanes.map(l=>({id:'header:'+l.id,x:l.x,y:l.y,width:l.width,height:64})));if(route.labelVisible)labels.push({x:route.label.x-(e.links.length>1?64:44),y:route.label.y-14,width:e.links.length>1?128:88,height:28});return [{...e,...route,selected:e.links.some(active),color:e.links.some(active)?'#2563eb':props.networks.find(n=>e.links.some(l=>l.networkIds.includes(n.id)))?.color||'#8095b2'}]});
});

function edgeName(edge:typeof edges.value[number]){const l=edge.links[0];return edge.links.length===1?`查看链路 ${props.state.devices.find(d=>d.id===l.sourceId)?.name} 到 ${props.state.devices.find(d=>d.id===l.targetId)?.name}`:`查看连接组 ${geometry.value.nodes.get(edge.from)?.name} 到 ${geometry.value.nodes.get(edge.to)?.name} ${edge.links.length}条`}
function chooseEdge(edge:typeof edges.value[number]){edge.links.length===1?emit('link',edge.links[0].id):emit('linkGroup',edge.links.map(l=>l.id))}
const short=(v:string,n=21)=>v.length>n?v.slice(0,Math.max(1,n-5))+'…'+v.slice(-4):v;
const locationOf=(node:TopologyNode)=>{const d=node.device;if(!d)return `${node.deviceIds.length} 台 · 点击展开，拖动整组`;const m=props.state.modules.find(m=>m.id===d.moduleId),r=props.state.racks.find(r=>r.id===d.rackId);return `${m?.point||m?.name||''} · ${r?`${r.name} U${d.startU}`:'待上架'}`};
const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')cancel()};onMounted(()=>window.addEventListener('keydown',escape));onBeforeUnmount(()=>{observer?.disconnect();window.removeEventListener('keydown',escape);cancel()});onDeactivated(cancel);
watch(()=>props.networkId,()=>{cancel()});watch(()=>props.merge,()=>{expanded.value=new Set();cancel()});watch(()=>props.state.revision,()=>{connectionPick.value=[];pendingNode.value=null;if(dragging.value||wire.value)cancel()});watch(()=>props.busy,value=>{if(!value)pendingNode.value=null});
</script>
<template>
 <div class="graph-display-options"><details class="graph-label-options"><summary>显示选项</summary><label><input v-model="showLabels" type="checkbox"/>全部连线标签</label></details></div>
 <div v-if="connectionPick.length" class="expanded-groups" aria-label="选择设备建立连接"><span>已选 {{connectionPick.length}} 台设备（Ctrl/⌘点击选择）</span><button :disabled="busy||connectionPick.length!==2" @click="emit('connect',connectionPick[0],connectionPick[1]);connectionPick=[]">连接这两台设备</button><button @click="connectionPick=[]">取消选择</button></div><div v-if="expandedBuckets.length" class="expanded-groups" aria-label="已展开设备组"><button v-for="b in expandedBuckets" :key="b.key" :disabled="busy||!!dragging||!!networkDrag.active.value" @click="collapse(b.key)">收起 {{b.name}}（{{b.devices.length}}台）</button><button @click="expanded=new Set()" :disabled="busy||!!dragging||!!networkDrag.active.value">全部收起</button></div>
 <div v-if="dragging?.moved&&networks.some(n=>n.id!==dragging?.node.networkId)" class="network-drop-strip" aria-label="拖入目标网络"><span>移入网络{{copyMembers?'（保留原网络）':''}}</span><span v-for="n in networks.filter(n=>n.id!==dragging?.node.networkId)" :key="n.id" :data-network-drop="n.id" :data-network-drop-option="n.id">{{n.name}}</span></div>
 <p v-if="networkDrag.invalid.value" class="error-banner" role="status">与其他网络重叠，松手将取消；请拖到空白位置。</p><p v-if="dragHint" class="topology-drag-hint" role="status">{{dragHint}}</p>
 <div ref="canvas" :class="['topology-canvas',{'summary-canvas':merge}]" tabindex="0" aria-label="网络拓扑画布，可横向和纵向滚动" @pointerdown="startPan" @pointermove="move($event);networkDrag.move($event)" @pointerup="up($event);networkDrag.up($event)" @pointercancel="cancel();networkDrag.cancel()"><div class="topology-stage">
  <svg ref="svg" :viewBox="`0 0 ${fitted.viewSize.value.width} ${fitted.viewSize.value.height}`" :style="fitted.style.value" role="group" aria-label="设备连接与网络分区图" :aria-busy="busy" :aria-disabled="busy" @click.capture="busy&&$event.stopImmediatePropagation()" @keydown.capture="busy&&($event.preventDefault(),$event.stopImmediatePropagation())">
   <defs><pattern id="topology-dots" width="18" height="18" patternUnits="userSpaceOnUse"><circle cx="2" cy="2" r=".8" fill="#d8e3e8"/></pattern></defs>
   <rect width="100%" height="100%" fill="transparent"/>
   <g v-for="lane in geometry.lanes" :key="lane.id" :data-topology-lane="lane.id" :data-parent-network="lane.parentId" :data-network-depth="lane.depth" :class="{'network-preview':lane.id===preview?.id,'network-selected':lane.id===networkId||lane.id===highlightNetwork,'network-related':!!selectedDevice&&!!lane.network?.deviceIds.includes(selectedDevice),'network-dragging':networkDrag.active.value?.id===lane.id&&networkDrag.dragging.value,'network-drop-target':dropTarget?.id===lane.id,'network-drop-invalid':networkDrag.active.value?.id===lane.id&&networkDrag.invalid.value}">
    <rect :x="lane.x" :y="lane.y" :width="lane.width" :height="lane.height" rx="12" :fill="lane.color+'07'" :stroke="lane.color+'55'"/>
    <g :class="{'network-drag-handle':!!lane.network}" :data-network-handle="lane.network?.id" :role="lane.network?'button':undefined" :tabindex="lane.network?0:undefined" :aria-label="lane.network?'选择网络 '+lane.name:undefined" :aria-pressed="lane.network?lane.id===networkId:undefined" aria-description="点击标题查看网络详情；拖动标题移动网络，Alt加方向键微调" @pointerdown="networkDrag.down($event,lane.network?.id||'')" @click="lane.id!=='network-preview'&&!networkDrag.consumeClick()&&lane.network&&emit('network',lane.id)" @keydown="networkDrag.key($event,lane.network?.id||'')" @keydown.enter.prevent="lane.network&&emit('network',lane.id)" @keydown.space.prevent="lane.network&&emit('network',lane.id)">
     <title>{{lane.name}} · 点击标题查看详情 · 拖动标题移动整个网络 · Alt + 方向键微调 · Esc取消</title><rect :x="lane.x" :y="lane.y" :width="lane.width" height="58" rx="10" :fill="lane.color+'13'"/><text :x="lane.x+16" :y="lane.y+25" :fill="lane.color" font-size="16" font-weight="650">{{short(lane.name,Math.max(6,Math.floor((lane.width-130)/16)))}}</text><text :x="lane.x+16" :y="lane.y+46" fill="#61758e" font-size="10">{{short(lane.items.reduce((n,item)=>n+item.deviceIds.length,0)+' 台直属 · '+lane.childCount+' 个内部网络 · '+(lane.network?.parentId?'上级：'+(state.networks.find(n=>n.id===lane.network?.parentId)?.name||'')+' · ':'')+(lane.network?.cidr||'网段未填写'),Math.floor((lane.width-32)/6))}}</text><g v-if="lane.network&&lane.id!=='network-preview'" role="button" tabindex="0" :aria-label="'修改网络 '+lane.name" @pointerdown.stop @click.stop="emit('editNetwork',lane.id)" @keydown.enter.stop.prevent="emit('editNetwork',lane.id)" @keydown.space.stop.prevent="emit('editNetwork',lane.id)"><title>修改网络名称与地址</title><rect :x="lane.x+lane.width-38" :y="lane.y+9" width="30" height="30" rx="4" fill="white" stroke="#cbd9eb"/><text :x="lane.x+lane.width-23" :y="lane.y+28" text-anchor="middle" fill="#2452a0" font-size="16">⋯</text></g>
<g v-if="lane.network&&lane.id!=='network-preview'" role="button" tabindex="0" :aria-label="'在'+lane.name+'内新建网络'" @pointerdown.stop @click.stop="emit('createNetwork',lane.id)" @keydown.enter.stop.prevent="emit('createNetwork',lane.id)" @keydown.space.stop.prevent="emit('createNetwork',lane.id)"><title>在此新建网络</title><rect :x="lane.x+lane.width-106" :y="lane.y+9" width="30" height="30" rx="4" fill="white" stroke="#cbd9eb"/><text :x="lane.x+lane.width-91" :y="lane.y+29" text-anchor="middle" fill="#2452a0" font-size="18">＋</text></g><g v-if="lane.network&&lane.id!=='network-preview'" role="button" tabindex="0" :aria-label="'为'+lane.name+'添加设备'" @pointerdown.stop @click.stop="emit('addMembers',lane.id)" @keydown.enter.stop.prevent="emit('addMembers',lane.id)" @keydown.space.stop.prevent="emit('addMembers',lane.id)"><title>添加设备</title><rect :x="lane.x+lane.width-72" :y="lane.y+9" width="30" height="30" rx="4" fill="white" stroke="#cbd9eb"/><text :x="lane.x+lane.width-57" :y="lane.y+28" text-anchor="middle" fill="#2452a0" font-size="13">设备</text></g>
    </g><text v-if="lane.childCount" :x="lane.x+20" :y="lane.childrenY-10" font-size="11" fill="#61758e">内部网络</text><g v-if="!lane.items.length&&!lane.childCount&&!lane.bands.some(b=>!!b.name)&&lane.id!=='network-preview'" class="topology-add-members" @pointerdown.stop role="button" tabindex="0" aria-label="为当前网络添加设备" @click="emit('addMembers',lane.id)" @keydown.enter.prevent="emit('addMembers',lane.id)" @keydown.space.prevent="emit('addMembers',lane.id)"><rect :x="lane.x+16" :y="lane.y+70" :width="lane.width-32" height="44" rx="6" fill="#eff6ff" stroke="#93b4f8"/><text :x="lane.x+lane.width/2" :y="lane.y+88" text-anchor="middle" font-size="14" font-weight="600" fill="#1d4ed8">添加设备</text><text :x="lane.x+lane.width/2" :y="lane.y+106" text-anchor="middle" font-size="11" fill="#526984">也可拖入未分配设备</text></g><g v-for="band in lane.bands.filter(b=>!!b.name)" :key="band.id" class="topology-layer-band" pointer-events="none">
     <rect :x="lane.nodeX-8" :y="lane.nodeY+band.top" :width="lane.ownWidth-24" :height="band.height-4" rx="6" :fill="layerTarget?.networkId===lane.id&&layerTarget?.id===band.id?'#dbeafe':layerDisplayVisible(lane.network)?layerStyle(lane.network,band.id).color+'0a':'transparent'" :stroke="layerTarget?.networkId===lane.id&&layerTarget?.id===band.id?'#2563eb':'none'" :data-layer-target="band.id" :data-layer-network="lane.id"/>
     <line v-if="layerDisplayVisible(lane.network)" :data-layer-separator="band.id" :x1="lane.nodeX-8" :x2="lane.nodeX+lane.ownWidth-32" :y1="lane.nodeY+band.top" :y2="lane.nodeY+band.top" :stroke="layerStyle(lane.network,band.id).color" :stroke-dasharray="layerStyle(lane.network,band.id).dash" stroke-width="1.25"/>
     <text v-if="layerDisplayVisible(lane.network)" :x="lane.nodeX" :y="lane.nodeY+band.y" fill="#43566f" font-size="12" font-weight="600" class="topology-role-label">{{band.name}}{{showLayers&&lane.network?' · '+lane.items.filter(n=>n.layerId===band.id).reduce((sum,n)=>sum+n.deviceIds.length,0)+'台':''}}</text>
    </g>
   </g>
   <g v-for="edge in edges" :key="edge.id" :opacity="(selectedDevice||selectedLink)&&!edge.selected?0.18:1" @click="chooseEdge(edge)" class="topology-edge">
    <title>{{edge.links.length}} 条连接 · {{edge.links.filter(l=>l.status==='pending').length}} 条待确认 · {{edge.links.filter(l=>l.kind==='physical').length}} 条物理链路</title>
    <path :d="edge.path" fill="none" stroke="transparent" stroke-width="15" pointer-events="stroke"/><path :d="edge.path" fill="none" :stroke="edge.color" :stroke-width="edge.selected?3:1.7" :stroke-dasharray="edge.links.every(l=>l.status==='pending')?'5 5':edge.links.every(l=>l.kind==='logical')?'2 4':undefined"/>
    <g :data-link-badge="edge.id" :transform="`translate(${edge.label.x},${edge.label.y})`" :class="['topology-edge-label',{'compact-label':!edge.labelVisible||!showLabels&&!edge.selected}]" role="button" tabindex="0" :aria-label="edgeName(edge)" :aria-pressed="edge.links.some(l=>l.id===selectedLink)" @keydown.enter.prevent="chooseEdge(edge)" @keydown.space.prevent="chooseEdge(edge)">
     <circle v-if="!edge.labelVisible" r="4" :fill="edge.color"/><rect :x="edge.links.length>1?-64:-44" y="-14" :width="edge.links.length>1?128:88" height="28" rx="6" fill="white" :stroke="edge.color" :stroke-width="edge.selected?2:1"/><text y="4" text-anchor="middle" font-size="10" :fill="edge.color" pointer-events="none">{{edge.links.length>1?edge.links.length+'条 · '+edge.links.filter(l=>l.status==='pending').length+'待确认':edge.links[0].speedGbps?edge.links[0].speedGbps+' Gbps':edge.links[0].status==='pending'?'待确认关系':edge.links[0].kind==='physical'?'物理连接':'逻辑关系'}}</text>
    </g>
   </g>
   <path v-if="wirePath" :d="wirePath" class="connection-draft" fill="none" stroke="#2563eb" stroke-width="2.5" stroke-dasharray="6 4" pointer-events="none"/>
   <g v-for="node in [...geometry.nodes.values()]" :key="node.id" :transform="`translate(${node.x},${node.y})`" role="button" tabindex="0" :aria-label="node.grouped?`展开 ${node.name} ${node.deviceIds.length}台`:`查看拓扑设备 ${node.name}`" :class="['topology-node',{'is-dragging':dragging?.node.id===node.id&&dragging.moved,'is-grouped':node.grouped,'is-link-target':wire?.target===node.id,'is-search-match':searchMatches&&node.deviceIds.some(id=>searchMatches!.includes(id)),'is-search-neighbor':searchMatches&&!node.deviceIds.some(id=>searchMatches!.includes(id))}]" :aria-pressed="node.deviceIds.includes(selectedDevice)||connectionPick.includes(node.id)" :data-network-node="node.id" :data-group-size="node.deviceIds.length" @pointerdown="start($event,node)" @click="clickNode(node,$event)" @keydown="keyboard($event,node)">
    <title>{{node.name}} · {{locationOf(node)}} · {{node.networkIds.map(id=>state.networks.find(n=>n.id===id)?.name).join(' / ')||'未划分网络'}}</title>
    <rect v-if="node.grouped" x="5" y="5" width="184" height="76" rx="7" fill="#eef4fc" stroke="#bacde8"/><rect width="184" height="76" rx="7" fill="white" :stroke="node.deviceIds.includes(selectedDevice)?'#1d4ed8':'#cddce3'" :stroke-width="node.deviceIds.includes(selectedDevice)?2.5:1"/><rect x="0" y="15" width="3" height="30" :fill="roleColors[node.role]"/>
    <text x="12" y="23" font-size="14" fill="#243b53" font-weight="600">{{short(node.name,12)}}</text><text x="12" y="43" font-size="12" fill="#526984">{{roleNames[node.role]}}{{node.grouped?' · '+node.deviceIds.length+' 台':''}}</text>
    <text v-if="node.grouped" x="12" y="64" font-size="12" fill="#1d4ed8">{{projection.internal.get(node.id)?'组内 '+projection.internal.get(node.id)+' 条连接 · ':''}}展开 ▾</text>
    <g v-if="!node.grouped" class="link-drag-handle" role="button" tabindex="0" :aria-label="'从 '+node.name+' 连线'" :data-link-source="node.id" @pointerdown.stop="startWire($event,node)" @click.stop @keydown.enter.stop.prevent="pickSource=node.id" @keydown.space.stop.prevent="pickSource=node.id"><title>拖动到另一台设备建立连接</title><circle cx="184" cy="38" r="9" fill="white" stroke="#2563eb" stroke-width="1.5"/><path d="M180 38h8M184 34v8" stroke="#2563eb"/></g>
   </g>
  </svg>
 </div></div>
</template>
