<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue';
defineProps<{label?:string}>();
const menu=ref<HTMLDetailsElement>();
function close(){if(menu.value)menu.value.open=false}
function outside(event:PointerEvent){if(!menu.value?.contains(event.target as Node))close()}
function escape(event:KeyboardEvent){if(event.key==='Escape'&&menu.value?.open){close();menu.value.querySelector('summary')?.focus()}}
function choose(event:MouseEvent){if((event.target as Element).closest('button,a'))close()}
onMounted(()=>{document.addEventListener('pointerdown',outside);document.addEventListener('keydown',escape)});
onBeforeUnmount(()=>{document.removeEventListener('pointerdown',outside);document.removeEventListener('keydown',escape)});
</script>
<template><details ref="menu" class="action-menu" @click="choose"><summary>{{label||'更多'}}</summary><div class="action-menu-items"><slot/></div></details></template>
