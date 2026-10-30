export const workspaceViews=['overview','topology','assets','models','templates','audit','room','module','elevation'];
export interface NavigationState {view:string;project:string;room:string;module:string;rack:string;network:string}
export function parseNavigation(hash:string):NavigationState{
 const [name,params='']=hash.replace(/^#/,'').split('?'),p=new URLSearchParams(params);
 return {view:workspaceViews.includes(name)?name:'overview',project:p.get('project')||'',room:p.get('room')||'',module:p.get('module')||'',rack:p.get('rack')||'',network:p.get('network')||''};
}
export function navigationHash(n:NavigationState){
 const p=new URLSearchParams();if(n.project)p.set('project',n.project);
 if(['room','module','elevation','assets'].includes(n.view)&&n.room)p.set('room',n.room);
 if(['module','elevation'].includes(n.view)&&n.module)p.set('module',n.module);
 if(n.view==='elevation'&&n.rack)p.set('rack',n.rack);
 if(n.view==='topology'&&n.network)p.set('network',n.network);
 return '#'+n.view+(p.size?'?'+p:'');
}
