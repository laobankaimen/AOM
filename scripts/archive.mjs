import { readFileSync, readdirSync, statSync, writeFileSync, renameSync } from 'node:fs';
import path from 'node:path';
import { deflateRawSync } from 'node:zlib';

// Small UTF-8 ZIP writer for the explicit release manifest (no ZIP64 needed).
// Set UTF-8 in both headers so Windows and standard ZIP readers retain Chinese names.
const table=Uint32Array.from({length:256},(_,n)=>{for(let k=0;k<8;k++)n=n&1?0xedb88320^(n>>>1):n>>>1;return n>>>0;});
function crc32(bytes){let crc=0xffffffff;for(const b of bytes)crc=table[(crc^b)&255]^(crc>>>8);return (crc^0xffffffff)>>>0;}
function dosTime(date){return {time:(date.getHours()<<11)|(date.getMinutes()<<5)|(date.getSeconds()>>1),date:((Math.max(1980,date.getFullYear())-1980)<<9)|((date.getMonth()+1)<<5)|date.getDate()};}
for(const platform of ['linux','windows']){
 const base=`aom-${platform}-amd64`,directory=path.join('release',base);
 const binary=platform==='windows'?'aom.exe':'aom',suffix=platform==='windows'?'cmd':'sh';
 const files=[binary,`start-local.${suffix}`,`start-lan.${suffix}`,'开始使用.txt','SHA256SUMS.txt','licenses/elkjs/LICENSE.md','licenses/elkjs/NOTICE.txt','licenses/elkjs/elk.bundled.js'];
 for(const sub of ['docs','examples'])for(const name of readdirSync(sub).sort())if(statSync(path.join(sub,name)).isFile() && (sub !== 'docs' || name.endsWith('.md')))files.push(`${sub}/${name}`);
 const locals=[],central=[];let offset=0;
 for(const file of files){
  const localPath=path.join(directory,file),stat=statSync(localPath),name=Buffer.from(`${base}/${file}`,'utf8');
  const data=readFileSync(localPath),packed=deflateRawSync(data),crc=crc32(data),stamp=dosTime(stat.mtime);
  const header=Buffer.alloc(30);header.writeUInt32LE(0x04034b50,0);header.writeUInt16LE(20,4);header.writeUInt16LE(0x800,6);header.writeUInt16LE(8,8);header.writeUInt16LE(stamp.time,10);header.writeUInt16LE(stamp.date,12);header.writeUInt32LE(crc,14);header.writeUInt32LE(packed.length,18);header.writeUInt32LE(data.length,22);header.writeUInt16LE(name.length,26);
  const record=Buffer.alloc(46);record.writeUInt32LE(0x02014b50,0);record.writeUInt16LE(0x0314,4);record.writeUInt16LE(20,6);record.writeUInt16LE(0x800,8);record.writeUInt16LE(8,10);record.writeUInt16LE(stamp.time,12);record.writeUInt16LE(stamp.date,14);record.writeUInt32LE(crc,16);record.writeUInt32LE(packed.length,20);record.writeUInt32LE(data.length,24);record.writeUInt16LE(name.length,28);record.writeUInt32LE(((stat.mode&0xffff)*0x10000)>>>0,38);record.writeUInt32LE(offset,42);
  locals.push(header,name,packed);central.push(record,name);offset+=header.length+name.length+packed.length;
 }
 const index=Buffer.concat(central),end=Buffer.alloc(22);end.writeUInt32LE(0x06054b50,0);end.writeUInt16LE(files.length,8);end.writeUInt16LE(files.length,10);end.writeUInt32LE(index.length,12);end.writeUInt32LE(offset,16);
 const target=path.join('release',`${base}.zip`),temporary=`${target}.tmp`;
 writeFileSync(temporary,Buffer.concat([...locals,index,end]));renameSync(temporary,target);
 console.log(`${target}: ${files.length} files, UTF-8 names`);
}
