<script setup lang="ts">
import {computed,ref,shallowRef,watch,onMounted,onBeforeUnmount,nextTick} from 'vue';import {Move,ArrowRight,Minus,Plus,Maximize2,RotateCcw,RotateCw,X} from 'lucide-vue-next';
import {useDiagramViewport} from './diagramViewport';
import DiagramExport from './DiagramExport.vue';
import ActionMenu from './ActionMenu.vue';
import type {State,CanvasPoint} from './model';import {infraColors} from './model';import {rowLayout,layoutWidth} from './layout';
import {projectRoom,roomMove,finiteRoomPoint,finiteRoomMatrix,transformRoomPointer,snapRoom,cuboidFaces,defaultRoomPositions,roomOverlaps,moduleSize,rotateRoomRect} from './roomLayout';
const props=defineProps<{state:State;siteId:string;busy?:boolean}>();const emit=defineEmits<{open:[moduleId:string];position:[moduleId:string,position:CanvasPoint|null,rotation?:number]}>();
const warning=ref(''),svg=ref<SVGSVGElement>(),selected=ref(''),isometric=ref(true),zoom=ref(100),scroller=ref<HTMLDivElement>(),snapOnDrop=ref(false),overlapping=ref(false),dragging=ref(false);
type Drag={id:string;pointer:number;capture:SVGGElement;start:CanvasPoint;origin:CanvasPoint;position:CanvasPoint;screenX:number;screenY:number;moved:boolean;matrix:DOMMatrix;scrollLeft:number;scrollTop:number};
const active=shallowRef<Drag|null>(null);let skipClick=false,frame=0,last:{x:number;y:number}|null=null;
const pending=shallowRef<{id:string;position:CanvasPoint;rotation:number}|null>(null);
const modules=computed(()=>props.state.modules.filter(m=>m.siteId===props.siteId));
const layouts=computed(()=>modules.value.map(m=>{const rows=['L',...(m.type==='double'?['R']:[])].map(side=>({side,tiles:rowLayout(m,props.state.racks,props.state.infra,side)}));const baseWidth=Math.max(160,...rows.map(r=>layoutWidth(r.tiles)*.24+44)),baseDepth=m.type==='double'?180:100;const rotation=pending.value?.id===m.id?pending.value.rotation:m.roomRotation||0;return {module:m,rows,baseWidth,baseDepth,rotation,...moduleSize(baseWidth,baseDepth,rotation)}}));
// Unpositioned neighbours keep their screen locations while one module moves.
const defaultCache=shallowRef(new Map<string,CanvasPoint>());
const positions=computed(()=>{const result=defaultRoomPositions(modules.value.map(m=>({...m,roomPosition:pending.value?.id===m.id?pending.value.position:m.roomPosition||defaultCache.value.get(m.id)})),new Map(layouts.value.map(l=>[l.module.id,l.baseWidth])));return result});
watch(positions,value=>{let changed=false;const next=new Map(defaultCache.value);for(const [id,p]of value){if(!next.has(id)&&!modules.value.find(m=>m.id===id)?.roomPosition&&pending.value?.id!==id){next.set(id,{...p});changed=true}}if(changed)defaultCache.value=next},{immediate:true});
const naturalWorld=computed(()=>({width:Math.max(800,...layouts.value.map(l=>positions.value.get(l.module.id)!.x+l.width+160)),depth:Math.max(540,...layouts.value.map(l=>positions.value.get(l.module.id)!.y+l.depth+140))}));
const world=ref({width:800,depth:540});
const bounds=ref({x:-75,y:-100,width:950,height:750});
const fitted=useDiagramViewport(scroller,()=>bounds.value,()=>zoom.value,()=>!!active.value);
const grid=computed(()=>{const lines:string[]=[];const point=(x:number,y:number)=>{const p=projectRoom({x,y},isometric.value);return `${p.x},${p.y}`};const step=Math.max(50,Math.ceil(Math.max(world.value.width,world.value.depth)/100/10)*10);for(let x=0;x<=world.value.width;x+=step)lines.push(`M${point(x,0)} L${point(x,world.value.depth)}`);for(let y=0;y<=world.value.depth;y+=step)lines.push(`M${point(0,y)} L${point(world.value.width,y)}`);return lines});
const floor=computed(()=>cuboidFaces(0,0,world.value.width,world.value.depth,0,isometric.value).top);
// Geometry is local and cached. Pointer frames update only the outer group transform.
const renderModules=computed(()=>layouts.value.map(l=>{const p=positions.value.get(l.module.id)!;const footprint=cuboidFaces(0,0,l.width,l.depth,5,isometric.value);const tiles=l.rows.flatMap((row,ri)=>row.tiles.map(tile=>{const r=rotateRoomRect(20+tile.x*.24,20+ri*80,tile.width*.24,42,l.baseWidth,l.baseDepth,l.rotation),h=isometric.value?42:0;return {...tile,faces:cuboidFaces(r.x,r.y,r.width,r.depth,h,isometric.value),center:projectRoom({x:r.x+r.width/2,y:r.y+r.depth/2},isometric.value,h),sort:r.x+r.y}})).sort((a,b)=>a.sort-b.sort);const label=projectRoom({x:l.width/2,y:l.depth/2},isometric.value);const aisle=rotateRoomRect(14,67,l.baseWidth-28,27,l.baseWidth,l.baseDepth,l.rotation);return {...l,p,footprint,tiles,origin:projectRoom(p,isometric.value),label:{x:label.x,y:projectRoom({x:l.width,y:l.depth},isometric.value).y+19},aisle:l.module.type==='double'?cuboidFaces(aisle.x,aisle.y,aisle.width,aisle.depth,6,isometric.value).top:''}}).sort((a,b)=>a.p.x+a.p.y-b.p.x-b.p.y));
function contentBounds(){
 const points:{x:number;y:number}[]=[];
 for(const item of renderModules.value){
  for(const shape of [item.footprint,...item.tiles.map(t=>t.faces)])for(const face of Object.values(shape))for(const pair of face.split(' ')){const [x,y]=pair.split(',').map(Number);points.push({x:x+item.origin.x,y:y+item.origin.y})}
  const half=Math.max(100,item.module.name.length*6);points.push({x:item.origin.x+item.label.x-half,y:item.origin.y+item.label.y-13},{x:item.origin.x+item.label.x+half,y:item.origin.y+item.label.y+18});
 }
 if(!points.length)return {x:0,y:0,width:800,height:540};
 const x=Math.min(...points.map(p=>p.x))-40,y=Math.min(...points.map(p=>p.y))-55;
 return {x,y,width:Math.max(...points.map(p=>p.x))-x+40,height:Math.max(...points.map(p=>p.y))-y+40};
}
const current=computed(()=>modules.value.find(m=>m.id===selected.value));
const blocked=computed(()=>!!props.busy||!!pending.value);
function collision(id:string,position:CanvasPoint,rotation?:number){const l=layouts.value.find(l=>l.module.id===id)!;const size=rotation===undefined?l:moduleSize(l.baseWidth,l.baseDepth,rotation);return layouts.value.some(other=>other.module.id!==id&&roomOverlaps(position,size.width,size.depth,positions.value.get(other.module.id)!,other.width,other.depth))}
function localPoint(x:number,y:number,a:Drag){const el=scroller.value;return transformRoomPointer({x,y},a.matrix,{x:el?el.scrollLeft-a.scrollLeft:0,y:el?el.scrollTop-a.scrollTop:0})}
function down(e:PointerEvent,id:string){
 if(e.button!==0||blocked.value)return;
 const screen=svg.value?.getScreenCTM();if(!screen)return;
 const matrix=screen.inverse();
 const start=transformRoomPointer({x:e.clientX,y:e.clientY},matrix);
 const origin=finiteRoomPoint(positions.value.get(id));
 if(!finiteRoomMatrix(matrix)||!start||!origin){warning.value='画布坐标暂不可用，请点击适应位置图后重试';return}
 e.preventDefault();const capture=e.currentTarget as SVGGElement;capture.setPointerCapture(e.pointerId);
 warning.value='';selected.value=id;skipClick=false;
 active.value={id,pointer:e.pointerId,capture,start,origin,position:{x:origin.x,y:origin.y},screenX:e.clientX,screenY:e.clientY,moved:false,matrix,scrollLeft:scroller.value?.scrollLeft||0,scrollTop:scroller.value?.scrollTop||0}
}
function invalidDrag(){warning.value='画布坐标暂不可用，已取消本次移动';cancel()}
function paint(){
 frame=0;const a=active.value;if(!a||!last)return;
 const p=localPoint(last.x,last.y,a);if(!p){invalidDrag();return}
 if(!a.moved&&Math.hypot(last.x-a.screenX,last.y-a.screenY)>4){a.moved=true;dragging.value=true}
 if(!a.moved)return;
 const position=finiteRoomPoint(roomMove(a.origin,a.start,p,isometric.value,false));
 const projected=position&&finiteRoomPoint(projectRoom(position,isometric.value));
 if(!position||!projected){invalidDrag();return}
 a.position=position;
 a.capture.setAttribute('transform',`translate(${projected.x},${projected.y})`);
 const invalid=collision(a.id,position);if(overlapping.value!==invalid)overlapping.value=invalid
}
function move(e:PointerEvent){if(!active.value||e.pointerId!==active.value.pointer)return;last={x:e.clientX,y:e.clientY};if(!frame)frame=requestAnimationFrame(paint)}
function clear(restore=true){
 cancelAnimationFrame(frame);frame=0;const a=active.value;active.value=null;dragging.value=false;overlapping.value=false;last=null;
 if(a){if(restore){const p=finiteRoomPoint(projectRoom(a.origin,isometric.value));if(p)a.capture.setAttribute('transform',`translate(${p.x},${p.y})`)}if(a.capture.hasPointerCapture(a.pointer))a.capture.releasePointerCapture(a.pointer)}
}
function cancel(){skipClick=!!active.value?.moved;clear()}
function persist(id:string,position:CanvasPoint,rotation:number){if(!finiteRoomPoint(position)){warning.value='位置无效，未保存';return}pending.value={id,position,rotation};emit('position',id,position,rotation)}
function up(e:PointerEvent){
 const a=active.value;if(!a||e.pointerId!==a.pointer)return;
 last={x:e.clientX,y:e.clientY};cancelAnimationFrame(frame);paint();if(active.value!==a)return;
 const rect=scroller.value?.getBoundingClientRect();const inside=rect&&e.clientX>=rect.left&&e.clientX<=rect.right&&e.clientY>=rect.top&&e.clientY<=rect.bottom;
 const p=finiteRoomPoint(snapOnDrop.value?snapRoom(a.position):{x:Math.round(a.position.x*100)/100,y:Math.round(a.position.y*100)/100});
 if(!p){invalidDrag();return}
 const valid=inside&&!collision(a.id,p);skipClick=a.moved;
 if(a.moved&&!valid)warning.value=inside?'位置重叠，已取消移动':'已移出画布，取消移动';
 const moved=a.moved,rotation=layouts.value.find(l=>l.module.id===a.id)!.rotation;
 clear(!valid);
 if(moved&&valid&&(p.x!==a.origin.x||p.y!==a.origin.y))persist(a.id,p,rotation)
}
function click(id:string){if(skipClick){skipClick=false;return}selected.value=id}
function rotate(){if(!current.value||blocked.value||active.value)return;const id=current.value.id,rotation=((current.value.roomRotation||0)+90)%360,p=positions.value.get(id)!;if(collision(id,p,rotation)){warning.value='旋转后与其他模块重叠，请先移动到空闲位置';return}warning.value='';persist(id,{...p},rotation)}
function key(e:KeyboardEvent,id:string){if(e.key==='Enter'){e.preventDefault();emit('open',id)}else if(e.key.toLowerCase()==='r'&&!e.ctrlKey&&!e.metaKey){e.preventDefault();selected.value=id;rotate()}else if(e.altKey&&['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key)&&!blocked.value){e.preventDefault();const p=positions.value.get(id)!;const next={x:Math.max(0,Math.min(10000,p.x+(e.key==='ArrowRight'?10:e.key==='ArrowLeft'?-10:0))),y:Math.max(0,Math.min(10000,p.y+(e.key==='ArrowDown'?10:e.key==='ArrowUp'?-10:0)))};selected.value=id;if(collision(id,next)){warning.value='位置重叠，未移动';return}warning.value='';persist(id,next,current.value?.roomRotation||0)}}
function fit(){if(active.value||blocked.value)return;world.value={...naturalWorld.value};bounds.value=contentBounds();zoom.value=100;nextTick(fitted.resetScroll)}
function reset(){if(current.value&&!blocked.value){defaultCache.value=new Map([...defaultCache.value].filter(([id])=>id!==current.value!.id));emit('position',current.value.id,null,0)}}
const escape=(e:KeyboardEvent)=>{if(e.key==='Escape')cancel()};onMounted(()=>window.addEventListener('keydown',escape));onBeforeUnmount(()=>{window.removeEventListener('keydown',escape);clear()});
watch(()=>props.siteId,()=>{cancel();pending.value=null;defaultCache.value=new Map();selected.value='';warning.value='';zoom.value=100;nextTick(()=>world.value={...naturalWorld.value})});
watch(()=>modules.value.map(m=>m.id).join(','),()=>{if(!active.value&&!pending.value){world.value={...naturalWorld.value};bounds.value=contentBounds()}},{immediate:true});
watch(isometric,()=>{bounds.value=contentBounds();zoom.value=100;nextTick(fitted.resetScroll)});
watch(()=>props.state.revision,()=>{if(active.value)cancel();pending.value=null});
watch(()=>props.busy,value=>{if(!value&&pending.value){pending.value=null}});
</script>
<template><section class="room-position-panel" aria-label="机房微模块布局"><div class="room-position-toolbar"><div class="view-switch"><button :class="{active:isometric}" :disabled="!!active||blocked" @click="isometric=true">立体示意</button><button :class="{active:!isometric}" :disabled="!!active||blocked" @click="isometric=false">平面布局</button></div><span class="room-drag-hint"><Move :size="14"/>拖动移动 · R旋转</span><ActionMenu label="布局选项"><label class="room-snap-toggle"><input v-model="snapOnDrop" type="checkbox" :disabled="!!active||blocked"/>松手吸附</label></ActionMenu><DiagramExport :title="(state.sites.find(s=>s.id===siteId)?.name||'机房')+(isometric?'立体布局图':'平面布局图')" :scope="(state.projects.find(p=>p.id===state.sites.find(s=>s.id===siteId)?.projectId)?.name||'')+' / '+(state.sites.find(s=>s.id===siteId)?.name||'')+' · '+modules.length+' 个微模块 · 位置示意'" selector=".room-position-svg" :disabled="blocked||!!active||!modules.length"/><div class="zoom-controls"><button aria-label="缩小机房位置图" :disabled="zoom<=25||!!active||blocked" @click="zoom-=25"><Minus :size="14"/></button><span>{{zoom}}%</span><button aria-label="放大机房位置图" :disabled="zoom>=400||!!active||blocked" @click="zoom+=25"><Plus :size="14"/></button><button aria-label="适应机房位置图" :disabled="!!active||blocked" @click="fit"><Maximize2 :size="14"/>适应画布</button></div></div>
 <div v-if="!modules.length" class="empty-state"><h3>此机房暂无微模块</h3><p>通过上方“导入初始化”添加模块。</p></div><div v-else ref="scroller" class="room-position-scroll"><svg ref="svg" class="room-position-svg" :viewBox="`${bounds.x} ${bounds.y} ${bounds.width} ${bounds.height}`" :style="fitted.style.value" role="group" aria-label="机房微模块位置图" @pointermove="move" @pointerup="up" @pointercancel="cancel" @lostpointercapture="active&&cancel()">
  <defs><linearGradient id="room-floor" x1="0" x2="1" y1="0" y2="1"><stop stop-color="#f4f9fb"/><stop offset="1" stop-color="#e3edf1"/></linearGradient><filter id="room-soft-shadow" x="-50%" y="-50%" width="200%" height="220%"><feDropShadow dx="0" dy="9" stdDeviation="6" flood-color="#2d5264" flood-opacity=".12"/></filter></defs>
  <polygon :points="floor" fill="url(#room-floor)" stroke="#c7d8e0" stroke-width="2"/><path v-for="(line,i) in grid" :key="i" :d="line" fill="none" stroke="#d4e2e8" stroke-width=".7"/>
  <g v-for="item in renderModules" :key="item.module.id" class="room-position-module" :class="{'is-selected':selected===item.module.id,'is-dragging':active?.id===item.module.id&&dragging}" role="button" tabindex="0" :aria-label="`选择布局模块 ${item.module.name}`" :aria-pressed="selected===item.module.id" :data-room-module="item.module.id" :data-room-rotation="item.rotation" :transform="`translate(${item.origin.x},${item.origin.y})`" @pointerdown="down($event,item.module.id)" @click="click(item.module.id)" @dblclick="!skipClick&&emit('open',item.module.id)" @keydown="key($event,item.module.id)">
   <title>{{item.module.name}} · {{item.module.type==='double'?'双排':'单排'}} · {{item.module.rackLeft+item.module.rackRight}}柜</title>
   <g :filter="active?.id===item.module.id?undefined:'url(#room-soft-shadow)'"><polygon :points="item.footprint.front" fill="#a7c7bc"/><polygon :points="item.footprint.right" fill="#bad5cb"/><polygon :points="item.footprint.top" :fill="active?.id===item.module.id&&overlapping?'#ffdfd7':selected===item.module.id?'#d9eee4':'#eaf2f3'" :stroke="active?.id===item.module.id&&overlapping?'#d87966':selected===item.module.id?'#42a583':'#bfd4dc'" :stroke-width="selected===item.module.id?2.4:1"/></g>
   <polygon v-if="item.aisle" :points="item.aisle" fill="#a7dcc7" fill-opacity=".65"/>
   <g v-for="tile in item.tiles" :key="tile.id" pointer-events="none"><polygon :points="tile.faces.front" :fill="tile.rack?'#527080':infraColors[tile.kind]"/><polygon :points="tile.faces.right" :fill="tile.rack?'#344f60':infraColors[tile.kind]" :opacity="tile.rack?1:.8"/><polygon :points="tile.faces.top" :fill="tile.rack?'#e0eaf0':infraColors[tile.kind]" stroke="#ffffff" stroke-width=".6"/><path v-if="tile.rack&&isometric" :d="`M ${tile.center.x-5} ${tile.center.y} h10`" stroke="#a2b9c6" stroke-width="1.5"/><circle v-if="tile.integrated.length" :cx="tile.center.x" :cy="tile.center.y-3" r="2.4" fill="#43b58c"/></g>
   <g :transform="`translate(${item.label.x},${item.label.y})`" class="room-module-caption"><rect :x="-Math.max(100,item.module.name.length*6)" y="-13" :width="Math.max(200,item.module.name.length*12)" height="31" rx="6" fill="white" fill-opacity=".95" stroke="#dbe7ec"/><text text-anchor="middle" y="6" font-size="12" fill="#486e80" font-weight="600">{{item.module.name.length>26?item.module.name.slice(0,25)+'…':item.module.name}}</text></g>
  </g>
 </svg></div>
 <p v-if="warning" class="room-layout-warning" role="status">{{warning}}</p><div class="room-position-footer"><template v-if="dragging"><span :class="{invalid:overlapping}" role="status">{{overlapping?'位置重叠，松开将取消':'松开保存位置'}} · Esc取消</span></template><template v-else-if="current"><strong><span class="selection-type">已选中 · 微模块</span>{{current.name}}</strong><button class="secondary" :disabled="blocked" aria-label="旋转微模块90度" @click="rotate"><RotateCw :size="14"/>旋转90°</button><span class="room-direction">{{(current.roomRotation||0)%180===0?'横向':'纵向'}} · {{current.roomRotation||0}}°</span><ActionMenu label="模块位置操作"><button :disabled="blocked||(!current.roomPosition&&!current.roomRotation)" @click="reset">恢复默认位置</button></ActionMenu><button class="primary" :disabled="blocked" @click="emit('open',current.id)">进入模块 <ArrowRight :size="14"/></button><button class="icon-button" aria-label="取消选择微模块" :disabled="blocked" @click="selected=''"><X :size="16"/></button></template><template v-else><span>选择模块查看，拖动直接移动</span><span class="muted small">位置示意 · 非工程尺寸</span></template></div>
</section></template>
