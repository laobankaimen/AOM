<script setup lang="ts">
import {ref,computed} from 'vue';
import {request} from './model';
import type {State,DeviceRole} from './model';
import {builtinRoles,roleDefinitions} from './deviceRoles';
const props=defineProps<{state:State}>();const emit=defineEmits<{saved:[state:State];busy:[value:boolean]}>();
const roles=ref<DeviceRole[]>(roleDefinitions(props.state.deviceRoles)),busy=ref(false),error=ref('');
const revision=props.state.revision;
const builtIn=(id:string)=>builtinRoles.some(r=>r.id===id);
const count=(id:string)=>props.state.models.filter(m=>m.role===id).length;
const draft=computed(()=>roles.value);defineExpose({draft});
function add(){roles.value.push({id:'role-'+crypto.randomUUID(),name:'',color:'#64748b',baseRole:'infra'})}
async function save(){if(busy.value)return;busy.value=true;emit('busy',true);error.value='';try{emit('saved',await request<State>('/device-roles','PUT',{revision,roles:roles.value}))}catch(e){error.value=(e as Error).message}finally{busy.value=false;emit('busy',false)}}
</script>
<template><form class="role-manager" @submit.prevent="save"><p class="panel-note">修改角色名称会同步显示到所有引用型号。自定义角色的拓扑分类决定自动分层与连接推演，分类本身不修改现有连接。</p><p v-if="error" role="alert" class="error-banner">{{error}}</p><div class="table-scroll"><table><thead><tr><th>角色名称</th><th>颜色</th><th>拓扑分类</th><th>引用型号</th><th>操作</th></tr></thead><tbody><tr v-for="(r,i) in roles" :key="r.id"><td><input v-model="r.name" :aria-label="'角色名称 '+(i+1)" required maxlength="40" autocomplete="off"/></td><td><input v-model="r.color" type="color" :aria-label="'角色颜色 '+(i+1)"/></td><td><select v-model="r.baseRole" :disabled="builtIn(r.id)" :aria-label="'角色拓扑分类 '+(i+1)"><option v-for="base in builtinRoles" :key="base.id" :value="base.id">{{base.name}}</option></select></td><td>{{count(r.id)}} 个</td><td><button v-if="!builtIn(r.id)" type="button" class="text-button danger-text" :disabled="!!count(r.id)||busy" :title="count(r.id)?'仍有型号使用此角色，先调整型号角色':''" @click="roles.splice(i,1)">移除</button><span v-else class="muted small">内置</span></td></tr></tbody></table></div><div class="modal-actions"><button type="button" class="secondary" :disabled="busy||roles.length>=128" @click="add">新增角色</button><button class="primary" :disabled="busy">{{busy?'正在保存…':'保存角色'}}</button></div></form></template>
