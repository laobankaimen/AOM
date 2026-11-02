<script setup lang="ts">
import {computed} from 'vue';
import type {ModulePlan} from './model';
import {infraColors,infraNames} from './model';
import {previewPlan} from './templatePlan';
import {rowLayout,layoutWidth} from './layout';
const props=defineProps<{plan:ModulePlan}>();
const preview=computed(()=>previewPlan(props.plan));
const rows=computed(()=>['L',...(props.plan.type==='double'?['R']:[])].map(side=>({side,tiles:rowLayout(preview.value.module,preview.value.racks,preview.value.infra,side)})));
const width=computed(()=>Math.max(200,...rows.value.map(r=>layoutWidth(r.tiles))));
</script>
<template><div class="module-schematic template-actual-preview"><svg :viewBox="`-8 -8 ${width+16} ${rows.length===2?290:140}`" role="img" :aria-label="`布局预览：${preview.racks.length}个机柜，${preview.infra.length}项设施`"><g v-for="(row,i) in rows" :key="row.side" :transform="`translate(0,${i*150})`"><g v-for="tile in row.tiles" :key="tile.id" :transform="`translate(${tile.x},0)`"><title>{{tile.name}} · {{tile.rack?'机柜':infraNames[tile.kind]}}</title><rect :width="tile.width" height="110" rx="4" :fill="tile.rack?'#dce8f7':infraColors[tile.kind]+'25'" :stroke="tile.rack?'#87a5cc':infraColors[tile.kind]"/><text :x="tile.width/2" y="64" text-anchor="middle" font-size="16" :fill="tile.rack?'#334155':infraColors[tile.kind]">{{tile.rack?'柜':infraNames[tile.kind]}}</text><circle v-for="(item,j) in tile.integrated" :key="item.id" :cx="8+j*10" cy="96" r="4" :fill="infraColors[item.kind]"><title>{{item.name}}</title></circle></g></g></svg><span>{{preview.racks.length}} 柜 · {{preview.infra.length}} 项设施</span></div></template>
