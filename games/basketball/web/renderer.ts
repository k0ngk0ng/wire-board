/** Original-art Canvas presentation. This module has no game rules or ROM runtime. */
import type { LoadingProgress } from "./loading";
import { rasterizePostResultRows, unpackPostResultRasterRows, type PackedPostResultRows, type PostResultRasterAssets } from "./postresult-raster";

/** The logical raster plan is interpreted and validated by Go/Wasm. */
export interface PostResultProjectionInput { surface: Record<string, unknown>; plan: Record<string, unknown> }
export interface PostResultRenderSnapshot extends PostResultProjectionInput { oam: number[] }
export interface RenderObject {
  id: number; kind: number; character: number; pose: number;
  projected: { x: number; y: number };
  position: { x: number; depth: number; height: number };
  airborne: boolean; shadowSuppressed: boolean; action: number;
}
export interface HUDPanel { lines: number[][]; edgeGlyphs: number[]; headerGlyphs?: number[]; turnMarker: number | null }
export interface RenderHUD { clockLabel?: number[]; clock: { minutes: number; seconds: number }; scores: [number, number]; panels: [HUDPanel, HUDPanel] }
export interface RenderResult { court: number; digits: number[][]; winningTeam: number }
export interface HudFetchTiming { addressResetFetch: number; fineScrollResetFetch: number }
export interface BasketballRenderSnapshot {
  postResult?: PostResultRenderSnapshot;
  court: number; paletteID?: number;
  /** Original full-group palette clamp; absent is full color, explicit zero is black. */
  paletteLevel?: number;
  /** Optional original display-clock observation; absent uses the stable 5/9 split. */
  hudFetchTiming?: HudFetchTiming;
  objects: RenderObject[]; drawOrder: number[]; teams: [number, number];
  camera: { x: number; y: number }; reverseSprites: boolean;
  /** Display-stage selection. Never infer this from the current simulation pose. */
  ballCHR: number; ballSource: number; phase: number; hud: RenderHUD; result?: RenderResult;
}
export interface CourtColumnCommit { column: number; phase: number }
export interface CourtTilePatch { plane: number; row: number; column: number; tiles: number[] }
export type CourtDisplayEvent = CourtColumnCommit | { patch: CourtTilePatch };
interface Pixels { width: number; height: number; data: Uint8ClampedArray }
interface Fragment { tile: number; attributes: number; x: number; y: number }
interface Ring { columns: number[]; tiles: number[][]; attributes: number[][] }
interface ColumnPacket { column: number; slot: number; tiles: number[][]; attributes: number[][]; attribute_indices: number[] }
interface RasterManifest {
  atlas: { file: string; sha256: string; banks: number[] };
  paletteFade: { entries: Array<{rgb:number[];levels:number[][]}> };
  headTiles: number[][][]; profiles: Record<string, Fragment[][]>;
  shadow: Fragment; source: { rom_sha256: string };
}
interface ResultImages { background: Pixels; hud: Pixels; digits: Pixels; hudTiles: Pixels }
interface ResultFiles {background:{file:string;sha256:string};hud:{file:string;sha256:string};digits:{file:string;sha256:string};hudTiles:{file:string;sha256:string}}
interface ResultManifest {source:{rom_sha256:string};actorProfiles:Fragment[][];courts:Array<ResultFiles & {court:number;paletteVariants?:Array<ResultFiles & {paletteID:number}>}>}
interface CourtPalette {courtTiles: Pixels; courtTexture: Pixels; hudGlyphs: Pixels; hudTemplate: Pixels}
interface CourtArt { palettes: Record<number,CourtPalette>; initialRing: Ring; columns: {initialization:ColumnPacket[];gameplay:ColumnPacket[]} }
export interface BasketballAssets {
  ensureCourt?: (court: number) => Promise<void>;
  postResult?: PostResultRasterAssets;
  courts: CourtArt[];
  results: { actorProfiles: Fragment[][]; courts: Array<Record<number,ResultImages>> };
  raster: RasterManifest; spriteTiles: Pixels; courtTiles: Pixels; courtTexture: Pixels;
  hudGlyphs: Pixels; hudTemplate: Pixels; initialRing: Ring;
  columns: { initialization: ColumnPacket[]; gameplay: ColumnPacket[] };
}
const integer = (n: number, min: number, max: number, label: string) => {
  if (!Number.isInteger(n) || n < min || n > max) throw new Error(`Invalid ${label}: ${n}`);
};
async function json<T>(url: string): Promise<T> {
  const response = await fetch(url, {signal: AbortSignal.timeout(45000)}); if (!response.ok) throw new Error(`Asset unavailable: ${url}`);
  return response.json() as Promise<T>;
}
async function image(url: string, digest?: string): Promise<Pixels> {
  const response = await fetch(url, {signal: AbortSignal.timeout(45000)}); if (!response.ok) throw new Error(`Image unavailable: ${url}`);
  const bytes = await response.arrayBuffer();
  if (digest) {
    const actual = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(v => v.toString(16).padStart(2, '0')).join('');
    if (actual !== digest) throw new Error(`Image digest differs: ${url}`);
  }
  const bitmap = await createImageBitmap(new Blob([bytes]), { colorSpaceConversion: 'none' });
  const canvas = document.createElement('canvas'); canvas.width = bitmap.width; canvas.height = bitmap.height;
  const context = canvas.getContext('2d', { willReadFrequently: true }); if (!context) throw new Error('Canvas 2D unavailable');
  context.drawImage(bitmap, 0, 0); bitmap.close();
  return { width: canvas.width, height: canvas.height, data: context.getImageData(0, 0, canvas.width, canvas.height).data };
}
/** baseURL serves only exported image/JSON assets; no original executable is fetched. */
export async function loadBasketballAssets(baseURL: string, options: {postResult?: boolean;court?: number;onProgress?:(p:LoadingProgress)=>void} = {}): Promise<BasketballAssets> {
  const report=options.onProgress ?? (()=>{});
  report({percent:0,label:"加载球场清单"});
  const base = baseURL.replace(/\/$/, '');
  const [raster, players, objects, resultManifest, courtsManifest] = await Promise.all([
    json<RasterManifest>(`${base}/raster/manifest.json`),
    json<{ source: { rom_sha256: string }; pose_counts: number[] }>(`${base}/static-atlas/atlas.json`),
    json<{ source: { rom_sha256: string }; pose_counts: number[] }>(`${base}/static-objects/atlas.json`),
    json<ResultManifest>(`${base}/results/manifest.json`),
    json<{source:{rom_sha256:string};courts:Array<{court:number;manifest:string;sha256:string}>}>(`${base}/courts/manifest.json`),
  ]);
  if ([players,objects,resultManifest,courtsManifest].some(m=>m.source.rom_sha256!==raster.source.rom_sha256)) throw new Error('Mixed original-art versions');
  for (let k = 0; k < 15; k++) if (raster.profiles[k]?.length !== (k < 8 ? players.pose_counts[k] : objects.pose_counts[k - 8])) throw new Error('Fragment/sheet pose coverage differs');
  const spriteTiles=await image(`${base}/raster/${raster.atlas.file}`,raster.atlas.sha256);
  const courts: CourtArt[]=new Array(8);
  const resultCourts: Array<Record<number,ResultImages>>=new Array(8);
  const courtJobs=new Map<number,Promise<void>>();
  const ensureCourt=(index:number):Promise<void>=>{
    integer(index,0,7,'court');
    const cached=courtJobs.get(index);if(cached)return cached;
    const job=(async()=>{
      report({percent:0,label:'加载当前球场'});
      const record=courtsManifest.courts[index],c=resultManifest.courts[index];
      if(record?.court!==index||c?.court!==index)throw Error('Court asset identity differs');
      const manifest=await json<{court:number;palettes:number[];assets:Record<string,{sha256:string}>}>(`${base}/courts/${record.manifest}`);
      if(manifest.court!==index)throw new Error('Court manifest identity differs');
      const root=`${base}/courts/${index}`;
      const palettes=[{...c,paletteID:c.court+1},...(c.paletteVariants??[])];
      const total=2+manifest.palettes.length*4+palettes.length*4;let done=0;
      const track=async<T,>(task:Promise<T>):Promise<T>=>{const value=await task;done++;report({percent:10+85*done/total,label:`加载球场图片 ${done}/${total}`});return value};
      const [initialRing,columns,pairs,resultPairs]=await Promise.all([
        track(json<Ring>(`${root}/initial-ring.json`)),track(json<CourtArt['columns']>(`${root}/stream-columns.json`)),
        Promise.all(manifest.palettes.map(async id=>{
          const names=[`court-tiles-${id}.png`,`court-${id}.png`,`hud-glyphs-${id}.png`,`hud-template-${id}.png`];
          const [courtTiles,courtTexture,hudGlyphs,hudTemplate]=await Promise.all(names.map(name=>track(image(`${root}/${name}`,manifest.assets[name].sha256))));
          return [id,{courtTiles,courtTexture,hudGlyphs,hudTemplate}] as const;
        })),
        Promise.all(palettes.map(async p=>{
          const [background,hud,digits,hudTiles]=await Promise.all([p.background,p.hud,p.digits,p.hudTiles].map(a=>track(image(`${base}/results/${a.file}`,a.sha256))));
          return [p.paletteID,{background,hud,digits,hudTiles}] as const;
        }))
      ]);
      courts[index]={initialRing,columns,palettes:Object.fromEntries(pairs)};
      resultCourts[index]=Object.fromEntries(resultPairs);
    })().catch(error=>{courtJobs.delete(index);throw error});
    courtJobs.set(index,job);return job;
  };
  if(courtsManifest.courts.length!==8||resultManifest.courts.length!==8||resultManifest.actorProfiles.length!==120)throw new Error('Court asset coverage differs');
  const initialCourt=options.court ?? 1;
  await Promise.all(options.court===undefined ? Array.from({length:8},(_,i)=>ensureCourt(i)) : [ensureCourt(initialCourt)]);
  // Legacy research consumers can still inspect the original New York default.
  let postResult: PostResultRasterAssets | undefined;
  if(options.postResult){
    const manifest=await json<{format:number;source:{rom_sha256:string};indexAtlas:{file:string;pages:number[];tileSize:number;tilesPerPage:number};rgbPalette:number[][];assets:Array<{file:string;sha256:string}>}>(`${base}/postresult/manifest.json`);
    const atlas=manifest.indexAtlas,record=manifest.assets.find(a=>a.file===atlas.file);
    if(manifest.format!==1||manifest.source.rom_sha256!==raster.source.rom_sha256||atlas.tileSize!==8||atlas.tilesPerPage!==64||!record)throw new Error('Post-result artwork identity differs');
    const texture=await image(`${base}/postresult/${atlas.file}`,record.sha256);
    if(texture.width!==512||texture.height!==atlas.pages.length*8)throw new Error('Post-result index atlas dimensions differ');
    postResult={pages:atlas.pages,indices:texture.data,rgb:manifest.rgbPalette};
  }
  report({percent:100,label:"球场已就绪"});
  const ny=courts[initialCourt];return { ensureCourt, raster, spriteTiles, courts,...ny.palettes[initialCourt+1],initialRing:ny.initialRing,columns:ny.columns,results:{actorProfiles:resultManifest.actorProfiles,courts:resultCourts},postResult };
}
/** Retry a complete verified bundle, so one broken CDN image cannot leave a
 * partial mixture of remote and fallback scene resources in a running board. */
export async function loadBasketballAssetsWithFallback(baseURL: string, localBaseURL = baseURL, options: {postResult?: boolean} = {}): Promise<BasketballAssets> {
  const errors: unknown[] = [];
  const roots = [...new Set([baseURL, localBaseURL].map(value => value.replace(/\/$/, "")))];
  for (const root of roots) {
    try { return await loadBasketballAssets(root, options); }
    catch (error) { errors.push(error); }
  }
  throw new AggregateError(errors, "无法加载篮球图片素材");
}
interface Piece { x: number; y: number; tile: number; attr: number }
export interface RenderDiagnostics { spritePieces: number; capacityReached: boolean; overflowingScanlines: number }

export class BasketballRenderer {
  private context: CanvasRenderingContext2D;
  private ring: Ring;
  private court = 1;
  private paletteID = 2;
  private fadeColors = new Map<number,number[][]>();
  private get art(): CourtArt { return this.assets.courts[this.court]; }
  private get palette(): CourtPalette { return this.art.palettes[this.paletteID]; }
  private frame = new ImageData(256, 240);
  constructor(canvas: HTMLCanvasElement, private assets: BasketballAssets, private projectPostResult?: (input: PostResultProjectionInput) => PackedPostResultRows) {
    if(!assets.raster.paletteFade?.entries.length)throw new Error('Original fade palette data is missing');
    for(const entry of assets.raster.paletteFade.entries){
      if(entry.rgb.length!==3||entry.levels.length!==5||entry.levels.some(c=>c.length!==3))throw new Error('Invalid fade color table');
      [...entry.rgb,...entry.levels.flat()].forEach(n=>integer(n,0,255,'fade RGB channel'));
      if(entry.levels[4].some((v,i)=>v!==entry.rgb[i]))throw new Error('Full-color fade table is not identity');
      const key=(entry.rgb[0]<<16)|(entry.rgb[1]<<8)|entry.rgb[2];if(this.fadeColors.has(key))throw new Error('Duplicate fade input RGB');this.fadeColors.set(key,entry.levels);
    }
    canvas.width = 256; canvas.height = 240; canvas.style.imageRendering = 'pixelated';
    const context = canvas.getContext('2d', { willReadFrequently: true }); if (!context) throw new Error('Canvas 2D unavailable');
    this.context = context; context.imageSmoothingEnabled = false; this.ring = structuredClone(assets.initialRing);
  }
  hasCourt(court: number): boolean { return !!this.assets.courts[court] && !!this.assets.results.courts[court]; }
  ensureCourt(court: number): Promise<void> { return this.assets.ensureCourt?.(court) ?? Promise.resolve(); }
  resetCourt(court = 1): void { integer(court,0,7,'court'); this.court=court;this.paletteID=court+1;this.ring = structuredClone(this.art.initialRing); }
  /** Call at the display commit boundary, separately from simulation/camera decisions. */
  commitCourtColumns(commits: CourtColumnCommit[]): void { this.commitCourtEvents(commits); }
  commitCourtEvents(events: CourtDisplayEvent[]): void {
    for(const event of events) {
      if('patch' in event) { const p=event.patch;integer(p.plane,0,1,'patch plane');integer(p.row,p.plane===0?6:0,29,'patch row');integer(p.column,0,31,'patch column');if(!p.tiles.length||p.column+p.tiles.length>32)throw new Error('Court patch crosses row boundary');p.tiles.forEach(t=>integer(t,0,255,'patch tile')); }
      else {integer(event.column,0,63,'court column');integer(event.phase,0,127,'court phase');}
    }
    for(const event of events) {
      if('patch' in event){const p=event.patch;this.ring.tiles[p.plane].splice(p.row*32+p.column,p.tiles.length,...p.tiles);}
      else this.commitColumns([event]);
    }
  }
  private commitColumns(commits: CourtColumnCommit[]): void {
    for (const c of commits) { integer(c.column, 0, 63, 'court column'); integer(c.phase, 0, 127, 'court phase'); }
    for (const c of commits) {
      const packet = (c.phase === 0 ? this.art.columns.initialization : this.art.columns.gameplay)[c.column];
      this.ring.columns[packet.slot] = packet.column;
      for (let page = 0; page < 2; page++) {
        for (let row = 0; row < 30; row++) this.ring.tiles[page][row * 32 + packet.slot] = packet.tiles[page][row];
        packet.attribute_indices.forEach((index, i) => { this.ring.attributes[page][index] = packet.attributes[page][i]; });
      }
    }
  }
  private validate(s: BasketballRenderSnapshot): void {
    integer(s.paletteLevel??4,0,4,'palette level');
    integer(s.court,0,7,'court');if(s.court!==this.court)throw new Error('Court must be reset before committing its display history');
    if(s.hudFetchTiming){integer(s.hudFetchTiming.addressResetFetch,5,6,'HUD address reset fetch');integer(s.hudFetchTiming.fineScrollResetFetch,8,10,'HUD fine-scroll reset fetch');}
    const palette=s.paletteID??s.court+1;integer(palette,1,15,'court palette');if(!this.art.palettes[palette])throw new Error('Palette is unavailable for court');
    if(s.result != null){
      if(s.phase!==6)throw new Error('Result display requires phase6');
      if(!this.assets.results.courts[s.court][palette])throw new Error('Result palette is not restored for court');
      integer(s.result.court,0,7,'result court');if(s.result.court!==s.court)throw new Error('Result court differs');integer(s.result.winningTeam,0,1,'result winner');
      if(s.result.digits.length!==2||s.result.digits.some(side=>side.length!==3))throw new Error('Result requires two decimal score caches');
      s.result.digits.flat().forEach(d=>integer(d,0,9,'result digit'));
      if(s.camera.x!==0||s.camera.y!==296)throw new Error('Original result camera must be 0/296');
    }else{
      integer(s.camera.x, 0, 255, 'display camera X'); integer(s.camera.y, 48, 0x129, 'display camera Y');
      if ((s.camera.y & 255) >= 240) throw new Error('Camera lies in the original 16-pixel page gap');
    }
    integer(s.phase, 0, 8, 'scene phase');
    // BA40 changes CHR before the old hidden submission is committed by video DMA.
    const blankResultEntry = s.phase===6 && s.result==null && s.drawOrder.length===0 && s.objects.every(o=>(o.pose&127)===127);
    if(s.result != null)integer(s.ballCHR,13,13,'result sprite CHR');
    else if(!(blankResultEntry && s.ballCHR===13))integer(s.ballCHR, 32, 35, 'display ball CHR');
    s.teams.forEach(n => integer(n, 0, 7, 'team')); integer(s.ballSource, 0, 255, 'ball source');
    if ((s.objects[6]?.pose & 127) !== 127 && s.objects[6]?.action >= 12 && s.ballSource > 3) throw new Error('Special ball display source is not reconstructed');
    if (s.objects.length !== 13 || s.drawOrder.length > 12) throw new Error('Invalid original draw input size');
    const ids = new Set<number>();
    for (const o of s.objects) {
      integer(o.id, 0, 12, 'object ID'); if (ids.has(o.id)) throw new Error('Duplicate object'); ids.add(o.id);
      integer(o.kind, 0, 17, 'object kind'); if(o.kind>14&&!(s.result != null&&o.kind===17))throw new Error('Object kind is not restored in this scene'); integer(o.character, 0, 4, 'character'); integer(o.pose, 0, 255, 'pose'); integer(o.action, 0, 127, 'action');
      integer(o.projected.x, -32768, 32767, 'projection X'); integer(o.projected.y, -32768, 32767, 'projection Y');
      integer(o.position.height, -8388608, 8388607, 'height');
      if ((o.pose & 127) !== 127 && !(o.kind===17?this.assets.results.actorProfiles:this.assets.raster.profiles[o.kind])[o.pose & 127]) throw new Error('Pose missing from original sprite kind');
    }
    const order = new Set<number>(); for (const id of s.drawOrder) { integer(id, 0, 12, 'draw object'); if (order.has(id)) throw new Error('Duplicate draw-order object'); order.add(id); }
    if(s.result != null)return;
    if (s.hud.clockLabel) { if (s.hud.clockLabel.length !== 4) throw new Error('HUD clock label must contain four glyphs'); s.hud.clockLabel.forEach(n => integer(n, 0, 255, 'clock label glyph')); }
    integer(s.hud.clock.minutes, 0, 9, 'display minutes'); integer(s.hud.clock.seconds, 0, 59, 'display seconds'); s.hud.scores.forEach(n => integer(n, 0, 999, 'display score'));
    for (const p of s.hud.panels) {
      if (p.lines.length !== 3 || p.edgeGlyphs.length !== 3 || p.lines.some(line => line.length !== 9)) throw new Error('HUD panel glyph layout differs');
      [...p.lines.flat(), ...p.edgeGlyphs].forEach(n => integer(n, 0, 255, 'HUD glyph'));
      if (p.headerGlyphs) { if (p.headerGlyphs.length !== 9) throw new Error('HUD header must contain nine glyphs'); p.headerGlyphs.forEach(n => integer(n, 0, 255, 'name accent glyph')); }
      if (p.turnMarker !== null) integer(p.turnMarker, 0, 8, 'HUD turn marker');
    }
  }
  private copy(source: Pixels, sx: number, sy: number, x: number, y: number): void {
    const a = (sy * source.width + sx) * 4; if (!source.data[a + 3]) return;
    const b = (y * 256 + x) * 4;
    this.frame.data[b] = source.data[a]; this.frame.data[b + 1] = source.data[a + 1]; this.frame.data[b + 2] = source.data[a + 2]; this.frame.data[b + 3] = 255;
  }
  private background(s: BasketballRenderSnapshot): void {
    this.frame.data.fill(0); for (let i = 3; i < this.frame.data.length; i += 4) this.frame.data[i] = 255;
    const startY = (s.camera.y >> 8) * 240 + (s.camera.y & 255);
    for (let y = 0; y < 193; y++) {
      const wy = startY + y, page = Math.floor(wy / 240), py = wy % 240, ty = py >> 3;
      for (let x = 8; x < 256; x++) {
        const px = (s.camera.x + x) & 255, tx = px >> 3;
        const tile = this.ring.tiles[page][ty * 32 + tx];
        const attr = this.ring.attributes[page][(ty >> 2) * 8 + (tx >> 2)];
        const group = (attr >> (((ty & 3) >> 1) * 4 + ((tx & 3) >> 1) * 2)) & 3;
        this.copy(this.palette.courtTiles, (tile & 15) * 8 + (px & 7), (group * 16 + (tile >> 4)) * 8 + (py & 7), x, y);
      }
    }
  }
  private hudCell(column: number, row: number, tile: number): void {
    for (let y = 0; y < 8; y++) for (let x = 0; x < 8; x++) {
      const dy = 191 + row * 8 + y; if (dy < 193 || dy >= 240) continue;
      this.copy(this.palette.hudGlyphs, (tile & 15) * 8 + x, (tile >> 4) * 8 + y, column * 8 + x, dy);
    }
  }
  private hud(h: RenderHUD): void {
    for (let y = 193; y < 240; y++) for (let x = 0; x < 256; x++) this.copy(this.palette.hudTemplate, x, y - 191, x, y);
    h.panels.forEach((p, side) => {
      const start = side ? 21 : 2, edge = side ? 30 : 1;
      for (let i = 0; i < 9; i++) this.hudCell(start + i, 0, p.headerGlyphs ? p.headerGlyphs[i] : (i === p.turnMarker ? 0xfe : 0xe5));
      p.lines.forEach((line, row) => { this.hudCell(edge, row + 1, p.edgeGlyphs[row]); line.forEach((glyph, i) => this.hudCell(start + i, row + 1, glyph)); });
    });
    const clock = [...(h.clockLabel ?? [0xf1, 0xf3, 0, 0]), h.clock.minutes + 1, 0xf9, Math.floor(h.clock.seconds / 10) + 1, h.clock.seconds % 10 + 1];
    clock.forEach((tile, i) => this.hudCell(12 + i, 1, tile));
    for (let i = 0; i < 8; i++) this.hudCell(12 + i, 2, 0xeb);
    const digits = (n: number) => [Math.floor(n / 100) + 1, Math.floor(n / 10) % 10 + 1, n % 10 + 1];
    [...digits(h.scores[0]), 0xfb, 0xfc, ...digits(h.scores[1])].forEach((tile, i) => this.hudCell(12 + i, 3, tile));
  }
  /** F331/F33B select the font before F348 resets the background address.
   * Five or six old-address tile rows are prefetched at the start of scanline 193.
   * Their colors use the HUD font but retain the court's tile and palette IDs.
   * F350 then clears fine X; the two fetched rows still drain in order.
   * This is a bounded display projection, not a CPU/PPU instruction runtime.
   */
  private hudBoundary(s: BasketballRenderSnapshot): void {
    const timing=s.hudFetchTiming??{addressResetFetch:5,fineScrollResetFetch:9};
    const lineOffset=193*256*4;
    const hudLine=this.frame.data.slice(lineOffset,lineOffset+256*4);
    const worldY=(s.camera.y>>8)*240+(s.camera.y&255)+193;
    const plane=Math.floor(worldY/240), row=Math.floor((worldY%240)/8), fineY=worldY%8;
    const fetched: Uint8ClampedArray[]=[];
    for(let fetch=0;fetch<34;fetch++) {
      if(fetch>=2) {
        const fineX=fetch<timing.fineScrollResetFetch?s.camera.x&7:0;
        for(let x=0;x<8;x++) {
          const screenX=(fetch-2)*8+x;if(screenX<8)continue;
          const offset=x+fineX, source=fetched[fetch-2+(offset>>3)], start=(offset&7)*4;
          this.frame.data.set(source.subarray(start,start+4),lineOffset+screenX*4);
        }
      }
      if(fetch<timing.addressResetFetch) {
        const column=((s.camera.x>>3)+fetch)&31;
        const tile=this.ring.tiles[plane][row*32+column];
        const attr=this.ring.attributes[plane][(row>>2)*8+(column>>2)];
        const group=(attr>>(((row&3)>>1)*4+((column&3)>>1)*2))&3;
        const start=(((group*16+(tile>>4))*8+fineY)*this.palette.hudGlyphs.width+(tile&15)*8)*4;
        fetched.push(this.palette.hudGlyphs.data.slice(start,start+32));
      } else fetched.push(hudLine.slice((fetch-timing.addressResetFetch)*32,(fetch-timing.addressResetFetch+1)*32));
    }
    for(let x=0;x<8;x++)this.frame.data.set([0,0,0,255],lineOffset+x*4);
  }
  private resultBackground(s: BasketballRenderSnapshot): void {
    const art=this.assets.results.courts[s.result!.court][this.paletteID];
    this.frame.data.fill(0);for(let i=3;i<this.frame.data.length;i+=4)this.frame.data[i]=255;
    for(let y=0;y<194;y++)for(let x=y<192?8:0;x<256;x++)this.copy(art.background,x,y+40,x,y);
    for(let y=194;y<239;y++)for(let x=0;x<256;x++)this.copy(art.hud,x,y-191,x,y);
    // Preset99 replaces six rows only. The final visible scanline retains
    // the preceding court-ring row, rendered through the phase6 font pages.
    for(let x=0;x<256;x++){
      const tx=x>>3,tile=this.ring.tiles[0][6*32+tx],attr=this.ring.attributes[0][8+(tx>>2)];
      const group=(attr>>(4+((tx&3)>>1)*2))&3;
      this.copy(art.hudTiles,(tile&15)*8+(x&7),(group*16+(tile>>4))*8,x,239);
    }
    s.result!.digits.forEach((side,team)=>side.forEach((digit,index)=>{
      for(let y=0;y<24;y++)for(let x=0;x<16;x++)this.copy(art.digits,digit*16+x,y,64+team*80+index*16+x,199+y);
    }));
  }
  private pieces(s: BasketballRenderSnapshot): { pieces: Piece[]; full: boolean } {
    const pieces: Piece[] = []; const objects = new Map(s.objects.map(o => [o.id, o])); let full = false;
    const emit = (o: RenderObject, piece: Fragment, shadow = false): boolean => {
      let tile = piece.tile, attr = piece.attributes, dx = piece.x;
      if (tile < 4) tile = this.assets.raster.headTiles[tile][o.id >= 2 && o.id < 4 ? 1 : 0][o.character];
      else if ((tile & 0xc0) === 0x40 && o.id >= 2 && o.id < 4) tile = (tile + 0x40) & 255;
      if (o.id === 6 && o.action >= 12 && s.ballSource < 2) attr &= 0xfd;
      if ((attr & 3) < 2 && o.id >= 2 && o.id < 4) attr += 2;
      // Shadow attributes start after the ordinary piece palette adjustment.
      if (shadow) attr = piece.attributes;
      if (!(o.pose & 128)) { dx = -dx - 8; attr ^= 64; }
      const x = o.projected.x + dx, y = o.projected.y + 24 + piece.y + (shadow ? Math.floor(o.position.height / 256) : 0);
      if (y - 1 < 0 || y - 1 >= 185 || x < 0 || x >= 256) return false;
      if (attr & 32) throw new Error('Behind-background sprite priority is not reconstructed');
      pieces.push({ x, y, tile, attr }); if (pieces.length === 64) { full = true; return true; } return false;
    };
    outer: for (const id of s.drawOrder) {
      const o = objects.get(id)!; if ((o.pose & 127) === 127) continue;
      for (const piece of (o.kind===17?this.assets.results.actorProfiles:this.assets.raster.profiles[o.kind])[o.pose & 127]) if (emit(o, piece)) break outer;
      if (o.airborne && !o.shadowSuppressed && !(id === 6 && (o.action === 1 || o.action === 2))) if (emit(o, this.assets.raster.shadow, true)) break;
    }
    if (s.reverseSprites) pieces.reverse();
    return { pieces, full };
  }
  /** The exported table proves the NES-index clamp is well-defined for every
   * duplicate RGB class. Apply it once to the composed background/HUD/sprites.
   * This changes palette colors exactly; alpha and page/controller UI stay intact.
   */
  private applyPaletteLevel(level:number): void {
    if(level===4)return;
    const pixels=this.frame.data;
    for(let i=0;i<pixels.length;i+=4){
      const key=(pixels[i]<<16)|(pixels[i+1]<<8)|pixels[i+2],colors=this.fadeColors.get(key);
      if(!colors)throw new Error('Rendered pixel is outside the original NES palette');
      [pixels[i],pixels[i+1],pixels[i+2]]=colors[level];
    }
  }
  render(snapshot: BasketballRenderSnapshot): RenderDiagnostics {
    if(snapshot.postResult){
      if(!this.assets.postResult||!this.projectPostResult)throw new Error('Post-result presentation is unavailable');
      const {surface,plan,oam}=snapshot.postResult;
      const rows=unpackPostResultRasterRows(this.projectPostResult({surface,plan}),oam);
      const pixels=rasterizePostResultRows(this.assets.postResult,rows);
      this.context.putImageData(new ImageData(pixels,256,240),0,0);
      const spritePieces=oam.filter((value,index)=>index%4===0&&value<239).length;
      let overflowingScanlines=0;
      for(const row of rows.rows){
        if(!(row.mask&16))continue;
        let count=0;for(let n=0;n<64;n++)if(row.y>oam[n*4]&&row.y<=oam[n*4]+8)count++;
        if(count>8)overflowingScanlines++;
      }
      return {spritePieces,capacityReached:spritePieces===64,overflowingScanlines};
    }
    this.validate(snapshot);this.paletteID=snapshot.paletteID??snapshot.court+1;const { pieces, full } = this.pieces(snapshot);
    if(snapshot.result != null)this.resultBackground(snapshot);else{this.background(snapshot);this.hud(snapshot.hud);this.hudBoundary(snapshot)} let overflowingScanlines = 0;
    for (let y = 0; y < 240; y++) {
      const candidates = pieces.filter(p => p.y <= y && y < p.y + 8); if (candidates.length > 8) overflowingScanlines++;
      const visible = candidates.slice(0, 8);
      for (let i = visible.length - 1; i >= 0; i--) {
        const p = visible[i], virtualPage = p.tile >> 6;
        const bank = virtualPage === 0 ? 0 : virtualPage === 1 ? snapshot.teams[0] + 1 : virtualPage === 2 ? snapshot.teams[1] + 1 : snapshot.ballCHR;
        const side = (p.attr & 3) >> 1, team = snapshot.teams[side], group = p.attr & 1;
        const bankIndex = this.assets.raster.atlas.banks.indexOf(bank); if (bankIndex < 0) throw new Error('Missing sprite CHR page');
        const sy = (bankIndex * 16 + team * 2 + group) * 8 + ((p.attr & 128) ? 7 - (y - p.y) : y - p.y);
        for (let dx = 0; dx < 8; dx++) {
          const x = p.x + dx; if (x < 8 || x >= 256) continue;
          const sx = (p.tile & 63) * 8 + ((p.attr & 64) ? 7 - dx : dx); this.copy(this.assets.spriteTiles, sx, sy, x, y);
        }
      }
    }
    this.applyPaletteLevel(snapshot.paletteLevel??4);
    this.context.putImageData(this.frame, 0, 0);
    return { spritePieces: pieces.length, capacityReached: full, overflowingScanlines };
  }
}
