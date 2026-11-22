import { motionColliders, type MotionCollider } from './motion-colliders.ts';
import { gunzipSync } from 'fflate';
import { GLTFLoader, type GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';
import * as T from 'three';
import { assetURL, selectModelKeys, selectSoundKeys, validateManifest, validateResource, validateRulesIdentity } from './asset-contract.ts';
import { assertModelTextures, decodeEmbeddedTexture } from './embedded-textures.ts';
import type { AssetManifest, MapRules, MotionFunction, Player, RenderMap, Resource, WeaponInfo } from './types.ts';

type MotionAPI = { version: number; unit: number; fps: number; sourceSHA256: string; init: (map: string) => boolean; move: (request: string) => string; canSmooth: (request: string) => boolean; weapons?: () => string };
declare global { interface Window { cs16Motion?: MotionAPI } }
type Progress = (percent: number, label: string, bytes?: { loaded: number; total: number }) => void;
export type LoadedAssets = { manifest: AssetManifest; map: RenderMap; rules: MapRules; images: Map<string, ImageBitmap | HTMLImageElement>; models: Map<string, GLTF>; sounds: Map<string, string>; weapons: WeaponInfo[]; motion: MotionFunction; canSmooth: import('./presentation.ts').SmoothTest; dynamics: (brushes: unknown[], players: Player[]) => void; audit: { models: number; materialCount: number; texturedMaterialCount: number; missingTextures: string[] } };
const manifests = new Map<string, Promise<AssetManifest>>();
const sizes = new WeakMap<AssetManifest, number>();
const cachedBytes = new Map<string, Uint8Array>();
const decodedImages = new Map<string, ImageBitmap | HTMLImageElement>();
const decodedModels = new Map<string, GLTF>();
let runtime: { identity: string; promise: Promise<void> } | undefined;

async function sha(bytes: Uint8Array) {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', new Uint8Array(bytes)));
  return [...digest].map(value => value.toString(16).padStart(2, '0')).join('');
}

export function loadManifest(base: string) {
  const url = assetURL(base, 'manifest.json');
  let promise = manifests.get(url);
  if (!promise) {
    promise = fetch(url).then(async response => {
      if (!response.ok) throw Error(`素材清单读取失败（${response.status}）`);
      const bytes = new Uint8Array(await response.arrayBuffer());
      const manifest = validateManifest(JSON.parse(new TextDecoder().decode(bytes)) as AssetManifest);
      sizes.set(manifest, bytes.byteLength);
      return manifest;
    }).catch(error => { manifests.delete(url); throw error; });
    manifests.set(url, promise);
  }
  return promise;
}

export async function readResource(base: string, resource: Resource, signal: AbortSignal, report: (bytes: number) => void = () => {}) {
  validateResource(resource);
  const identity = `${resource.sha256}:${resource.bytes}`;
  let bytes = cachedBytes.get(identity);
  if (!bytes) {
    const response = await fetch(assetURL(base, resource.file), { signal });
    if (!response.ok) throw Error(`素材读取失败（${response.status}）：${resource.file}`);
    const reader = response.body?.getReader();
    const chunks: Uint8Array[] = [];
    let count = 0;
    if (reader) {
      try {
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          chunks.push(value);
          count += value.byteLength;
          if (count > resource.bytes) throw Error(`素材大小超出清单：${resource.file}`);
          report(count);
        }
      } finally { reader.releaseLock(); }
    } else {
      const value = new Uint8Array(await response.arrayBuffer());
      chunks.push(value);
      count = value.byteLength;
      report(count);
    }
    if (count !== resource.bytes) throw Error(`素材大小不符：${resource.file}`);
    bytes = new Uint8Array(count);
    let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    if (await sha(bytes) !== resource.sha256) throw Error(`素材校验失败：${resource.file}`);
    cachedBytes.set(identity, bytes);
  } else report(bytes.byteLength);
  if (resource.encoding === 'gzip') {
    const decoded = gunzipSync(bytes);
    if (decoded.byteLength !== resource.decodedBytes || await sha(decoded) !== resource.decodedSHA256) throw Error(`素材解压校验失败：${resource.file}`);
    return decoded;
  }
  if (resource.encoding) throw Error(`未知素材编码：${resource.encoding}`);
  return bytes;
}

async function decodeImage(base: string, resource: Resource, bytes: Uint8Array) {
  let image = decodedImages.get(resource.sha256);
  if (image) return image;
  if (typeof createImageBitmap === 'function') image = await createImageBitmap(new Blob([new Uint8Array(bytes)], { type: resource.mime ?? 'image/png' }), { imageOrientation: 'none', premultiplyAlpha: 'none' });
  else {
    const value = new Image();
    value.src = assetURL(base, resource.file);
    await value.decode();
    image = value;
  }
  decodedImages.set(resource.sha256, image);
  return image;
}

async function initializeRuntime(base: string, manifest: AssetManifest, data: Map<string, Uint8Array>) {
  const selected = manifest.runtime;
  if (!selected) throw Error('缺少运动预测运行时');
  const identity = `${selected.sourceSHA256}:${selected.buildSHA256}`;
  if (runtime && runtime.identity !== identity) throw Error('运动规则已更新，请刷新页面');
  if (!runtime) runtime = { identity, promise: (async () => {
    const globals = window as Window & { Go?: new () => { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> } };
    {
      const bytes = data.get('runtime:script')!;
      const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', new Uint8Array(bytes)));
      await new Promise<void>((resolve, reject) => {
        const script = document.createElement('script');
        script.src = assetURL(base, selected.script.file);
        script.integrity = `sha256-${btoa(String.fromCharCode(...digest))}`;
        script.crossOrigin = 'anonymous';
        script.onload = () => { script.remove(); resolve(); };
        script.onerror = () => { script.remove(); reject(Error('运动运行时脚本读取失败')); };
        document.head.append(script);
      });
    }
    if (!globals.Go) throw Error('运动运行时不可用');
    const go = new globals.Go();
    const compiled = await WebAssembly.instantiate(new Uint8Array(data.get('runtime:wasm')!), go.importObject);
    void go.run(compiled.instance).catch(() => {});
    await new Promise<void>((resolve, reject) => {
      const deadline = performance.now() + 10000;
      const poll = () => {
        if (window.cs16Motion) resolve();
        else if (performance.now() > deadline) reject(Error('运动规则初始化超时'));
        else setTimeout(poll, 10);
      };
      poll();
    });
    if (window.cs16Motion?.version !== 1 || window.cs16Motion.unit !== 256 || window.cs16Motion.fps !== 100 || window.cs16Motion.sourceSHA256 !== selected.sourceSHA256 || typeof window.cs16Motion.canSmooth !== 'function') throw Error('运动规则版本不兼容');
  })().catch(error => { runtime = undefined; throw error; }) };
  await runtime.promise;
}

export async function loadBattleAssets(base: string, mapID: string, characters: string[], signal: AbortSignal, report: Progress, rulesSHA256: string | undefined): Promise<LoadedAssets> {
  report(0, '读取战场清单');
  const manifest = await loadManifest(base);
  const selected = manifest.maps[mapID];
  if (!selected) throw Error('未找到所选原版地图');
  validateRulesIdentity(selected.rules, rulesSHA256);
  const rawMap = await readResource(base, selected.render, signal);
  const map = JSON.parse(new TextDecoder().decode(rawMap)) as RenderMap;
  if (map.id !== mapID || !map.meshes?.length || !map.textures || !map.lightmap) throw Error('原版地图绘制数据无效');
  const files = new Map<string, Resource>();
  files.set('map', selected.render);
  files.set('rules', selected.rules!);
  for (const [name, resource] of Object.entries(map.textures)) files.set(`texture:${name}`, resource);
  files.set('lightmap', map.lightmap);
  for (const [index, resource] of (selected.sky ?? []).entries()) files.set(`sky:${index}`, resource);
  const required = [...new Set([...selectModelKeys(characters, selected.mode), ...map.decorations?.map(item => item.model) ?? []])];
  for (const key of required) {
    const resource = manifest.models[key];
    if (!resource) {
      if (key === 'hostage' && selected.mode !== 'hostage' || key.startsWith('p-') || key.startsWith('w-')) continue;
      throw Error(`缺少原版模型：${key}`);
    }
    files.set(`model:${key}`, resource);
  }
  const soundIDs = selectSoundKeys(manifest.sounds, selected.mode);
  if (manifest.hud) files.set('hud', manifest.hud.atlas);
  for (const key of soundIDs) files.set(`sound:${key}`, manifest.sounds[key]);
  if (!manifest.runtime) throw Error('缺少运动预测运行时');
  files.set('runtime:script', manifest.runtime.script);
  files.set('runtime:wasm', manifest.runtime.wasm);
  const unique = new Map<string, { resource: Resource; keys: string[] }>();
  for (const [key, resource] of files) {
    const identity = `${resource.sha256}:${resource.bytes}`;
    const prior = unique.get(identity);
    if (prior) prior.keys.push(key);
    else unique.set(identity, { resource, keys: [key] });
  }
  const metadata = sizes.get(manifest) ?? 0;
  const total = metadata + [...unique.values()].reduce((count, file) => count + file.resource.bytes, 0);
  const counts = new Map<string, number>();
  const data = new Map<string, Uint8Array>();
  const entries = [...unique.entries()];
  let cursor = 0;
  await Promise.all(Array.from({ length: Math.min(6, entries.length) }, async () => {
    while (cursor < entries.length) {
      const [identity, file] = entries[cursor++];
      const bytes = await readResource(base, file.resource, signal, count => {
        counts.set(identity, count);
        const loaded = metadata + [...counts.values()].reduce((sum, count) => sum + count, 0);
        report(Math.trunc(loaded * 90 / Math.max(1, total)), '加载原版战场与装备', { loaded, total });
      });
      for (const key of file.keys) data.set(key, bytes);
    }
  }));
  signal.throwIfAborted();
  const images = new Map<string, ImageBitmap | HTMLImageElement>();
  for (const [key, resource] of files) if (key.startsWith('texture:') || key === 'lightmap' || key === 'hud' || key.startsWith('sky:')) images.set(key, await decodeImage(base, resource, data.get(key)!));
  report(93, '解码角色与武器', { loaded: total, total });
  const models = new Map<string, GLTF>();
  const audit = { models: 0, materialCount: 0, texturedMaterialCount: 0, missingTextures: [] as string[] };
  const loader = new GLTFLoader();
  loader.register(parser => ({ name: 'CS16_embedded_textures', loadTexture: index => decodeEmbeddedTexture(parser, index, blob => createImageBitmap(blob, { imageOrientation: 'none', premultiplyAlpha: 'none', colorSpaceConversion: 'none' }), (image, sampler, name) => {
    const texture = new T.Texture(image);
    texture.name = name;
    texture.flipY = false;
    texture.magFilter = sampler.magFilter === 9728 ? T.NearestFilter : T.LinearFilter;
    const filters: Record<number, T.MinificationTextureFilter> = { 9728: T.NearestFilter, 9729: T.LinearFilter, 9984: T.NearestMipmapNearestFilter, 9985: T.LinearMipmapNearestFilter, 9986: T.NearestMipmapLinearFilter, 9987: T.LinearMipmapLinearFilter };
    const wrapping: Record<number, T.Wrapping> = { 33071: T.ClampToEdgeWrapping, 33648: T.MirroredRepeatWrapping, 10497: T.RepeatWrapping };
    texture.minFilter = filters[sampler.minFilter] ?? T.LinearMipmapLinearFilter;
    texture.wrapS = wrapping[sampler.wrapS] ?? T.RepeatWrapping;
    texture.wrapT = wrapping[sampler.wrapT] ?? T.RepeatWrapping;
    texture.generateMipmaps = texture.minFilter !== T.NearestFilter && texture.minFilter !== T.LinearFilter;
    texture.needsUpdate = true;
    parser.associations.set(texture, { textures: index });
    return texture;
  }) }));
  for (const [key, resource] of files) if (key.startsWith('model:')) {
    signal.throwIfAborted();
    let model = decodedModels.get(resource.sha256);
    if (!model) {
      model = await loader.parseAsync(new Uint8Array(data.get(key)!).buffer, '');
      assertModelTextures(model.scene);
      decodedModels.set(resource.sha256, model);
    }
    const textures = assertModelTextures(model.scene);
    audit.models++;
    audit.materialCount += textures.materials;
    audit.texturedMaterialCount += textures.texturedMaterials;
    models.set(key.slice(6), model);
  }
  const rules = JSON.parse(new TextDecoder().decode(data.get('rules'))) as MapRules;
  if (rules.id !== mapID) throw Error('地图与碰撞规则身份不符');
  report(97, '初始化共同运动规则', { loaded: total, total });
  await initializeRuntime(base, manifest, data);
  signal.throwIfAborted();
  const api = window.cs16Motion!;
  if (!api.init(JSON.stringify(rules))) throw Error('运动碰撞规则初始化失败');
  let brushes: unknown[] = [];
  let players: MotionCollider[] = [];
  const motion: MotionFunction = (player, input, frames) => {
    const value = JSON.parse(api.move(JSON.stringify({ player, input, frames, brushes, players }))) as Player & { error?: string };
    if (value.error) throw Error(value.error);
    return value;
  };
  const sounds = new Map(soundIDs.map(key => [key, assetURL(base, manifest.sounds[key].file)]));
  signal.throwIfAborted();
  return { manifest, map, rules, images, models, sounds, weapons: api.weapons ? JSON.parse(api.weapons()) as WeaponInfo[] : [], motion, audit, canSmooth: (from, to, crouch) => api.canSmooth(JSON.stringify({ from: { x: from.x, y: from.y, z: from.z }, to: { x: to.x, y: to.y, z: to.z }, crouch, brushes })), dynamics: (nextBrushes, nextPlayers) => { brushes = nextBrushes; players = motionColliders(nextPlayers); } };
}

export { assetURL } from './asset-contract.ts';
