<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref} from 'vue';
import {useEditorBoundary,useEditorDraft,useFormFeedback} from './editorGuard';
import { ArrowLeft } from 'lucide-vue-next';
const props=defineProps<{title:string;busy?:boolean;wide?:boolean;draft?:unknown;error?:string}>();
const root=ref<HTMLElement>();useEditorBoundary(root,()=>!!props.busy);useEditorDraft(()=>props.draft,()=>!!props.busy);useFormFeedback(root,()=>props.error||'');
const heading=ref<HTMLElement>();const previousFocus=document.activeElement as HTMLElement|null;
onMounted(()=>heading.value?.focus());onBeforeUnmount(()=>{if(previousFocus?.isConnected)previousFocus.focus({preventScroll:true})});
const emit=defineEmits<{close:[]}>();
</script>
<template><section ref="root" class="editor-page" role="region" :aria-label="title"><div class="editor-page-inner"><div class="editor-page-heading"><button class="secondary" :disabled="busy" @click="emit('close')"><ArrowLeft :size="16"/>返回查看</button><div><h1 ref="heading" tabindex="-1">{{title}}</h1></div></div><fieldset class="panel editor-page-body editor-fieldset" :disabled="busy" :aria-busy="busy"><p v-if="busy" class="saving-status" role="status">正在保存，请稍候…</p><slot/></fieldset></div></section></template>
