import * as T from 'three';
import { GLTFLoader, type GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { assetURL, loadManifest, readResource, type LoadedAssets } from '../web/assets.ts';
import { decodeEmbeddedTexture, assertModelTextures } from '../web/embedded-textures.ts';
import { BattleRenderer } from '../web/renderer.ts';
import { soundCues } from '../web/classic-hud.ts';
import type { GameEvent, MapRules, Player, RenderMap, Resource, View } from '../web/types.ts';

const canvas = document.createElement('canvas');
canvas.style.cssText = 'width:640px;height:480px;display:block';
document.body.style.margin = '0';
document.body.append(canvas);
const manifest = await loadManifest('');
const selected = manifest.maps.cs_bloodstrike;
const abort = new AbortController();
const read = (resource: Resource) => readResource('', resource, abort.signal);
const map = JSON.parse(new TextDecoder().decode(await read(selected.render))) as RenderMap;
const rules = JSON.parse(new TextDecoder().decode(await read(selected.rules!))) as MapRules;
const images = new Map<string, ImageBitmap>();
const files = new Map<string, Resource>([['lightmap', map.lightmap], ...Object.entries(map.textures).map(([name, resource]) => [`texture:${name}`, resource] as [string, Resource]), ...(selected.sky ?? []).map((resource, index) => [`sky:${index}`, resource] as [string, Resource]), ...Object.entries(manifest.effects ?? {}).map(([name, sprite]) => [`effect:${name}`, sprite.atlas] as [string, Resource])]);
await Promise.all([...files].map(async ([name, resource]) => images.set(name, await createImageBitmap(new Blob([new Uint8Array(await read(resource))], { type: resource.mime ?? 'image/png' }), { imageOrientation: 'none', premultiplyAlpha: 'none' }))));
const models = new Map<string, GLTF>();
const loader = new GLTFLoader();
loader.register(parser => ({ name: 'CS16_effect_fixture_textures', loadTexture: index => decodeEmbeddedTexture(parser, index, blob => createImageBitmap(blob, { imageOrientation: 'none', premultiplyAlpha: 'none', colorSpaceConversion: 'none' }), (image, sampler, name) => {
  const texture = new T.Texture(image);
  texture.name = name; texture.flipY = false; texture.colorSpace = T.SRGBColorSpace;
  texture.magFilter = sampler.magFilter === 9728 ? T.NearestFilter : T.LinearFilter;
  texture.minFilter = T.LinearMipmapLinearFilter; texture.needsUpdate = true;
  return texture;
}) }));
for (const name of new Set(['player-terror', 'player-urban', 'v-glock18', 'p-glock18', 'v-awp', 'v-scout', ...map.decorations?.map(item => item.model) ?? []])) {
  const model = await loader.parseAsync(new Uint8Array(await read(manifest.models[name])).buffer, '');
  assertModelTextures(model.scene); models.set(name, model);
}
const sounds = new Map<string, string>();
for (const name of ['weapons/explode3.wav', 'weapons/explode4.wav', 'weapons/explode5.wav', 'weapons/debris1.wav', 'weapons/debris2.wav', 'weapons/debris3.wav']) { await read(manifest.sounds[name]); sounds.set(name, assetURL('', manifest.sounds[name].file)); }
const assets: LoadedAssets = { manifest, map, rules, images, models, sounds, weapons: [], motion: () => { throw Error('Renderer fixture never executes motion rules'); }, canSmooth: () => { throw Error('Renderer fixture never executes motion rules'); }, dynamics: () => { throw Error('Renderer fixture never executes motion rules'); }, audit: { models: models.size, materialCount: 0, texturedMaterialCount: 0, missingTextures: [] } };
const spawn = assets.rules.spawnsT[0];
const player: Player = { seat: 0, team: 't', character: 'terror', x: spawn.x, y: spawn.y, z: spawn.z, yaw: 0, pitch: 0, vx: 0, vy: 0, vz: 0, alive: true, life: 1, visible: true, health: 100, armor: 0, money: 800, weapon: 'glock18', ammo: 20, reserve: 120, inventory: [{ id: 'glock18', ammo: 20, reserve: 120 }], kills: 0, deaths: 0, crouch: false, grounded: true, receivedSeq: 0, appliedSeq: 0 };
let enemy: Player = { ...player, seat: 1, team: 'ct', character: 'urban', x: spawn.x + 96 * 256 };
let view: View = { schema: 1, epoch: 'combat-effects', serverAt: 0, startAt: 0, frame: 100, mapId: 'cs_bloodstrike', options: { map: 'cs_bloodstrike', roundSeconds: 120, maxRounds: 30, friendlyFire: false, teams: ['t', 'ct'], characters: ['terror', 'urban'] }, players: [player, enemy], events: [], eventId: 0, round: 1, phase: 'live', phaseEndsAt: 0, teamScores: [0, 0], bomb: { carrier: -1, dropped: false, planted: false, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 }, hostages: [], grenades: [], drops: [], finished: false, you: 0, canAct: true };
let now = 1000;
const renderer = new BattleRenderer(canvas, assets);
const internals = renderer as unknown as { scene: T.Scene; combat?: { root: T.Group } };
const draw = () => renderer.draw(view, player, 0, now);
renderer.update(view, now);
draw();
await renderer.prepare();
let baseline: Uint8Array | undefined;
const image = (mark = false) => {
  draw();
  const gl = renderer.renderer.getContext();
  const bytes = new Uint8Array(canvas.width * canvas.height * 4);
  gl.readPixels(0, 0, canvas.width, canvas.height, gl.RGBA, gl.UNSIGNED_BYTE, bytes);
  let hash = 2166136261, changed = 0, changedRed = 0, changedOrange = 0, lines = 0;
  for (let i = 0; i < bytes.length; i += 4) {
    for (let channel = 0; channel < 3; channel++) hash = Math.imul(hash ^ bytes[i + channel], 16777619);
    if (!baseline || bytes[i] === baseline[i] && bytes[i + 1] === baseline[i + 1] && bytes[i + 2] === baseline[i + 2]) continue;
    changed++;
    if (bytes[i] >= 20 && bytes[i] > bytes[i + 1] * 2 && bytes[i] > bytes[i + 2] * 2) changedRed++;
    if (bytes[i] > 100 && bytes[i + 1] > 35 && bytes[i] > bytes[i + 2] * 2) changedOrange++;
  }
  internals.scene.traverse(object => { if ((object as T.Line).isLine) lines++; });
  const sprites = internals.combat?.root.children.filter(object => object.visible).map(object => ({ name: object.userData.effect, frame: object.userData.frame, position: object.position.toArray(), scale: object.scale.toArray(), source: object.userData.sourceSHA256 })) ?? [];
  if (mark) baseline = bytes;
  return { hash: hash >>> 0, changed, changedRed, changedOrange, lines, sprites, ...renderer.visualStats };
};
Object.assign(window, { cs16CombatAudit: {
  image,
  event: (values: Partial<GameEvent>) => { const event: GameEvent = { id: 1, frame: view.frame, type: 'hit', seat: 0, target: 1, weapon: 'ak47', hitgroup: 2, fx: 1, value: 25, life: enemy.life, x: enemy.x, y: enemy.y, z: enemy.z, ...values }; renderer.event(event, now, 0); draw(); return event; },
  advance: (milliseconds: number) => { now += milliseconds; draw(); },
  clear: () => { view = { ...view, round: view.round + 1 }; renderer.update(view, now); draw(); },
  hidden: (hidden: boolean) => { enemy = { ...enemy, visible: !hidden }; view = { ...view, frame: view.frame + 1, players: [player, enemy] }; renderer.update(view, now); draw(); },
  arm: (weapon: string, zoom: number) => { player.weapon = weapon; player.inventory = [{id:weapon,ammo:10,reserve:10,zoom}]; view = {...view,round:view.round+1}; renderer.update(view,now); draw(); },
  shoot: () => { renderer.feedback('shot', now); draw(); },
  audio: (id: number) => { const cue = soundCues({ type: 'explode', weapon: 'hegrenade', id }); return { cue, sources: [cue.files, ...cue.layers?.map(layer => layer.files) ?? []].map(files => files.map(file => ({ file, resource: manifest.sounds[file], loaded: assets.sounds.has(file) }))) }; },
  dispose: () => renderer.dispose(),
} });
document.body.dataset.ready = 'true';
