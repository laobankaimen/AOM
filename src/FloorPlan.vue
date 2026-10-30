<script setup lang="ts">
import { computed,ref,watch,nextTick } from 'vue';
import {useDiagramViewport} from './diagramViewport';
import {useCabinetDrag} from './useCabinetDrag';
import {useInfrastructureDrag} from './useInfrastructureDrag';
import { infraColors, infraNames } from './model';
import { rowLayout, layoutWidth } from './layout';
import type { Module, Rack, Device, Model, Infra } from './model';
const props=defineProps<{module:Module;racks:Rack[];devices:Device[];models:Model[];infra:Infra[];selected:string;zoom:number;busy?:boolean;selectedInfrastructure?:string}>();
const emit=defineEmits<{clear:[];select:[id:string,inspect?:boolean];infrastructure:[item:Infra,inspect?:boolean];insertRack:[id:string,side:string,beforeId:string];moveInfrastructure:[id:string,side:string,afterRack:number,beforeId:string]}>();
const rows=computed(()=>['L',...(props.module.type==='double'?['R']:[])].map(side=>({side,tiles:rowLayout(props.module,props.racks,props.infra,side)})));
const drag=useCabinetDrag(()=>props.racks.filter(r=>r.moduleId===props.module.id),()=>rows.value,(id,side,before)=>emit('insertRack',id,side,before),()=>!props.busy&&!facilities.active.value);
const facilities=useInfrastructureDrag(()=>rows.value,(id,side,after,before)=>emit('moveInfrastructure',id,side,after,before),()=>!props.busy&&!drag.active.value);
const dropIndicator=computed(()=>{
 const cabinet=drag.active.value?.moved&&drag.destination.value;
 if(cabinet){const target=rows.value.find(r=>r.side===cabinet.side)?.tiles.find(t=>t.id===cabinet.beforeId);return {...cabinet,kind:'cabinet',label:cabinet.error||`${cabinet.side}列 · ${target?target.name.slice(0,8)+'前':'列尾'}`}}
 const infra=facilities.active.value?.moved&&facilities.destination.value;
 return infra?{...infra,kind:'infra',error:'',label:`${infra.side}列 · ${infra.afterRack===0?'列首':'第'+infra.afterRack+'柜后'}`}:null;
});
const width=computed(()=>Math.max(940,...rows.value.map(row=>layoutWidth(row.tiles)+160)));
const used=(id:string)=>props.devices.filter(d=>d.rackId===id).reduce((n,d)=>n+(props.models.find(m=>m.id===d.modelId)?.uHeight||0),0);
function pointerDown(event:PointerEvent,tile:ReturnType<typeof rowLayout>[number]){
 if(event.button!==0||props.busy)return;
 if(tile.rack){emit('select',tile.id,false);drag.down(event,tile.rack)}else if(tile.infra){emit('infrastructure',tile.infra,false);facilities.down(event,tile.infra)}
}
function clearOnBlank(event:MouseEvent){if(!(event.target as Element).closest('.rack-hit'))emit('clear')}
const scroller=ref<HTMLElement>();
const fitted=useDiagramViewport(scroller,()=>({width:width.value,height:viewHeight.value}),()=>props.zoom,()=>!!drag.active.value||!!facilities.active.value);
watch(()=>props.zoom,value=>{if(value===100)nextTick(fitted.resetScroll)});
defineExpose({fit:()=>nextTick(fitted.resetScroll)});
const viewHeight=computed(()=>props.module.type==='double'?480:330);
const y=(side:string)=>props.module.type==='double'?(side==='L'?82:300):100;
</script>
<template>
 <div ref="scroller" class="floor-scroll"><svg class="floor-svg" :viewBox="`0 0 ${width} ${viewHeight}`" :style="fitted.style.value" role="group" @click="clearOnBlank" :aria-label="`${module.name}俯视图`" @pointermove="drag.move($event);facilities.move($event)" @pointerup="drag.up($event);facilities.up($event)" @pointercancel="drag.cancel();facilities.cancel()">
  <defs><pattern id="floor-grid" width="22" height="22" patternUnits="userSpaceOnUse"><path d="M22 0H0V22" fill="none" stroke="#e0e8ed" stroke-width=".7"/></pattern></defs>
  <rect width="100%" height="100%" fill="#f7fafb"/><rect width="100%" height="100%" fill="url(#floor-grid)"/>
  <rect x="25" y="25" :width="width-50" :height="viewHeight-50" rx="12" fill="white" fill-opacity=".55" stroke="#cedce2" stroke-dasharray="6 5"/>
  <text x="48" y="52" class="svg-caption">{{module.productModel||'自定义微模块'}} · {{module.type==='double'?'双排':'单排'}} · 位置示意</text>
  <g v-if="module.type==='double'"><rect x="70" y="240" :width="width-140" height="40" rx="4" fill="#dff3ed" stroke="#bce3d7" stroke-dasharray="5 5"/><text :x="width/2" y="265" text-anchor="middle" fill="#459b85" font-size="12" letter-spacing="3">← 冷通道 / COLD AISLE →</text></g>
  <g v-for="row in rows" :key="row.side" :data-cabinet-row="row.side"><rect x="70" :y="y(row.side)-12" :width="width-120" height="160" fill="transparent" pointer-events="all"/>
   <text x="43" :y="y(row.side)+52" fill="#829ba7" font-size="11">{{row.side}}列</text>
   <g v-for="tile in row.tiles" :key="tile.id" :transform="`translate(${80+tile.x},${y(row.side)})`" :class="['rack-hit',{'cabinet-dragging':drag.active.value?.id===tile.id&&drag.active.value?.moved||facilities.active.value?.item.id===tile.id&&facilities.active.value?.moved}]" :data-cabinet-id="tile.rack?tile.id:undefined" :data-infrastructure-id="tile.infra?.id" @pointerdown="pointerDown($event,tile)" @keydown="tile.rack?drag.key($event,tile.rack):facilities.key($event,tile.infra!)" role="button" tabindex="0" :aria-pressed="selected===tile.id||selectedInfrastructure===tile.id" :aria-label="tile.rack?`选择机柜 ${tile.name}`:`查看设施 ${tile.name}`" @click="tile.rack?drag.clicked(()=>emit('select',tile.id)):facilities.clicked(()=>emit('infrastructure',tile.infra!))" @keydown.enter="tile.rack?emit('select',tile.id):emit('infrastructure',tile.infra!)">
    <title>{{tile.name}}{{tile.infra?' · 基础设施位置待现场核对':''}}</title>
    <rect x="-4" y="-4" :width="tile.width+8" height="130" rx="7" :fill="(selected===tile.id||selectedInfrastructure===tile.id)?'#e9efff':'transparent'" :stroke="(selected===tile.id||selectedInfrastructure===tile.id)?'#1d4ed8':'transparent'"/>
    <rect :width="tile.width" height="122" rx="5" :fill="tile.rack?'white':`${infraColors[tile.kind]}18`" :stroke="tile.rack?'#b5cbd3':infraColors[tile.kind]"/>
    <template v-if="tile.rack"><rect x="8" y="9" :width="tile.width-16" height="70" rx="3" :fill="used(tile.id)>0?'#d9ece7':'#eaf0f3'"/><path v-for="line in 6" :key="line" :d="`M16 ${12+line*7}H${tile.width-16}`" stroke="#bfd0d7" stroke-width="2"/><text x="12" y="99" fill="#314c5c" font-size="14" font-weight="600">{{tile.name}}</text><rect x="8" y="128" :width="tile.width-16" height="3" rx="1.5" fill="#e1e8f2"/><rect x="8" y="128" :width="(tile.width-16)*Math.min(1,used(tile.id)/42)" height="3" rx="1.5" fill="#497cda"/><text :x="tile.width/2" y="143" text-anchor="middle" fill="#8a9ea9" font-size="10">{{used(tile.id)}} / 42 U</text>
     <g v-for="(item,j) in tile.integrated.slice(0,4)" :key="item.id" :transform="`translate(${8+(j%4)*24},108)`"><circle r="4" :fill="infraColors[item.kind]"/><title>{{item.name}} · 柜内集成，U位待确认</title></g><text v-if="tile.integrated.length>4" x="88" y="112" font-size="8" fill="#8ca1ac">+{{tile.integrated.length-4}}</text>
    </template>
    <template v-else><path v-if="tile.kind==='ac'" d="M38 23V61M21 32L55 52M21 52L55 32" :stroke="infraColors[tile.kind]" stroke-width="2"/><path v-else-if="tile.kind==='env'" d="M17 48H26L33 30L42 57L50 41H61" :stroke="infraColors[tile.kind]" stroke-width="2" fill="none"/><path v-else d="M42 23L27 45H39L33 64L52 39H40Z" :fill="infraColors[tile.kind]"/><text x="38" y="88" text-anchor="middle" :fill="infraColors[tile.kind]" font-size="12">{{infraNames[tile.kind]}}</text><text x="38" y="107" text-anchor="middle" fill="#91a1aa" font-size="9">独立位置</text></template>
    <g v-if="selected===tile.id||selectedInfrastructure===tile.id" class="object-selection-badge" pointer-events="none"><rect :width="tile.width" height="17" y="-22" rx="4" fill="#1d4ed8"/><text :x="tile.width/2" y="-10" text-anchor="middle" fill="white" font-size="9">✓ 已选中</text></g>
   </g>
  </g>
  <g v-if="dropIndicator" pointer-events="none" :class="['layout-drop-indicator',`${dropIndicator.kind}-drop-indicator`,{'invalid':!!dropIndicator.error}]" :data-side="dropIndicator.side" :data-before-id="dropIndicator.beforeId||''">
   <rect :x="80+dropIndicator.markerX-3" :y="y(dropIndicator.side)-10" width="6" height="145" rx="3" :fill="dropIndicator.error?'#c03939':'#279b7a'"/>
   <rect :x="80+dropIndicator.markerX-60" :y="y(dropIndicator.side)-36" width="120" height="22" rx="5" :fill="dropIndicator.error?'#a72c2c':'#24866b'"/>
   <text :x="80+dropIndicator.markerX" :y="y(dropIndicator.side)-21" text-anchor="middle" fill="white" font-size="10">{{dropIndicator.label}}</text>
  </g>
  <g v-if="module.type==='single'"><path :d="`M90 270H${width-90}`" stroke="#a9d5c9" stroke-dasharray="6 5"/><text :x="width/2" y="295" text-anchor="middle" fill="#83a99d" font-size="11" letter-spacing="3">FRONT / 操作通道</text></g>


 </svg></div>
</template>
