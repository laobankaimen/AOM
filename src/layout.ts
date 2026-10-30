import type { Infra, Module, Rack } from './model';
export interface LayoutTile { id: string; name: string; kind: string; x: number; width: number; rack?: Rack; infra?: Infra; integrated: Infra[] }
export function rowLayout(module: Module, racks: Rack[], infrastructure: Infra[], side: string): LayoutTile[] {
  const list = racks.filter(r => r.moduleId === module.id && r.side === side).sort((a,b) => a.index-b.index);
  const infra = infrastructure.filter(i => i.moduleId === module.id && i.side === side).sort((a,b)=>(a.order||0)-(b.order||0));
  const tiles: LayoutTile[] = [];let x = 0;
  const addInfra = (after: number) => infra.filter(i => i.placement === 'inline' && i.afterRack === after).forEach(i => {
    tiles.push({ id:i.id, name:i.name, kind:i.kind, x, width:76, infra:i, integrated:[] });x+=88;
  });
  addInfra(0);
  for(const rack of list){tiles.push({id:rack.id,name:rack.name,kind:'rack',x,width:108,rack,integrated:infra.filter(i=>i.placement==='integrated'&&i.afterRack===rack.index)});x+=120;addInfra(rack.index)}
  return tiles;
}
export function layoutWidth(tiles: LayoutTile[]) { const last=tiles.at(-1);return last?last.x+last.width:0; }
