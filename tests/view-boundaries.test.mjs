import {readFileSync} from 'node:fs';import test from 'node:test';import assert from 'node:assert/strict';import {parse} from '@vue/compiler-sfc';import {compile} from '@vue/compiler-dom';import {renderToString} from '@vue/server-renderer';import * as Vue from 'vue';import ts from 'typescript';
const source=readFileSync(new URL('../src/App.vue',import.meta.url),'utf8');
const template=parse(source).descriptor.template.content;
const names=[...new Set([...template.matchAll(/<([A-Z]\w*)\b/g)].map(m=>m[1]))];
const handlers=Object.fromEntries([...source.matchAll(/function\s+(\w+)\s*\(/g)].map(m=>[m[1],()=>{}]));
const stub=name=>Vue.defineComponent({setup:(_, {slots})=>()=>Vue.h('div',{'data-test-component':name},['ActionMenu','KeepAlive'].includes(name)?slots.default?.():undefined)});
const components=Object.fromEntries(names.filter(n=>n!=='Transition').map(n=>[n,stub(n)]));
const code=compile(template,{mode:'function',prefixIdentifiers:true,expressionPlugins:['typescript']}).code;
const render=new Function('Vue',ts.transpileModule(code,{compilerOptions:{module:ts.ModuleKind.None,target:ts.ScriptTarget.ES2022}}).outputText)(Vue);
async function renderView(view,overrides={}){const project={id:'p',name:'项目'},room={id:'s',name:'机房',projectId:'p'},module={id:'m',name:'模块',siteId:'s',type:'single',rackLeft:1,rackRight:0};const state={templates:[],projects:[project],sites:[room],modules:[module],racks:[],infra:[],models:[],devices:[],networks:[],links:[],revision:1};
 const context={...handlers,topologyNavigation:{path:[],busy:false},topologyNetworkID:'',navigationBusy:false,topologyReturn:false,display:{details:true,tools:true},displayClasses:{},state,view,title:view,projects:state.projects,sites:state.sites,projectID:'p',siteID:'s',moduleID:'m',currentProject:project,currentSite:room,currentModule:module,breadcrumbRoom:room,breadcrumbModule:module,breadcrumbRack:null,navs:[{id:'overview',name:'空间总览'},{id:'topology',name:'网络拓扑'}],activeNav:['overview','room','module','elevation'].includes(view)?'overview':view,spatialView:['overview','room','module','elevation'].includes(view),loading:false,loadError:'',modules:[module],siteRacks:[],siteDevices:[],totalUsed:0,usage:0,editingPage:false,topologyEditTitle:'',modal:'',toast:'',audits:[],filteredModels:[],pageDevices:[],filteredDevices:[],assetPage:1,statuses:{},query:'',statusFilter:'',roleNames:{},pending:[],racks:[],shownRacks:[],moduleDevices:[],selectedInfra:null,currentRack:null,selectedDevice:null,rackID:'',deviceID:'',zoom:100,busy:false,compareMode:false,elevationScale:90,rackRow:'all',dragDevices:{active:{value:null},target:{value:null},height:{value:0}}};
 Object.assign(context,overrides);const app=Vue.createSSRApp(Vue.defineComponent({components,setup:()=>context,render}));app.config.warnHandler=()=>{};return renderToString(app);
}
function count(html,name){return (html.match(new RegExp(`data-test-component="${name}"`,'g'))||[]).length}
test('home mounts only overview, while room and module expand separate layers',async()=>{
 const home=await renderView('overview');assert.equal(count(home,'ProjectOverview'),1);for(const name of ['RoomOverview','FloorPlan','RackElevation','TopologyView'])assert.equal(count(home,name),0);assert.ok(!home.includes('class="context-bar"'));assert.ok(!home.includes('class="detail-panel"'));
 const room=await renderView('room');assert.equal(count(room,'RoomOverview'),1);assert.equal(count(room,'FloorPlan'),0);assert.equal(count(room,'ProjectOverview'),0);
 const module=await renderView('module');assert.equal(count(module,'FloorPlan'),1);assert.equal(count(module,'RoomOverview'),0);assert.equal(count(module,'ProjectOverview'),0);
});
test('topology has one workspace in content, never in header actions',async()=>{const html=await renderView('topology');assert.equal(count(html,'TopologyView'),1);const header=html.slice(html.indexOf('class="page-heading"'),html.indexOf('</section>',html.indexOf('class="page-heading"')));assert.equal(count(header,'TopologyView'),0)});

test("template library has one content mount and home emphasizes initialization",async()=>{const html=await renderView("templates");assert.equal(count(html,"TemplateLibrary"),1);const header=html.slice(html.indexOf('class="page-heading"'),html.indexOf('data-test-component="TemplateLibrary"'));assert.equal(count(header,"TemplateLibrary"),0);const home=await renderView("overview");assert.match(home,/<button class="primary"[^>]*>[\s\S]*?初始化机房<\/button>/)});

test('navigation has one spatial/network entry and module actions belong to its scope',async()=>{
 for(const view of ['overview','room','module','elevation','topology']){
  const html=await renderView(view);
  assert.ok(!html.includes('workspace-view-tabs'));
  assert.ok(!html.includes('module-tabs'));
  const active=view==='topology'?'网络拓扑':'空间总览';
  assert.match(html,new RegExp(`aria-label="${active}"[^>]*aria-current="page"`));
 }
 const module=await renderView('module');
 const header=module.slice(module.indexOf('class="page-heading"'),module.indexOf('</section>',module.indexOf('class="page-heading"')));
 assert.ok(header.includes('导入设备'));assert.ok(header.includes('编辑微模块'));
 assert.ok(!header.includes('创建机房'));assert.ok(!module.includes('aria-label="选择机房"'));
 assert.ok(module.includes('选择机柜或设施'));
 for(const view of ['models','audit','templates','assets']){
  const html=await renderView(view);const nav=html.slice(html.indexOf('aria-label="位置导航"'),html.indexOf('</nav>',html.indexOf('aria-label="位置导航"')));
  assert.ok(!nav.includes('返回机房'));
 }
});
