export interface RouteBox {id:string;x:number;y:number;width:number;height:number}
interface Point {x:number;y:number}
function simplify(points:Point[]){const result:Point[]=[];for(const p of points){const last=result.at(-1);if(last&&last.x===p.x&&last.y===p.y)continue;const a=result.at(-2);if(a&&last&&(a.x===last.x&&last.x===p.x||a.y===last.y&&last.y===p.y))result.pop();result.push(p)}return result}
function intersects(a:Point,b:Point,r:RouteBox,pad=7){return a.x===b.x?a.x>r.x-pad&&a.x<r.x+r.width+pad&&Math.max(a.y,b.y)>r.y-pad&&Math.min(a.y,b.y)<r.y+r.height+pad:a.y>r.y-pad&&a.y<r.y+r.height+pad&&Math.max(a.x,b.x)>r.x-pad&&Math.min(a.x,b.x)<r.x+r.width+pad}
const length=(p:Point[])=>p.slice(1).reduce((sum,v,i)=>sum+Math.abs(v.x-p[i].x)+Math.abs(v.y-p[i].y),0);
function ports(r:RouteBox,index:number){const offset=(index%5-2)*5;return [{point:{x:r.x+r.width/2+offset,y:r.y},stub:{x:r.x+r.width/2+offset,y:r.y-16}},{point:{x:r.x+r.width/2+offset,y:r.y+r.height},stub:{x:r.x+r.width/2+offset,y:r.y+r.height+16}},{point:{x:r.x,y:r.y+r.height/2+offset},stub:{x:r.x-16,y:r.y+r.height/2+offset}},{point:{x:r.x+r.width,y:r.y+r.height/2+offset},stub:{x:r.x+r.width+16,y:r.y+r.height/2+offset}}]}
/** Orthogonal corridors avoid device rectangles. Returns a collision count so
 * callers/tests can detect layouts that leave no safe corridor. */
export function routeConnection(a:RouteBox,b:RouteBox,boxes:RouteBox[],index=0,labels:{x:number;y:number;width:number;height:number}[]=[],labelWidth=88,labelObstacles:RouteBox[]=[]){
 const obstacles=boxes;let best:Point[]|undefined,bestCost=Infinity,bestHits=Infinity;
 const minX=Math.min(...boxes.map(n=>n.x))-28-(index%5)*10,maxX=Math.max(...boxes.map(n=>n.x+n.width))+28+(index%5)*10;
 const minY=Math.min(...boxes.map(n=>n.y))-24-(index%5)*8,maxY=Math.max(...boxes.map(n=>n.y+n.height))+24+(index%5)*8;
 const middleX=(a.x+b.x)/2,middleY=(a.y+b.y)/2;
 const xs=[minX,maxX,...[...new Set(boxes.flatMap(n=>[n.x-14,n.x+n.width+14]))].sort((x,y)=>Math.abs(x-middleX)-Math.abs(y-middleX)).slice(0,18)];
 const ys=[minY,maxY,...[...new Set(boxes.flatMap(n=>[n.y-14,n.y+n.height+14]))].sort((x,y)=>Math.abs(x-middleY)-Math.abs(y-middleY)).slice(0,18)];
 function consider(points:Point[]){const p=simplify(points),distance=length(p)+(p.length-2)*12;if(bestHits===0&&distance>=bestCost)return;let hits=0;
  for(let i=1;i<p.length;i++)for(const box of obstacles){if(i===1&&box.id===a.id||i===p.length-1&&box.id===b.id)continue;if(intersects(p[i-1],p[i],box))hits++}
  const cost=distance+hits*100000;if(cost<bestCost){best=p;bestCost=cost;bestHits=hits}
 }
 for(const start of ports(a,index))for(const end of ports(b,index)){
  const p=start.stub,q=end.stub;
  consider([start.point,p,{x:p.x,y:q.y},q,end.point]);consider([start.point,p,{x:q.x,y:p.y},q,end.point]);
  for(const x of xs)consider([start.point,p,{x,y:p.y},{x,y:q.y},q,end.point]);
  for(const y of ys)consider([start.point,p,{x:p.x,y},{x:q.x,y},q,end.point]);
 }
 const points=best||[{x:a.x+a.width,y:a.y+a.height/2},{x:b.x,y:b.y+b.height/2}];
 const segments=points.slice(1).map((p,i)=>({a:points[i],b:p,d:Math.abs(p.x-points[i].x)+Math.abs(p.y-points[i].y)})).sort((u,v)=>v.d-u.d);
 let label={x:(points[0].x+points.at(-1)!.x)/2,y:(points[0].y+points.at(-1)!.y)/2},labelVisible=false;
 for(const seg of segments){for(const t of [.5,.35,.65]){const candidate={x:seg.a.x+(seg.b.x-seg.a.x)*t,y:seg.a.y+(seg.b.y-seg.a.y)*t};const r={x:candidate.x-labelWidth/2,y:candidate.y-14,width:labelWidth,height:28};if(![...boxes,...labelObstacles,...labels].some(n=>r.x<n.x+n.width+3&&r.x+r.width+3>n.x&&r.y<n.y+n.height+3&&r.y+r.height+3>n.y)){label=candidate;labelVisible=true;break}}if(labelVisible)break}
 // Dense paths can have no segment long enough for a readable badge. Place
 // it above the nodes rather than leaving a clickable badge behind a device.
 if(!labelVisible){for(const y of [Math.max(16,minY-18),maxY-6]){for(let x=Math.max(labelWidth/2+8,minX+labelWidth/2);x<=maxX+labelWidth/2;x+=labelWidth+8){const candidate={x,y},r={x:x-labelWidth/2,y:candidate.y-14,width:labelWidth,height:28};if(![...boxes,...labelObstacles,...labels].some(n=>r.x<n.x+n.width+3&&r.x+r.width+3>n.x&&r.y<n.y+n.height+3&&r.y+r.height+3>n.y)){label=candidate;labelVisible=true;break}}if(labelVisible)break}}
 return {path:points.map((p,i)=>`${i?'L':'M'} ${p.x} ${p.y}`).join(' '),points,label,labelVisible,collisions:bestHits};
}
