<script setup lang="ts">
import {computed,ref,nextTick,watch} from 'vue';import Modal from './Modal.vue';import {request,infraNames} from './model';import type {State} from './model';
const props=defineProps<{state:State;kind:'module'|'rack';id:string}>();const emit=defineEmits<{close:[];saved:[state:State];refresh:[state:State];busy:[value:boolean]}>();
const rack=computed(()=>props.kind==='rack'?props.state.racks.find(r=>r.id===props.id):undefined);
const module=computed(()=>props.state.modules.find(m=>m.id===(rack.value?.moduleId||props.id)));
const racks=computed(()=>props.state.racks.filter(r=>r.moduleId===module.value?.id));
const whole=computed(()=>props.kind==='module'||!!rack.value&&racks.value.length===1);
const name=computed(()=>whole.value?module.value?.name:rack.value?.name);
const devices=computed(()=>props.state.devices.filter(d=>whole.value?d.moduleId===module.value?.id:d.rackId===rack.value?.id));
const ids=computed(()=>new Set(devices.value.map(d=>d.id)));
const facilities=computed(()=>props.state.infra.filter(i=>whole.value?i.moduleId===module.value?.id:i.moduleId===rack.value?.moduleId&&i.side===rack.value?.side&&i.afterRack===rack.value?.index&&i.placement==='integrated'));
const links=computed(()=>props.state.links.filter(l=>ids.value.has(l.sourceId)||ids.value.has(l.targetId)));
const crossLinks=computed(()=>links.value.filter(l=>!ids.value.has(l.sourceId)||!ids.value.has(l.targetId)));
const memberships=computed(()=>props.state.networks.filter(n=>n.deviceIds.some(id=>ids.value.has(id))));
const project=computed(()=>props.state.sites.find(s=>s.id===module.value?.siteId)?.projectId);
const targets=computed(()=>props.state.modules.filter(m=>m.id!==module.value?.id&&props.state.sites.some(s=>s.id===m.siteId&&s.projectId===project.value)));
const mode=ref(''),target=ref(''),confirmed=ref(false),removeFacilities=ref(true),busy=ref(false),error=ref(''),alert=ref<HTMLElement>();
const initialRevision=ref(props.state.revision);
const actualMode=computed(()=>!devices.value.length?(whole.value?'empty':'uninstall'):mode.value);
const canSubmit=computed(()=>!!name.value&&!!actualMode.value&&(!whole.value||actualMode.value!=='migrate'||!!target.value)&&(actualMode.value!=='cascade'||confirmed.value)&&(!facilities.value.length||whole.value||removeFacilities.value));
const collapsedRow=computed(()=>!whole.value&&racks.value.filter(r=>r.side===rack.value?.side).length===1);
watch(()=>props.state.revision,value=>{if(value!==initialRevision.value){confirmed.value=false;initialRevision.value=value} });
function resetChoice(){mode.value=whole.value?(targets.value.length?'migrate':''):'uninstall';target.value='';confirmed.value=false;error.value=''}
resetChoice();
async function submit(){
 if(busy.value||!canSubmit.value)return;busy.value=true;emit('busy',true);error.value='';
 try{const result=await request<State>((whole.value?'/modules/'+module.value!.id:'/racks/'+rack.value!.id),'DELETE',{revision:initialRevision.value,mode:actualMode.value,targetModuleId:target.value,removeFacilities:removeFacilities.value,confirmName:actualMode.value==='cascade'?name.value:''});emit('saved',result);emit('close')}
 catch(e){error.value=(e as Error).message;if((e as {status?:number}).status===409){try{emit('refresh',await request<State>('/state'));error.value='数据刚刚发生变化，已刷新数量；请核对本窗口内容后重新确认。';confirmed.value=false}catch{error.value='刷新数据失败，请关闭窗口刷新后重试。'}}await nextTick();alert.value?.scrollIntoView({block:'nearest'});alert.value?.focus()}
 finally{busy.value=false;emit('busy',false)}
}
</script>
<template><Modal :title="whole?'删除微模块':'删除机柜'" :busy="busy" wide @close="emit('close')"><form class="space-delete-dialog" @submit.prevent="submit">
 <h3>删除「{{name||'对象已不存在'}}」</h3><p v-if="kind==='rack'&&whole" class="info-banner">这是微模块的最后一个机柜，将直接删除整个微模块，无需先清理机柜或设施。</p><p v-else-if="collapsedRow" class="info-banner">这是该列的最后一个机柜。删除后模块转为单排，另一列及独立设施保留。</p>
 <div class="deletion-impact" aria-label="删除影响范围"><span><b>{{whole?racks.length:1}}</b> 个机柜</span><span><b>{{facilities.length}}</b> 项设施标记</span><span><b>{{devices.filter(d=>!!d.rackId).length}}</b> 台在架设备</span><span><b>{{devices.filter(d=>!d.rackId).length}}</b> 台待上架设备</span></div>
 <p class="delete-explanation">设施标记描述供电、空调、环境等布局，与设备台账分开统计；可能来自创建模板，不代表已核实的实物设备。</p>
 <details v-if="facilities.length" class="compact-help"><summary>查看设施标记（{{facilities.length}}）</summary><ul><li v-for="i in facilities" :key="i.id">{{i.name}} · {{infraNames[i.kind]||i.kind}} · {{i.placement==='integrated'?'柜内集成':'列内独立'}}</li></ul></details>
 <details v-if="devices.length" class="compact-help"><summary>查看设备（含待上架，共{{devices.length}}台）</summary><div class="deletion-device-list"><p v-for="d in devices" :key="d.id">{{d.name}} · {{d.assetTag}} · {{d.rackId?(state.racks.find(r=>r.id===d.rackId)?.name+' / U'+d.startU):'待上架'}}</p></div></details>
 <fieldset v-if="devices.length" :disabled="busy" class="deletion-choices"><legend>如何处理这 {{devices.length}} 台设备</legend>
  <label v-if="whole" class="delete-choice"><input v-model="mode" type="radio" value="migrate" :disabled="!targets.length" @change="confirmed=false"/><span><strong>迁移设备后删除微模块</strong><small>{{targets.length?'设备统一移到目标模块的待上架区，资产信息、网络归属和连接保留。':'当前项目没有其他微模块可接收；可取消后新建目标模块，或选择下方连同内容删除。'}}</small></span></label>
  <label v-else class="delete-choice"><input v-model="mode" type="radio" value="uninstall" @change="confirmed=false"/><span><strong>保留设备，移至本模块待上架区</strong><small>删除机柜，设备台账、网络和连接保留。</small></span></label>
  <label class="delete-choice"><input v-model="mode" type="radio" value="cascade" @change="confirmed=false"/><span><strong>{{whole?'连同全部内容删除':'连同柜内设备删除'}}</strong><small>一并删除这 {{devices.length}} 台设备、{{links.length}} 条关联连接，并清理 {{memberships.length}} 个网络的成员引用。网络分区和其他设备保留。</small></span></label>
 </fieldset>
 <label v-if="devices.length&&mode==='migrate'" class="full-label">设备迁移目标<select v-model="target" aria-label="设备迁移目标" :disabled="busy"><option value="">选择同项目的其他微模块</option><option v-for="m in targets" :key="m.id" :value="m.id">{{state.sites.find(s=>s.id===m.siteId)?.name}} / {{m.name}}</option></select></label>
 <p v-if="mode==='cascade'&&crossLinks.length" class="deletion-warning">包含 {{crossLinks.length}} 条连接到范围外设备的链路：仅删除这些关联链路，范围外设备保留。</p>
 <label v-if="!whole&&facilities.length" class="checkbox-label deletion-confirm"><input v-model="removeFacilities" type="checkbox" :disabled="busy"/>一并删除 {{facilities.length}} 项柜内设施标记</label>
 <p v-if="whole" class="delete-explanation">本模块的机柜和设施标记将一并删除，机房、其他模块与布局模板保留。</p>
 <label v-if="actualMode==='cascade'" class="checkbox-label deletion-confirm"><input v-model="confirmed" type="checkbox" :disabled="busy"/>我确认删除上述设备及关联连接</label>
 <div class="modal-actions deletion-actions"><div class="deletion-feedback"><p v-if="error" ref="alert" tabindex="-1" class="error-banner" role="alert">{{error}}</p><span v-else-if="busy" role="status">正在处理，请稍候…</span><span v-else class="muted small">一次完成，变更明细保留在审计中</span></div><div><button type="button" class="secondary" :disabled="busy" @click="emit('close')">取消删除</button><button class="danger" :disabled="busy||!canSubmit">{{whole?'确认删除微模块':'确认删除机柜'}}</button></div></div>
 </form></Modal></template>
