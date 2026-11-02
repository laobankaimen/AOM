import {nextTick,onMounted,onBeforeUnmount,watch,type Ref} from 'vue';

type Editor = {dirty:()=>boolean;busy:()=>boolean;accept:()=>void};
const editors=new Set<Editor>();
export function requestLeaveEditors(){
 if([...editors].some(e=>e.busy()))return false;
 const changed=[...editors].filter(e=>e.dirty());
 if(!changed.length)return true;
 if(!window.confirm('有尚未保存的修改。确定放弃修改并离开？\n选择“取消”继续编辑，也可以先保存再返回。'))return false;
 changed.forEach(e=>e.accept());return true;
}
export function useEditorDraft(draft:()=>unknown,busy:()=>boolean=()=>false){
 let baseline=JSON.stringify(draft());
 const dirty=()=>JSON.stringify(draft())!==baseline;
 const accept=()=>{baseline=JSON.stringify(draft())};
 const entry={dirty,busy,accept};
 onMounted(()=>{accept();editors.add(entry)});onBeforeUnmount(()=>editors.delete(entry));
 return {accept,dirty};
}
export function useEditorBoundary(root:Ref<HTMLElement|undefined>,busy:()=>boolean){
 const entry={dirty:()=>false,busy,accept:()=>{}};
 let observer:MutationObserver|undefined,lastError='';
 function feedback(){const alert=root.value?.querySelector<HTMLElement>('.error-banner[role="alert"]');const text=alert?.textContent?.trim()||'';if(text&&text!==lastError){lastError=text;void nextTick(()=>{if(alert?.isConnected){alert.tabIndex=-1;alert.focus();alert.scrollIntoView({block:'nearest'})}})}if(!text)lastError=''}
 const selectValues=new WeakMap<HTMLSelectElement,string>();
 function remember(event:Event){const select=event.target;if(select instanceof HTMLSelectElement&&select.matches('.project-picker select,.topology-actions select'))selectValues.set(select,select.value)}
 function change(event:Event){const select=event.target;if(!(select instanceof HTMLSelectElement)||!select.matches('.project-picker select,.topology-actions select')||root.value?.contains(select))return;if(!requestLeaveEditors()){event.stopImmediatePropagation();select.value=selectValues.get(select)||''}else selectValues.set(select,select.value)}
 function click(event:MouseEvent){
  const target=(event.target as Element).closest('button,a');if(!target||target.closest('[disabled],[inert]'))return;
  if(event.ctrlKey||event.metaKey||event.shiftKey||event.altKey||event.button!==0)return;
  const inside=root.value?.contains(target);
  const leaving=inside?(target.hasAttribute('data-editor-exit')||/^(取消|返回查看|关闭快捷操作)$/.test(target.getAttribute('aria-label')||target.textContent?.trim()||'')):!!target.closest('nav,.sidebar,.account-menu [role=menuitem]');
  if(leaving&&!requestLeaveEditors()){event.preventDefault();event.stopImmediatePropagation()}
 }
 function beforeUnload(event:BeforeUnloadEvent){if([...editors].some(e=>e.dirty()||e.busy())){event.preventDefault();event.returnValue=''}}
 onMounted(()=>{editors.add(entry);document.querySelectorAll<HTMLSelectElement>('.project-picker select,.topology-actions select').forEach(select=>selectValues.set(select,select.value));observer=new MutationObserver(feedback);if(root.value)observer.observe(root.value,{childList:true,subtree:true,characterData:true});document.addEventListener('click',click,true);document.addEventListener('focusin',remember,true);document.addEventListener('pointerdown',remember,true);document.addEventListener('keydown',remember,true);document.addEventListener('change',change,true);window.addEventListener('beforeunload',beforeUnload)});
 onBeforeUnmount(()=>{observer?.disconnect();editors.delete(entry);document.removeEventListener('click',click,true);document.removeEventListener('focusin',remember,true);document.removeEventListener('pointerdown',remember,true);document.removeEventListener('keydown',remember,true);document.removeEventListener('change',change,true);window.removeEventListener('beforeunload',beforeUnload)});
}
// Server-side validation has no stable field codes yet. Keep its summary focused
// and associated with the form; specific client validation belongs at the field.
export function useFormFeedback(root:Ref<HTMLElement|undefined>,error:()=>string){
 watch(error,async value=>{if(!value)return;await nextTick();const alert=root.value?.querySelector<HTMLElement>('[role="alert"]');if(alert){alert.tabIndex=-1;alert.focus();alert.scrollIntoView({block:'nearest'})}});
}
