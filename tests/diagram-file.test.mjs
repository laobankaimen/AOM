import {readFileSync} from 'node:fs';import assert from 'node:assert/strict';import test from 'node:test';import ts from 'typescript';
const js=ts.transpileModule(readFileSync(new URL('../src/diagramFile.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
const {rasterSize,diagramFilename,imagePDF}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
test('export dimensions retain aspect ratio and cap canvas memory for wide and tall drawings',()=>{
 assert.deepEqual(rasterSize(900,600),{width:1800,height:1200,reduced:false});
 for(const [w,h]of [[100000,1000],[600,100000],[20000,20000]]){const size=rasterSize(w,h);assert.ok(size.width<=16384&&size.height<=16384);assert.ok(size.width*size.height<=24000000);assert.ok(size.reduced);assert.ok(Math.abs(size.width/w-size.height/h)<.002)}
 assert.throws(()=>rasterSize(NaN,500));assert.throws(()=>rasterSize(500,0));
});
test('export filenames preserve Chinese and remove path/control characters',()=>{assert.equal(diagramFilename('项目/机房:图'),'项目_机房_图');assert.equal(diagramFilename('  '),'图形导出');assert.ok(!diagramFilename('a\nb').includes('\n'))});
test('single page PDF stores exact JPEG stream and valid byte offsets including binary input',()=>{
 const jpeg=new Uint8Array([255,216,0,255,224,128,13,10,255,217]);const pdf=imagePDF(jpeg,2000,1200);const bytes=Buffer.from(pdf),source=bytes.toString('latin1');
 assert.ok(source.startsWith('%PDF-1.4'));assert.match(source,/\/Count 1/);assert.match(source,/\/MediaBox \[0 0 1000 600\]/);
 const xref=Number(source.match(/startxref\n(\d+)/)[1]);assert.equal(bytes.subarray(xref,xref+4).toString(),'xref');
 const entries=source.slice(xref).split('\n').slice(3,8);for(let i=0;i<5;i++){const at=Number(entries[i].slice(0,10));assert.ok(bytes.subarray(at,at+16).toString().startsWith(`${i+1} 0 obj`))}
 const start=source.indexOf('stream\n')+7;assert.deepEqual(bytes.subarray(start,start+jpeg.length),Buffer.from(jpeg));
 assert.throws(()=>imagePDF(new Uint8Array([0]),10,10));assert.throws(()=>imagePDF(jpeg,0,10));
});
