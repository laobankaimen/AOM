export const workspaceViews=['overview','topology','assets','models','templates','audit','room','module','elevation'];
export interface TopologyRouteState {query?:string;status?:string;filterRoom?:string;device?:string;link?:string;unassigned?:boolean;searchOnly?:boolean;presentation?:string;mode?:string}
export interface NavigationState extends TopologyRouteState {view:string;project:string;room:string;module:string;rack:string;network:string;query?:string;status?:string;page?:number;device?:string}
export function parseNavigation(hash:string):NavigationState{
 const [name,params='']=hash.replace(/^#/,'').split('?'),p=new URLSearchParams(params);
 const detail:Partial<NavigationState>={};if(name==='topology'){for(const key of ['filterRoom','link','presentation','mode'] as const){if(p.has(key))detail[key]=p.get(key)||''}if(p.has('unassigned'))detail.unassigned=p.get('unassigned')==='1';if(p.has('searchOnly'))detail.searchOnly=p.get('searchOnly')==='1'}if(p.has('q'))detail.query=p.get('q')||'';if(p.has('status'))detail.status=p.get('status')||'';if(p.has('page'))detail.page=Math.max(1,Number.parseInt(p.get('page')||'1',10)||1);if(p.has('device'))detail.device=p.get('device')||'';
 return {...detail,view:workspaceViews.includes(name)?name:'overview',project:p.get('project')||'',room:p.get('room')||'',module:p.get('module')||'',rack:p.get('rack')||'',network:p.get('network')||''};
}
export function navigationHash(n:NavigationState){
 const p=new URLSearchParams();if(n.project)p.set('project',n.project);
 if(['room','module','elevation','assets'].includes(n.view)&&n.room)p.set('room',n.room);
 if(['module','elevation'].includes(n.view)&&n.module)p.set('module',n.module);
 if(n.view==='elevation'&&n.rack)p.set('rack',n.rack);
 if(n.view==='topology'&&n.network)p.set('network',n.network);
 if(['assets','models','topology'].includes(n.view)&&n.query)p.set('q',n.query);
 if(n.view==='assets'){if(n.status)p.set('status',n.status);if(n.page&&n.page>1)p.set('page',String(n.page))}
 if(n.view==='topology'){if(n.status)p.set('status',n.status);for(const key of ['filterRoom','link','presentation','mode'] as const){if(n[key])p.set(key,n[key]!)}if(typeof n.unassigned==='boolean')p.set('unassigned',n.unassigned?'1':'0');if(n.searchOnly)p.set('searchOnly','1')}
 if(['assets','elevation','topology'].includes(n.view)&&n.device)p.set('device',n.device);
 return '#'+n.view+(p.size?'?'+p:'');
}
