<script setup lang="ts">
import {ref} from 'vue';
import ActionMenu from './ActionMenu.vue';
import {exportDiagram} from './diagramExport';
const props=defineProps<{title:string;scope:string;selector:string;disabled?:boolean}>();
const busy=ref(false),message=ref(''),failed=ref(false);
async function run(format:'png'|'pdf'|'svg'){
 if(busy.value||props.disabled)return;busy.value=true;message.value='正在生成导出文件…';failed.value=false;
 try{message.value=await exportDiagram([...document.querySelectorAll<SVGSVGElement>(props.selector)],props.title,props.scope,format)}catch(e){failed.value=true;message.value=(e as Error).message}finally{busy.value=false}
}
</script>
<template><div class="diagram-export"><ActionMenu label="导出图形"><p class="diagram-export-note">当前图形完整范围，包含屏幕外内容</p><button :disabled="disabled||busy" @click="run('png')">导出 PNG 图片</button><button :disabled="disabled||busy" @click="run('pdf')">导出 PDF（单页）</button><button :disabled="disabled||busy" @click="run('svg')">导出 SVG（矢量）</button></ActionMenu><span v-if="message" :class="['diagram-export-status',{'is-error':failed}]" :role="failed?'alert':'status'">{{message}}<button v-if="!busy" aria-label="关闭导出提示" @click="message=''">×</button></span></div></template>
