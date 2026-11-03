import {onMounted,onBeforeUnmount,onActivated,onDeactivated} from 'vue';
export interface Shortcut {key:string;alt?:boolean;command?:boolean;run:()=>void}
export function useWorkspaceShortcuts(enabled:()=>boolean,shortcuts:Shortcut[]){
 function keydown(event:KeyboardEvent){
  if(event.defaultPrevented||event.repeat||event.isComposing||!enabled())return;
  const target=event.target as Element|null,editing=!!target?.closest('input,textarea,select,[contenteditable="true"],[role="textbox"]');
  for(const shortcut of shortcuts){
   if(event.key.toLowerCase()!==shortcut.key.toLowerCase()||!!event.altKey!==!!shortcut.alt||!!(event.ctrlKey||event.metaKey)!==!!shortcut.command)continue;
   if(event.shiftKey&&shortcut.key!=='?')continue;
   if(editing&&!shortcut.command&&shortcut.key!=='Escape')continue;
   event.preventDefault();shortcut.run();return;
  }
 }
 const attach=()=>window.addEventListener('keydown',keydown),detach=()=>window.removeEventListener('keydown',keydown);
 onMounted(attach);onActivated(attach);onDeactivated(detach);onBeforeUnmount(detach);
}
