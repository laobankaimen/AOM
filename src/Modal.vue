<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { X } from 'lucide-vue-next';
const props = defineProps<{ title: string; wide?: boolean; busy?: boolean }>();
const emit = defineEmits<{ close: [] }>();const dialog = ref<HTMLDialogElement>();
onMounted(() => dialog.value?.showModal());
function close() { if (!props.busy) emit('close'); }
</script>
<template><dialog ref="dialog" :class="['modal', {wide}]" aria-labelledby="modal-heading" @cancel.prevent="close" @click="($event.target===dialog)&&close()"><div class="modal-heading"><div><span class="eyebrow">AOM WORKSPACE</span><h2 id="modal-heading">{{title}}</h2></div><button class="icon-button" aria-label="关闭弹窗" :disabled="busy" @click="close"><X :size="20"/></button></div><div class="modal-body"><slot/></div></dialog></template>
