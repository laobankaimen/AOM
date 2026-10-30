<script setup lang="ts">
import {nextTick,onMounted,onBeforeUnmount,ref} from 'vue';
import {UserRound,ChevronDown,KeyRound,LogOut,Info,ShieldCheck} from 'lucide-vue-next';
import Modal from './Modal.vue';import {systemInfo} from './systemInfo';
const props=defineProps<{username:string;busy?:boolean;error?:string}>();
const emit=defineEmits<{password:[];logout:[]}>();
const root=ref<HTMLElement>(),trigger=ref<HTMLButtonElement>(),menu=ref<HTMLElement>();
const open=ref(false),about=ref(false);
function close(restore=false){open.value=false;if(restore)void nextTick(()=>trigger.value?.focus())}
async function toggle(){if(open.value){close(true);return}open.value=true;await nextTick();menu.value?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()}
function choose(action:'password'|'logout'|'about'){close();if(action==='about')about.value=true;else if(action==='password')emit('password');else emit('logout')}
function outside(event:PointerEvent){if(open.value&&!root.value?.contains(event.target as Node))close()}
function focusOut(event:FocusEvent){if(open.value&&event.relatedTarget&&!root.value?.contains(event.relatedTarget as Node))close()}
function key(event:KeyboardEvent){if(event.key==='Escape'){event.preventDefault();close(true);return}if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return;event.preventDefault();const items=[...menu.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')||[]];if(!items.length)return;const i=items.indexOf(document.activeElement as HTMLButtonElement);const next=event.key==='Home'?0:event.key==='End'?items.length-1:(i+(event.key==='ArrowUp'?-1:1)+items.length)%items.length;items[next]?.focus()}
async function closeAbout(){about.value=false;await nextTick();trigger.value?.focus()}
onMounted(()=>document.addEventListener('pointerdown',outside));onBeforeUnmount(()=>document.removeEventListener('pointerdown',outside));
</script>
<template>
 <div ref="root" class="account-menu" @focusout="focusOut">
  <button ref="trigger" class="account-trigger" aria-label="用户菜单" aria-haspopup="menu" :aria-expanded="open" aria-controls="account-menu-items" @click="toggle" @keydown.down.prevent="!open&&toggle()" @keydown.esc.prevent="close(true)">
   <span class="account-avatar"><UserRound :size="18"/></span><span class="account-name">{{username}}</span><ChevronDown :size="13"/>
  </button>
  <div v-if="open" ref="menu" id="account-menu-items" class="account-dropdown" role="menu" aria-label="账户操作" @keydown="key">
   <div class="account-menu-heading" role="presentation"><strong>{{username}}</strong><span><ShieldCheck :size="12"/>管理员 · 已登录</span></div>
   <button role="menuitem" :disabled="busy" @click="choose('password')"><KeyRound :size="16"/>修改登录密码</button>
   <button role="menuitem" @click="choose('about')"><Info :size="16"/>系统信息</button>
   <div class="account-menu-separator" role="separator"></div>
   <button role="menuitem" class="account-logout" :disabled="busy" @click="choose('logout')"><LogOut :size="16"/>退出登录</button>
  </div>
  <div v-if="error" class="account-action-error" role="alert">{{error}}</div>
 </div>
 <Modal v-if="about" title="系统信息" @close="closeAbout"><div class="system-information"><div class="system-app-mark"><Info :size="26"/></div><h3>{{systemInfo.name}}</h3><dl><div><dt>版本号</dt><dd data-system-version>{{systemInfo.version}}</dd></div><div><dt>开发者</dt><dd>{{systemInfo.developer}}</dd></div><div><dt>公司</dt><dd>{{systemInfo.company}}</dd></div></dl><div class="modal-actions"><span></span><button class="primary" @click="closeAbout">关闭</button></div></div></Modal>
</template>
