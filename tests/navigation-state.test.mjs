import {readFileSync} from 'node:fs';import assert from 'node:assert/strict';import test from 'node:test';import ts from 'typescript';
const js=ts.transpileModule(readFileSync(new URL('../src/navigationState.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
const {parseNavigation,navigationHash}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
test('network routes carry project and network only, never physical context',()=>{
 const route={view:'topology',project:'项目 a&b',room:'r',module:'m',rack:'rack',network:'业务网络/子网'};
 const hash=navigationHash(route);assert.ok(!hash.includes('room='));assert.ok(!hash.includes('rack='));assert.deepEqual(parseNavigation(hash),{...route,room:'',module:'',rack:''});
});
test('legacy hashes work and physical routes preserve drill-down identifiers',()=>{
 assert.equal(parseNavigation('#topology').view,'topology');assert.equal(parseNavigation('#missing').view,'overview');
 const route={view:'elevation',project:'p',room:'r',module:'m',rack:'c',network:''};assert.deepEqual(parseNavigation(navigationHash(route)),route);
 assert.equal(navigationHash({...route,view:'models'}),'#models?project=p');
});
test('asset filters and topology context round-trip without leaking physical context',()=>{
 const assets={view:'assets',project:'p',room:'r',module:'',rack:'',network:'',query:'服务器 & 1',status:'pending',page:3};assert.deepEqual(parseNavigation(navigationHash(assets)),assets);
 const topology={view:'topology',project:'p',room:'',module:'',rack:'',network:'n',query:'device',status:'pending',filterRoom:'r',device:'d',link:'l',unassigned:true,searchOnly:true,presentation:'detail',mode:'links'};assert.deepEqual(parseNavigation(navigationHash(topology)),topology);
 assert.equal(parseNavigation('#assets?page=-5').page,1);assert.equal(parseNavigation('#assets?page=oops').page,1);
});
