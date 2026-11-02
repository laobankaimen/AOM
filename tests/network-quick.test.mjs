import {readFileSync,mkdirSync} from 'node:fs';import path from 'node:path';import {pathToFileURL} from 'node:url';import test from 'node:test';import assert from 'node:assert/strict';
import {build} from 'esbuild';import {parse,compileScript} from '@vue/compiler-sfc';import {createSSRApp,h,reactive} from 'vue';import {renderToString} from '@vue/server-renderer';
mkdirSync('.cache',{recursive:true});
const output=path.resolve('.cache/network-components-ssr.mjs');
await build({stdin:{contents:"export {default as Graph} from './src/TopologyGraph.vue';export {default as Quick} from './src/NetworkQuickPanel.vue';",resolveDir:process.cwd()},bundle:true,platform:'node',format:'esm',outfile:output,external:['vue','lucide-vue-next'],plugins:[{name:'vue-ssr-check',setup(b){b.onLoad({filter:/\.vue$/},args=>{const {descriptor}=parse(readFileSync(args.path,'utf8'),{filename:args.path});return {contents:compileScript(descriptor,{id:args.path,inlineTemplate:true}).content,loader:'ts',resolveDir:path.dirname(args.path)}})}}]});
const {Quick,Graph}=await import(pathToFileURL(output).href);
const network={id:'n',projectId:'p',name:'数据网络',purpose:'business',color:'#497cda',deviceIds:['a','b'],vlan:0,cidr:'',gateway:'',note:''};
const state={racks:[],projects:[{id:'p',name:'项目'}],sites:[{id:'r',projectId:'p'}],modules:[{id:'m',siteId:'r',name:'模块'}],devices:[{id:'a',name:'交换机A',modelId:'switch',moduleId:'m',assetTag:'a'},{id:'b',name:'服务器B',modelId:'server',moduleId:'m',assetTag:'b'}],models:[{id:'switch',name:'接入交换机',role:'access'},{id:'server',name:'计算型号',role:'server'}],networks:[network],links:[],revision:1};
test('quick forms render Vue reactive edit data and actionable duplicate feedback without opening an editor',async()=>{
 const html=await renderToString(createSSRApp({render:()=>h(Quick,{mode:'network',state,projectId:'p',initial:reactive({...network,id:''})})}));
 assert.match(html,/项目中已有/);assert.match(html,/打开已有网络/);assert.match(html,/aria-invalid="true"/);assert.ok(!html.includes('editor-page'));
 const edit=await renderToString(createSSRApp({render:()=>h(Quick,{mode:'network',state,projectId:'p',initial:reactive(network)})}));assert.ok(!edit.includes('项目中已有'));assert.match(edit,/保存网络/);
 const batch=await renderToString(createSSRApp({render:()=>h(Quick,{mode:'links',state,projectId:'p',network})}));assert.match(batch,/选择上联 交换机A/);assert.match(batch,/确认建立连接/);
});
test('merged graph renders counts, selectable cards and finite geometry',async()=>{
 const devices=[...state.devices,{...state.devices[1],id:'c',name:'服务器C'}],net={...network,deviceIds:['a','b','c']};
 const html=await renderToString(createSSRApp({render:()=>h(Graph,{state:{...state,devices,networks:[net]},devices,networks:[net],links:[{id:'l',sourceId:'a',targetId:'b',kind:'logical',status:'pending',networkIds:['n']}],networkId:'',selectedDevice:'',selectedLink:'',zoom:85,merge:true})}));
 assert.match(html,/设备连接与网络分区图/);assert.match(html,/查看拓扑设备 服务器B/);assert.match(html,/查看拓扑设备 服务器C/);assert.ok(!html.includes('展开 计算型号 2台'));assert.ok(!html.includes('NaN'));assert.ok(!html.includes('undefined'));assert.match(html,/stroke-dasharray="5 5"/);
});

test('device graph renders only populated networks and a full-size finite viewport',async()=>{
 const servers=Array.from({length:3},(_,i)=>({...state.devices[1],id:'srv'+i,name:'服务器'+i}));
 const net={...network,deviceIds:servers.map(d=>d.id)};
 const html=await renderToString(createSSRApp({render:()=>h(Graph,{state:{...state,devices:servers},devices:servers,networks:[{...network,id:'empty',name:'空网络',deviceIds:[]},net],links:[],networkId:'',selectedDevice:'',selectedLink:'',zoom:85})}));
 assert.ok(!html.includes('data-topology-lane="empty"'));assert.match(html,/data-topology-lane="n"/);
 const bounds=html.match(/viewBox="0 0 ([\d.]+) ([\d.]+)"/);assert.ok(bounds);assert.ok(Number(bounds[2])>=300);assert.ok(!html.includes('NaN'));
});
