import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
const compiled = ts.transpileModule(readFileSync(new URL('../src/topologyGeometry.ts', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
}).outputText;
const { topologyEdgeGeometry } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'));

test('vertical links have a finite interaction anchor on the path despite zero-width geometry', () => {
  const edge = topologyEdgeGeometry({ x: 115, y: 466 }, { x: 115, y: 644 }, 0);
  assert.equal(edge.path, 'M 193 564 C 193 590 193 590 193 644');
  assert.deepEqual(edge.label, { x: 193, y: 593.5 });
  assert.ok(edge.label.y > 564 && edge.label.y < 644);
});
test('horizontal links place the label at the curve apex, outside node cards', () => {
  const edge = topologyEdgeGeometry({ x: 115, y: 466 }, { x: 331, y: 466 }, 2);
  assert.deepEqual(edge.label, { x: 315, y: 430 });
  assert.ok(edge.label.y + 14 < 466);
});
test('reverse, diagonal and parallel connections keep labels on the cubic midpoint', () => {
  for (const [a, b] of [[{x:115,y:644},{x:115,y:466}], [{x:115,y:466},{x:547,y:644}], [{x:547,y:644},{x:115,y:466}]]) {
    for (let i = 0; i < 5; i++) {
      const edge = topologyEdgeGeometry(a, b, i);
      const numbers = edge.path.match(/-?\d+(?:\.\d+)?/g).map(Number);
      assert.equal(edge.label.x, (numbers[0] + 3*numbers[2] + 3*numbers[4] + numbers[6])/8);
      assert.equal(edge.label.y, (numbers[1] + 3*numbers[3] + 3*numbers[5] + numbers[7])/8);
      assert.ok(Number.isFinite(edge.label.x) && Number.isFinite(edge.label.y));
    }
  }
});
