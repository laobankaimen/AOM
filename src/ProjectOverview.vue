<script setup lang="ts">
import {navigationHash} from './navigationState';
import {computed} from 'vue';
import {Building2,Server,HardDrive,Layers3,ArrowRight} from 'lucide-vue-next';
import type {State} from './model';
const props=defineProps<{state:State;projectId:string}>();const emit=defineEmits<{room:[id:string];assets:[]}>();
const rooms=computed(()=>props.state.sites.filter(s=>s.projectId===props.projectId));
const modules=computed(()=>props.state.modules.filter(m=>rooms.value.some(r=>r.id===m.siteId)));
const racks=computed(()=>props.state.racks.filter(r=>modules.value.some(m=>m.id===r.moduleId)));
const devices=computed(()=>props.state.devices.filter(d=>modules.value.some(m=>m.id===d.moduleId)));
function roomHref(id:string){return navigationHash({view:'room',project:props.projectId,room:id,module:'',rack:'',network:''})}
function enter(event:MouseEvent,id:string){if(event.button!==0||event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;event.preventDefault();emit('room',id)}
const roomModules=(id:string)=>modules.value.filter(m=>m.siteId===id);
const roomRacks=(id:string)=>racks.value.filter(r=>roomModules(id).some(m=>m.id===r.moduleId));
const roomDevices=(id:string)=>devices.value.filter(d=>roomModules(id).some(m=>m.id===d.moduleId));
</script>
<template>
 <section class="project-home" aria-label="项目总览">
  <div class="home-metrics"><div><Building2 :size="21"/><strong>{{rooms.length}}</strong><span>机房</span></div><div><Layers3 :size="21"/><strong>{{modules.length}}</strong><span>微模块</span></div><div><Server :size="21"/><strong>{{racks.length}}</strong><span>机柜</span></div><button @click="emit('assets')"><HardDrive :size="21"/><strong>{{devices.length}}</strong><span>设备资产 <ArrowRight :size="13"/></span></button></div>
  <section class="panel home-room-panel" :style="{maxWidth:rooms.length>0&&rooms.length<3?rooms.length*466+24+'px':undefined}"><div class="panel-heading"><h2>机房总览</h2><span class="muted small">选择机房，逐层查看模块与设备</span></div><div class="home-room-grid" ><a v-for="room in rooms" :href="roomHref(room.id)" :key="room.id" class="home-room-card" :aria-label="`进入机房 ${room.name}`" @click="enter($event,room.id)"><div class="home-room-visual" aria-hidden="true"><Building2 :size="37"/><div class="room-silhouettes"><i v-for="n in Math.min(6,roomModules(room.id).length||1)" :key="n"><b></b><b></b><b></b></i></div></div><div class="home-room-name"><h3>{{room.name}}</h3><ArrowRight :size="18"/></div><p>{{roomModules(room.id).length}} 微模块 <span>·</span> {{roomRacks(room.id).length}} 机柜 <span>·</span> {{roomDevices(room.id).length}} 设备</p><div class="home-room-footer"><span>{{roomDevices(room.id).filter(d=>!!d.rackId).length}} 台已上架</span><span>{{roomDevices(room.id).filter(d=>!d.rackId).length}} 台待上架</span></div></a></div><div v-if="!rooms.length" class="empty-state"><Building2 :size="34"/><h3>项目中还没有机房</h3><p>通过创建机房进入配置页。</p></div></section>
 </section>
</template>
