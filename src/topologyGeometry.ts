export interface TopologyPoint { x: number; y: number }
export interface TopologyEdgeGeometry { path: string; label: TopologyPoint }

/** Use the same cubic curve for the drawing and its visible interaction label. */
export function topologyEdgeGeometry(a: TopologyPoint, b: TopologyPoint, index: number, nodeHeight=98): TopologyEdgeGeometry {
  const offset = (index % 5 - 2) * 7;
  const x1 = a.x + 92 + offset, x2 = b.x + 92 + offset;
  const horizontal = a.y === b.y;
  const down = a.y < b.y;
  const y1 = horizontal ? a.y : a.y + (down ? nodeHeight : 0);
  const y2 = horizontal ? b.y : b.y + (down ? 0 : nodeHeight);
  const controlY = horizontal ? a.y - 48 - offset : (y1 + y2) / 2 + offset;
  return {
    path: `M ${x1} ${y1} C ${x1} ${controlY} ${x2} ${controlY} ${x2} ${y2}`,
    // Cubic Bézier position at t=0.5: (P0 + 3P1 + 3P2 + P3) / 8.
    label: { x: (x1 + x2) / 2, y: (y1 + 6 * controlY + y2) / 8 },
  };
}
