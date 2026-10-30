<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref} from 'vue';
import { ArrowLeft } from 'lucide-vue-next';
defineProps<{title:string;busy?:boolean;wide?:boolean}>();
const heading=ref<HTMLElement>();const previousFocus=document.activeElement as HTMLElement|null;
onMounted(()=>heading.value?.focus());onBeforeUnmount(()=>{if(previousFocus?.isConnected)previousFocus.focus({preventScroll:true})});
const emit=defineEmits<{close:[]}>();
</script>
<template><section class="editor-page" role="region" :aria-label="title"><div class="editor-page-inner"><div class="editor-page-heading"><button class="secondary" :disabled="busy" @click="emit('close')"><ArrowLeft :size="16"/>返回查看</button><div><span class="eyebrow">DETAIL EDITOR</span><h1 ref="heading" tabindex="-1">{{title}}</h1></div></div><div class="panel editor-page-body"><slot/></div></div></section></template>
