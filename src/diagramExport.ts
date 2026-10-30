import {diagramFilename,imagePDF,rasterSize} from './diagramFile';
const NS='http://www.w3.org/2000/svg';
const styles=['fill','fill-opacity','stroke','stroke-width','stroke-opacity','stroke-dasharray','stroke-linecap','stroke-linejoin','opacity','font-family','font-size','font-weight','font-style','letter-spacing','text-anchor','dominant-baseline','visibility','display','paint-order'];
function element(name:string,attrs:Record<string,string|number>={}){const el=document.createElementNS(NS,name);for(const [k,v]of Object.entries(attrs))el.setAttribute(k,String(v));return el}
function text(value:string,x:number,y:number,size:number,color='#18324f'){const el=element('text',{x,y,'font-size':size,fill:color});el.textContent=value;return el}
function copySVG(source:SVGSVGElement,index:number){
 const clone=source.cloneNode(true) as SVGSVGElement;
 const originals=[source,...source.querySelectorAll<SVGElement>('*')],copies=[clone,...clone.querySelectorAll<SVGElement>('*')];
 for(let i=0;i<originals.length;i++){
  const original=originals[i],copy=copies[i],css=getComputedStyle(original);copy.removeAttribute('style');
  for(const key of styles)copy.style.setProperty(key,css.getPropertyValue(key));
  copy.removeAttribute('tabindex');copy.removeAttribute('role');
  for(const attr of [...copy.attributes])if(attr.name.startsWith('on'))copy.removeAttribute(attr.name);
  // Inline resolved colours, but keep SVG-local gradient/filter references local.
  for(const key of ['fill','stroke']){const value=copy.style.getPropertyValue(key);if(value.includes('url(')){const match=value.match(/#([^\s)'"]+)/);if(match)copy.style.setProperty(key,`url(#${match[1]})`)}}
 }
 clone.querySelectorAll('.layout-drop-indicator,.object-selection-badge,.link-drag-handle,script,foreignObject').forEach(el=>el.remove());
 // Export all relationships at normal opacity, even when a node is selected.
 clone.querySelectorAll('.topology-edge').forEach(el=>{(el as SVGElement).style.opacity='1'});
 const ids=new Map<string,string>();clone.querySelectorAll('[id]').forEach(el=>{const id=el.id;const next=`export-${index}-${id}`;ids.set(id,next);el.id=next});
 for(const el of [clone,...clone.querySelectorAll('*')])for(const attr of [...el.attributes]){
  let value=attr.value;for(const [id,next]of ids){value=value.split(`url(#${id})`).join(`url(#${next})`);if((attr.name==='href'||attr.name==='xlink:href')&&value===`#${id}`)value=`#${next}`}if(value!==attr.value)el.setAttribute(attr.name,value);
 }
 const v=source.viewBox.baseVal;let x=v.x,y=v.y,w=v.width,h=v.height;
 if(!(w>0&&h>0))throw new Error('当前图形尚未就绪，请稍后再导出');
 // Include moved objects that extend beyond the current viewport.
 const bbox=source.getBBox();if(Number.isFinite(bbox.x)&&bbox.width>0&&bbox.height>0){const right=Math.max(x+w,bbox.x+bbox.width+12),bottom=Math.max(y+h,bbox.y+bbox.height+12);x=Math.min(x,bbox.x-12);y=Math.min(y,bbox.y-12);w=right-x;h=bottom-y}
 clone.setAttribute('viewBox',`${x} ${y} ${w} ${h}`);clone.style.removeProperty('display');clone.style.removeProperty('visibility');clone.style.removeProperty('opacity');clone.setAttribute('preserveAspectRatio','xMidYMid meet');
 return {svg:clone,width:w,height:h};
}
export async function composeDiagram(sources:SVGSVGElement[],title:string,scope:string){
 if(!sources.length)throw new Error('请先打开要导出的图形；列表页面不包含可导出图形');
 if(sources.some(s=>s.querySelector('.cabinet-dragging,.is-dragging,.network-dragging,.connection-draft,.layout-drop-indicator')))throw new Error('请先结束拖动并保存位置，再导出图形');
 await document.fonts.ready;
 const items=sources.map(copySVG),columns=Math.min(4,items.length),gap=24,padding=28;
 const cellWidth=items.length===1?items[0].width:Math.max(...items.map(i=>i.width));
 const rows:number[]=[];for(let i=0;i<items.length;i+=columns)rows.push(Math.max(...items.slice(i,i+columns).map(v=>v.height))+ (items.length>1?30:0));
 const width=Math.max(800,columns*cellWidth+(columns-1)*gap+padding*2),header=128;
 const height=header+rows.reduce((a,b)=>a+b,0)+(rows.length-1)*gap+padding;
 const root=element('svg',{xmlns:NS,viewBox:`0 0 ${width} ${height}`,width,height});
 root.style.fontFamily='"Microsoft YaHei", "Noto Sans CJK SC", "PingFang SC", sans-serif';root.append(element('rect',{width,height,fill:'#ffffff'}));
 function wrapped(value:string,y:number,size:number,maxLines:number){const limit=Math.max(12,Math.floor((width-padding*2)/size));const chars=Array.from(value);for(let line=0;line<maxLines&&chars.length;line++){let part=chars.splice(0,limit).join('');if(line===maxLines-1&&chars.length)part+='…';root.append(text(part,padding,y+line*(size+6),size,line===0&&size>16?'#18324f':'#526984'))}}
 wrapped(title,38,22,1);wrapped(scope,66,12,2);root.append(text(`导出时间：${new Date().toLocaleString('zh-CN',{hour12:false})}`,padding,112,11,'#61758e'));
 let rowY=header;
 for(let row=0;row<rows.length;row++){
  for(let col=0;col<columns;col++){
   const i=row*columns+col,item=items[i];if(!item)continue;const x=padding+col*(cellWidth+gap);
   if(items.length>1)root.append(text(sources[i].getAttribute('aria-label')||`图形 ${i+1}`,x,rowY+20,14));
   item.svg.setAttribute('x',String(x));item.svg.setAttribute('y',String(rowY+(items.length>1?30:0)));item.svg.setAttribute('width',String(item.width));item.svg.setAttribute('height',String(item.height));root.append(item.svg);
  }
  rowY+=rows[row]+gap;
 }
 return {xml:new XMLSerializer().serializeToString(root),width,height};
}
function save(blob:Blob,filename:string){const url=URL.createObjectURL(blob),anchor=document.createElement('a');anchor.href=url;anchor.download=filename;document.body.append(anchor);anchor.click();anchor.remove();setTimeout(()=>URL.revokeObjectURL(url),60000)}
function canvasBlob(canvas:HTMLCanvasElement,type:string,quality?:number){return new Promise<Blob>((resolve,reject)=>canvas.toBlob(blob=>blob?resolve(blob):reject(new Error('图形过大，无法生成图片；请改用SVG或缩小导出范围')),type,quality))}
export async function exportDiagram(sources:SVGSVGElement[],title:string,scope:string,format:'png'|'pdf'|'svg'){
 const {xml,width,height}=await composeDiagram(sources,title,scope),filename=diagramFilename(title);
 if(format==='svg'){save(new Blob([xml],{type:'image/svg+xml;charset=utf-8'}),filename+'.svg');return 'SVG已导出'}
 const size=rasterSize(width,height),canvas=document.createElement('canvas');canvas.width=size.width;canvas.height=size.height;
 const context=canvas.getContext('2d');if(!context)throw new Error('浏览器无法创建绘图画布，请改用SVG导出');
 const url=URL.createObjectURL(new Blob([xml],{type:'image/svg+xml;charset=utf-8'}));
 try{
  const image=new Image();await new Promise<void>((resolve,reject)=>{image.onload=()=>resolve();image.onerror=()=>reject(new Error('图形渲染失败，请改用SVG导出'));image.src=url});
  context.fillStyle='#fff';context.fillRect(0,0,size.width,size.height);context.drawImage(image,0,0,size.width,size.height);
  if(format==='png')save(await canvasBlob(canvas,'image/png'),filename+'.png');
  else{const jpeg=new Uint8Array(await (await canvasBlob(canvas,'image/jpeg',.96)).arrayBuffer());save(new Blob([imagePDF(jpeg,size.width,size.height)],{type:'application/pdf'}),filename+'.pdf')}
  return `${format.toUpperCase()}已导出${size.reduced?'；超大图已限制图片分辨率，放大查看建议导出SVG':''}`;
 }finally{URL.revokeObjectURL(url);canvas.width=0;canvas.height=0}
}
