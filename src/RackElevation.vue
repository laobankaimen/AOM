<script setup lang="ts">
import { computed, ref } from 'vue';
import { roleColors } from './model';
import {rackUnitAt} from './dragGeometry';
import type { Rack, Device, Model } from './model';
const props = defineProps<{ rack: Rack; devices: Device[]; models: Model[]; selected?: string; focused?: boolean;drop?:{rackId:string;startU:number;valid:boolean;reason:string}|null;dragHeight?:number;draggingId?:string }>();
const emit = defineEmits<{ select: [id: string]; move: [id: string, rackId: string, startU: number]; dragStart:[event:PointerEvent,device:Device] }>();
const svg = ref<SVGSVGElement>();const externalHover = ref(0);
const inRack = computed(() => props.devices.filter(d => d.rackId === props.rack.id));
const model = (d: Device) => props.models.find(m => m.id === d.modelId)!;
const toU = (clientY:number)=>rackUnitAt(clientY,svg.value!.getBoundingClientRect());
const hoverU=computed(()=>props.drop?.rackId===props.rack.id?props.drop.startU:externalHover.value);
const dropHeight=computed(()=>props.drop?.rackId===props.rack.id?props.dragHeight||1:1);
const dropValid=computed(()=>props.drop?.rackId===props.rack.id?props.drop.valid:true);
function externalDrop(event:DragEvent){event.preventDefault();const id=event.dataTransfer?.getData('device');if(id)emit('move',id,props.rack.id,toU(event.clientY));externalHover.value=0;}
</script>
<template>
 <div :class="['elevation-card',{focused}]" :data-rack-id="rack.id">
  <div class="elevation-title"><span class="rack-indicator"></span><strong>{{rack.name}}</strong><span>{{inRack.length}} 台设备 · 42U</span></div>
  <svg ref="svg" viewBox="0 0 280 934" class="rack-elevation" :data-drop-rack="rack.id" role="group" :aria-label="`${rack.name} 42U立面`" @dragover.prevent="externalHover=toU($event.clientY)" @dragleave="externalHover=0" @drop="externalDrop">
   <rect x="26" y="10" width="240" height="908" rx="6" fill="#dbe4ef"/><rect x="42" y="36" width="208" height="840" fill="#f8fafc"/>
   <circle v-for="x in [34,258]" :key="x" :cx="x" cy="24" r="2" fill="#697886"/>
   <g v-for="u in 42" :key="u"><rect x="43" :y="36+(42-u)*20" width="206" height="20" fill="#f4f7fb" stroke="#dce4ef" stroke-width=".5"/><text x="20" :y="50+(42-u)*20" text-anchor="end" fill="#536881" font-size="9">{{u}}</text><circle cx="35" :cy="46+(42-u)*20" r="1.5" fill="#526574"/><circle cx="258" :cy="46+(42-u)*20" r="1.5" fill="#526574"/></g>
   <g v-for="d in inRack" :key="d.id" class="elevation-device" :aria-pressed="selected===d.id" :aria-label="`${d.name} U${d.startU}`" role="button" tabindex="0" @pointerdown="emit('dragStart',$event,d)" @keydown.enter="emit('select',d.id)">
    <title>{{d.name}} · {{model(d).name}} · U{{d.startU}}–{{d.startU+model(d).uHeight-1}}</title><rect x="45" :y="36+(43-d.startU-model(d).uHeight)*20+1" width="202" :height="model(d).uHeight*20-2" rx="2" :fill="selected===d.id?'#dce8ff':roleColors[model(d).role]+'18'" :stroke="selected===d.id ? '#1d4ed8' : roleColors[model(d).role]+'80'" :stroke-width="selected===d.id?2:1"/>
    <rect x="50" :y="36+(43-d.startU-model(d).uHeight)*20+5" width="3" :height="model(d).uHeight*20-10" rx="1" :fill="roleColors[model(d).role]"/>
    <text x="60" :y="36+(43-d.startU-model(d).uHeight)*20+model(d).uHeight*10+4" fill="#18324f" font-size="10" font-weight="550">{{d.name.length>20?d.name.slice(0,19)+'…':d.name}}</text>
    <circle cx="235" :cy="36+(43-d.startU-model(d).uHeight)*20+model(d).uHeight*10" r="2" fill="#b4e3d8"/>
   </g>
   <g v-if="hoverU" pointer-events="none"><rect x="43" :y="36+(43-Math.max(1,hoverU)-dropHeight)*20" width="206" :height="dropHeight*20" :fill="dropValid?'#50d6b9':'#ed7777'" fill-opacity=".4" :stroke="dropValid?'#77edce':'#ffaaaa'"/><rect x="43" :y="36+(42-Math.max(1,hoverU))*20" width="206" height="20" :fill="dropValid?'#287c68':'#9e4343'"/><text x="146" :y="50+(42-Math.max(1,hoverU))*20" text-anchor="middle" fill="#fff" font-size="10">{{dropValid?'放置起始位 U'+hoverU:drop?.reason||'位置不可用'}}</text></g>
   <text x="146" y="899" text-anchor="middle" fill="#728493" font-size="10" letter-spacing="3">{{rack.name}} · 42U STANDARD</text>
  </svg>
 </div>
</template>
