import type { BrushState } from './types.ts';

export class BrushCache {
  private identity = '';
  private value: BrushState[] = [];
  update(epoch: string, mapId: string, brushes?: BrushState[]): BrushState[] {
    const identity = `${epoch}:${mapId}`;
    if (identity !== this.identity) { this.identity = identity; this.value = []; }
    if (brushes !== undefined) this.value = brushes;
    return this.value;
  }
}
