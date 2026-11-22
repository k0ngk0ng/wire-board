import * as T from 'three';
import { loadBattleAssets, loadManifest } from '../web/assets.ts';
import { BattleRenderer } from '../web/renderer.ts';
import type { Player, View } from '../web/types.ts';
import '../web/style.css';

const canvas = document.createElement('canvas');
canvas.style.cssText = 'width:640px;height:480px;display:block';
document.body.style.margin = '0';
document.body.append(canvas);
document.body.dataset.stage = 'assets';
const manifest = await loadManifest('');
const rules = manifest.maps.cs_bloodstrike.rules;
if (!rules) throw Error('Renderer fixture requires the local map rules');
const assets = await loadBattleAssets('', 'cs_bloodstrike', ['terror', 'urban'], new AbortController().signal, (percent, label, bytes) => { document.body.dataset.stage = label; document.body.dataset.progress = String(percent); document.body.dataset.bytes = JSON.stringify(bytes); }, rules.decodedSHA256 ?? rules.sha256);
document.body.dataset.stage = 'renderer';
const spawn = assets.rules.spawnsT[0];
let player: Player = { seat: 0, team: 't', character: 'terror', x: spawn.x, y: spawn.y, z: spawn.z, yaw: 0, pitch: 0, vx: 0, vy: 0, vz: 0, alive: true, life: 1, visible: true, health: 100, armor: 0, money: 800, weapon: 'glock18', ammo: 20, reserve: 120, inventory: [{ id: 'glock18', ammo: 20, reserve: 120, zoom: 0 }], kills: 0, deaths: 0, crouch: false, grounded: true, receivedSeq: 0, appliedSeq: 0 };
let enemy: Player = { ...player, seat: 1, team: 'ct', character: 'urban', x: spawn.x + 96 * 256 };
let view: View = { schema: 1, epoch: 'fixture', serverAt: 0, startAt: 0, frame: 100, mapId: 'cs_bloodstrike', options: { map: 'cs_bloodstrike', roundSeconds: 120, maxRounds: 30, friendlyFire: false, teams: ['t', 'ct'], characters: ['terror', 'urban'] }, players: [player, enemy], events: [], eventId: 0, round: 1, phase: 'live', phaseEndsAt: 0, teamScores: [0, 0], bomb: { carrier: -1, dropped: false, planted: false, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 }, hostages: [], grenades: [], drops: [], finished: false, you: 0, canAct: true };
const renderer = new BattleRenderer(canvas, assets);
const internals = renderer as unknown as { camera: T.PerspectiveCamera; observedSeat: number; weapon?: { animation: string; root: T.Object3D }; weaponRoot: T.Object3D; actors: Map<number, { animation: string; alive: boolean; root: T.Object3D }> };
const update = () => { view = { ...view, players: [player, enemy] }; renderer.update(view, performance.now()); };
update();
renderer.draw(view, player, 0, performance.now());
document.body.dataset.stage = 'compile';
await renderer.prepare();
let stopped = false;
const draw = () => { if (stopped) return; renderer.draw(view, player.alive ? player : undefined, 0, performance.now()); requestAnimationFrame(draw); };
draw();
const stats = () => ({ ...renderer.visualStats, cameraFov: internals.camera.fov, cameraSeat: internals.observedSeat, cameraQuaternion: internals.camera.quaternion.toArray(), weaponAnimation: internals.weapon?.animation, weaponZ: internals.weaponRoot.position.z, cameraY: internals.camera.position.y, actors: [...internals.actors].map(([seat, actor]) => ({ seat, animation: actor.animation, alive: actor.alive, visible: actor.root.visible, position: actor.root.position.toArray() })) });
const image = () => new Promise(resolve => requestAnimationFrame(() => {
  renderer.draw(view, player.alive ? player : undefined, 0, performance.now());
  const gl = renderer.renderer.getContext();
  const bytes = new Uint8Array(64 * 64 * 4);
  gl.readPixels(canvas.width / 2 - 32, canvas.height / 2 - 32, 64, 64, gl.RGBA, gl.UNSIGNED_BYTE, bytes);
  let hash = 2166136261, light = 0;
  const rgb = [0, 0, 0];
  for (let i = 0; i < bytes.length; i++) { hash = Math.imul(hash ^ bytes[i], 16777619); if (i % 4 !== 3) { light += bytes[i]; rgb[i % 4] += bytes[i]; } }
  resolve({ hash: hash >>> 0, light, rgb, ...stats() });
}));
Object.assign(window, { cs16RendererAudit: {
  set: (values: Partial<Player>) => { player = { ...player, ...values }; view = { ...view, frame: view.frame + 1 }; update(); },
  enemy: (values: Partial<Player>) => { enemy = { ...enemy, ...values }; view = { ...view, frame: view.frame + 1 }; update(); },
  frame: (frame: number) => { view = { ...view, frame }; update(); },
  zoom: (zoom: number) => { player = { ...player, weapon: 'awp', inventory: [{ id: 'awp', ammo: 10, reserve: 30, zoom }] }; view = { ...view, frame: view.frame + 1 }; update(); },
  feedback: (kind: Parameters<BattleRenderer['feedback']>[0]) => renderer.feedback(kind, performance.now()),
  event: (event: Parameters<BattleRenderer['event']>[0]) => renderer.event(event, performance.now(), 0),
  stats, image,
  stop: () => { stopped = true; renderer.dispose(); },
} });
document.body.dataset.ready = 'true';
document.body.dataset.stage = 'ready';
