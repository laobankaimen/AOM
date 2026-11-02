<script setup lang="ts">
import {computed,ref,onMounted,nextTick,watch} from 'vue';
import {useEditorDraft,useEditorBoundary,useFormFeedback} from './editorGuard';
import {X} from 'lucide-vue-next';
import {request,networkPurposes,roleNames} from './model';
import type {State,Device,NetworkSegment} from './model';
const props=defineProps<{mode:'network'|'members'|'links';state:State;projectId:string;network?:NetworkSegment;initial?:NetworkSegment}>();
const emit=defineEmits<{close:[];saved:[state:State,message:string];advanced:[network:NetworkSegment];open:[id:string];busy:[value:boolean]}>();
const draftRevision=ref(props.state.revision);
const draft=ref<NetworkSegment>(props.initial?JSON.parse(JSON.stringify(props.initial)):{id:'',projectId:props.projectId,parentId:props.network?.id||'',name:'',purpose:'business',vlan:0,cidr:'',gateway:'',color:'#497cda',deviceIds:[],note:''});
const busy=ref(false),error=ref(''),search=ref(''),role=ref(''),moduleId=ref(''),chosen=ref<string[]>([]),sources=ref<string[]>([]),nameInput=ref<HTMLInputElement>();
const root=ref<HTMLElement>();const draftGuard=useEditorDraft(()=>props.mode==='network'?draft.value:{chosen:chosen.value,sources:sources.value},()=>busy.value);useEditorBoundary(root,()=>busy.value);useFormFeedback(root,()=>error.value);
watch(busy,value=>emit('busy',value),{flush:'sync'});
const rooms=computed(()=>new Set(props.state.sites.filter(s=>s.projectId===props.projectId).map(s=>s.id)));
const modules=computed(()=>props.state.modules.filter(m=>rooms.value.has(m.siteId)));
const devices=computed(()=>props.state.devices.filter(d=>modules.value.some(m=>m.id===d.moduleId)));
const duplicate=computed(()=>props.state.networks.find(n=>n.projectId===props.projectId&&n.id!==draft.value.id&&n.name.trim().toLowerCase()===draft.value.name.trim().toLowerCase()));
const modelRole=(d:Device)=>props.state.models.find(m=>m.id===d.modelId)?.role||'server';
const candidates=computed(()=>devices.value.filter(d=>(props.mode==='members'?!props.network?.deviceIds.includes(d.id):props.network?.deviceIds.includes(d.id))&&!sources.value.includes(d.id)));
const filtered=computed(()=>candidates.value.filter(d=>(!moduleId.value||d.moduleId===moduleId.value)&&(!role.value||modelRole(d)===role.value)&&[d.name,d.assetTag,props.state.models.find(m=>m.id===d.modelId)?.name].join(' ').toLowerCase().includes(search.value.toLowerCase())));
const upstream=computed(()=>devices.value.filter(d=>props.network?.deviceIds.includes(d.id)).sort((a,b)=>{const rank=(d:Device)=>['core','aggregation','access','management','storage','server'].indexOf(modelRole(d));return rank(a)-rank(b)||a.name.localeCompare(b.name)}));
const pairs=computed(()=>sources.value.flatMap(sourceId=>chosen.value.filter(targetId=>targetId!==sourceId).map(targetId=>({sourceId,targetId}))));
const newPairs=computed(()=>pairs.value.filter(p=>!props.state.links.some(l=>l.networkIds.includes(props.network?.id||'')&&(l.sourceId===p.sourceId&&l.targetId===p.targetId||l.sourceId===p.targetId&&l.targetId===p.sourceId))));
function selectVisible(){chosen.value=[...new Set([...chosen.value,...filtered.value.map(d=>d.id)])]}
function sourceChanged(){chosen.value=chosen.value.filter(id=>!sources.value.includes(id))}
function preset(name:string,purpose:string,color:string){draft.value.name=name;draft.value.purpose=purpose;draft.value.color=color;nameInput.value?.focus()}
async function submit(){
 if(busy.value)return;error.value='';
 if(props.mode==='network'&&(!draft.value.name.trim()||duplicate.value)){error.value=duplicate.value?`「${draft.value.name.trim()}」已经存在，请打开已有网络或换一个名称。`:'请填写网络名称';nameInput.value?.focus();return}
 busy.value=true;
 try{
  let next:State,message:string;
  if(props.mode==='network'){const n={...draft.value,vlan:Number(draft.value.vlan)||0,name:draft.value.name.trim(),cidr:draft.value.cidr.trim(),gateway:draft.value.gateway.trim()};next=await request<State>('/networks'+(n.id?'/'+n.id:''),n.id?'PUT':'POST',{revision:draftRevision.value,network:n});message=n.id?'网络已更新':'网络已创建，可批量添加设备'}
  else if(props.mode==='members'){next=await request<State>('/network-members','POST',{revision:draftRevision.value,deviceIds:chosen.value,targetId:props.network!.id,mode:'add'});message=`已将 ${chosen.value.length} 台设备加入「${props.network!.name}」`}
  else{next=await request<State>('/links/batch','POST',{revision:draftRevision.value,networkId:props.network!.id,sources:sources.value,targets:chosen.value});message=`已创建 ${newPairs.value.length} 条待确认连接`}
  draftGuard.accept();emit('saved',next,message);
  if(props.mode==='network'){const n=next.networks.find(n=>n.projectId===props.projectId&&n.name===draft.value.name.trim());if(n)emit('open',n.id)}
  emit('close');
 }catch(e){error.value=(e as Error).message}finally{busy.value=false}
}
onMounted(()=>nextTick(()=>nameInput.value?.focus()));
</script>
<template>
 <section ref="root" class="panel quick-network" :aria-label="mode==='network'?'快速网络配置':mode==='members'?'批量加入网络':'批量建立连接'">
  <div class="panel-heading"><div><h2>{{mode==='network'?(initial?'修改网络':'创建网络'):mode==='members'?'批量加入网络':'批量建立连接'}}</h2><span v-if="network" class="text-badge">{{network.name}}</span></div><button class="icon-button" aria-label="关闭快捷操作" :disabled="busy" @click="emit('close')"><X :size="16"/></button></div>
  <form class="quick-network-body" @submit.prevent="submit"><fieldset class="editor-fieldset" :disabled="busy" :aria-busy="busy">
   <p v-if="error" class="error-banner" role="alert">{{error}}</p>
   <template v-if="mode==='network'">
    <div v-if="!initial" class="quick-presets"><span>常用名称</span><button type="button" @click="preset('数据网络','business','#497cda')">数据网络</button><button type="button" @click="preset('管理网络','management','#159784')">管理网络</button><button type="button" @click="preset('存储网络','storage','#9b6acb')">存储网络</button></div>
    <div class="form-grid"><label>网络名称<input ref="nameInput" v-model="draft.name" required maxlength="80" :aria-invalid="!!duplicate" placeholder="例如：数据网络、接入组01"/></label><label>网络用途<select v-model="draft.purpose"><option v-for="(name,key) in networkPurposes" :key="key" :value="key">{{name}}</option></select></label></div>
    <p v-if="duplicate" class="field-error" role="alert">项目中已有「{{duplicate.name}}」。<button type="button" class="text-button" @click="emit('open',duplicate.id);emit('close')">打开已有网络</button>，或换一个名称。</p>
    <details class="selection-tools"><summary>选填 VLAN、网段与网关</summary><div class="form-grid three"><label>VLAN ID<input v-model.number="draft.vlan" type="number" min="0" max="4094"/></label><label>网段 CIDR<input v-model="draft.cidr" placeholder="10.106.3.0/24" name="cidr" spellcheck="false" autocomplete="off"/></label><label>网关地址<input v-model="draft.gateway" placeholder="10.106.3.1" name="gateway" spellcheck="false" autocomplete="off"/></label></div></details>
    <div class="quick-actions"><button type="button" class="text-button" :disabled="busy" @click="draftGuard.accept();emit('advanced',draft)">高级编辑</button><button class="primary" :disabled="busy||!!duplicate">{{initial?'保存网络':'创建网络'}}</button></div>
   </template>
   <template v-else>
    <div v-if="mode==='links'" class="batch-upstream"><h3>1. 选择上联设备 <small>可选两台，形成双上联</small></h3><div class="quick-device-list"><label v-for="d in upstream" :key="d.id" class="checkbox-label"><input v-model="sources" type="checkbox" :value="d.id" :aria-label="`选择上联 ${d.name}`" :disabled="busy||sources.length>=8&&!sources.includes(d.id)" @change="sourceChanged"/>{{d.name}}<small>{{roleNames[modelRole(d)]}}</small></label></div><h3>2. 选择下联设备</h3></div>
    <div class="quick-filters"><input v-model="search" aria-label="筛选批量设备" placeholder="搜索名称、编号、型号"/><select v-model="role" aria-label="筛选设备角色"><option value="">全部角色</option><option v-for="(name,key) in roleNames" :key="key" :value="key">{{name}}</option></select><select v-model="moduleId" aria-label="筛选设备微模块"><option value="">全部微模块</option><option v-for="m in modules" :key="m.id" :value="m.id">{{m.name}}</option></select></div>
    <div class="quick-selection"><button type="button" class="text-button" :disabled="busy" @click="selectVisible">全选筛选结果（{{filtered.length}}）</button><button type="button" class="text-button" :disabled="busy" @click="chosen=[]">清空选择</button><strong>已选 {{chosen.length}} 台</strong></div>
    <div class="quick-device-list"><label v-for="d in filtered" :key="d.id" class="checkbox-label"><input v-model="chosen" type="checkbox" :value="d.id" :aria-label="`选择设备 ${d.name}`" :disabled="busy"/>{{d.name}}<small>{{d.assetTag}}</small></label><p v-if="!filtered.length" class="panel-note">没有匹配设备，请调整筛选条件。</p></div>
    <p v-if="mode==='links'&&pairs.length>2000" class="field-error" role="alert">最多一次建立2000对连接，当前 {{pairs.length}} 对。请减少上联或下联设备后再提交。</p><div class="quick-actions"><span v-if="mode==='links'">将新增 <b>{{newPairs.length}}</b> 条待确认逻辑关系，跳过 {{pairs.length-newPairs.length}} 条已有关系</span><span v-else>保留设备已有的其他网络归属</span><button class="primary" :disabled="busy||!chosen.length||mode==='links'&&(!sources.length||!newPairs.length||pairs.length>2000)">{{mode==='members'?'加入网络':'确认建立连接'}}</button></div>
   </template>
  </fieldset></form>
 </section>
</template>
