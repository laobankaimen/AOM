<script setup lang="ts">
import {computed} from 'vue';
import ActionMenu from './ActionMenu.vue';
import {displayPreferences as display,focusDisplay,resetDisplay,focusedDisplay} from './displayPreferences';
defineProps<{canvas?:boolean}>();
const items=[['sidebar','侧栏导航'],['stats','统计卡片'],['networks','网络分区列表'],['tools','筛选与视图工具'],['labels','图例与辅助说明'],['details','对象详情栏']] as const;
const hasHidden=computed(()=>Object.values(display).some(value=>!value));
</script>
<template><div class="view-controls"><button v-if="canvas" class="text-button" :aria-pressed="focusedDisplay" @click="focusDisplay">{{focusedDisplay?'退出专注':'专注画布'}}</button><span v-if="hasHidden" class="display-hidden-hint">部分界面已隐藏</span><ActionMenu label="界面显示"><label v-for="[key,name] in items" :key="key" class="checkbox-label"><input v-model="display[key]" type="checkbox"/>{{name}}</label><button @click="resetDisplay">恢复默认显示</button></ActionMenu></div></template>
