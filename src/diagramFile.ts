/** Bound raster allocations while retaining a vector export for very large drawings. */
export function rasterSize(width:number,height:number){
 if(!Number.isFinite(width)||!Number.isFinite(height)||width<=0||height<=0)throw new Error('图形尺寸无效，请重新打开图形后导出');
 const scale=Math.min(2,16384/width,16384/height,Math.sqrt(24000000/(width*height)));
 return {width:Math.max(1,Math.floor(width*scale)),height:Math.max(1,Math.floor(height*scale)),reduced:scale<2};
}
export function diagramFilename(title:string){return (title.replace(/[\\/:*?"<>|\u0000-\u001f]/g,'_').trim().slice(0,100)||'图形导出')}
/** A one-page PDF containing the browser-rendered JPEG; offsets are byte offsets. */
export function imagePDF(jpeg:Uint8Array,width:number,height:number):Uint8Array<ArrayBuffer>{
 if(!Number.isInteger(width)||!Number.isInteger(height)||width<1||height<1||jpeg[0]!==255||jpeg[1]!==216)throw new Error('无法生成PDF：图像数据无效');
 const encode=(s:string)=>new TextEncoder().encode(s),chunks:Uint8Array[]=[],offsets=[0];let size=0;
 const add=(chunk:string|Uint8Array)=>{const bytes=typeof chunk==='string'?encode(chunk):chunk;chunks.push(bytes);size+=bytes.length};
 const factor=Math.min(.5,14000/Math.max(width,height)),w=+(width*factor).toFixed(3),h=+(height*factor).toFixed(3);
 const object=(id:number,body:string)=>{offsets[id]=size;add(`${id} 0 obj\n${body}\nendobj\n`)};
 add('%PDF-1.4\n');add(new Uint8Array([37,226,227,207,211,10]));
 object(1,'<< /Type /Catalog /Pages 2 0 R >>');
 object(2,'<< /Type /Pages /Kids [3 0 R] /Count 1 >>');
 object(3,`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${w} ${h}] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>`);
 offsets[4]=size;add(`4 0 obj\n<< /Type /XObject /Subtype /Image /Width ${width} /Height ${height} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>\nstream\n`);add(jpeg);add('\nendstream\nendobj\n');
 const content=`q\n${w} 0 0 ${h} 0 0 cm\n/Im0 Do\nQ\n`;
 object(5,`<< /Length ${encode(content).length} >>\nstream\n${content}endstream`);
 const xref=size;add('xref\n0 6\n0000000000 65535 f \n');for(let i=1;i<=5;i++)add(`${String(offsets[i]).padStart(10,'0')} 00000 n \n`);
 add(`trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`);
 const result=new Uint8Array(size);let at=0;for(const chunk of chunks){result.set(chunk,at);at+=chunk.length}return result;
}
