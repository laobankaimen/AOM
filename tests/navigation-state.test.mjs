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
