import { createRoot } from 'react-dom/client';
import { useRef, useState } from 'react';
import { Board } from '../web/Board';
import { GameAudio } from '../../../web/src/audio';
import { GameFullscreen, type GameFullscreenHandle } from '../../../web/src/ui/room-fullscreen';
import type { GameAudioPort, GameRoom } from '../../../web/src/game-api/module';
import type { Player, View } from '../web/types';

const seats = [{ id: 'clock-t', name: '计时验收', ready: true }, { id: 'clock-ct', name: '对手', ready: true }];
const room: GameRoom = { id: 'clock-fixture', name: '计时房间', kind: 'cs16', host: seats[0].id, capacity: 2, status: 'playing', version: 1, you: 0, seats };
const player: Player = { seat: 0, team: 't', character: 'terror', x: 0, y: 0, z: 0, vx: 0, vy: 0, vz: 0, yaw: 0, pitch: 0, alive: true, visible: true, life: 1, health: 100, armor: 0, money: 800, weapon: 'glock18', ammo: 20, reserve: 120, inventory: [{ id: 'glock18', ammo: 20, reserve: 120 }], kills: 0, deaths: 0, crouch: false, grounded: true, receivedSeq: 0, appliedSeq: 0 };
const initial: View = { schema: 1, epoch: 'clock-audio', serverAt: 0, startAt: 0, frame: 100, mapId: 'de_dust2', rulesSHA256: 'a'.repeat(64), options: { map: 'de_dust2', roundSeconds: 120, maxRounds: 30, friendlyFire: false, teams: ['t', 'ct'], characters: ['terror', 'urban'] }, players: [player, { ...player, seat: 1, team: 'ct', character: 'urban' }], events: [], eventId: 0, round: 1, phase: 'live', phaseEndsAt: 12100, buyAllowed: true, buyZone: true, teamScores: [0, 0], bomb: { carrier: 0, dropped: false, planted: false, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 }, hostages: [], grenades: [], drops: [], finished: false, you: 0, canAct: true };
const calls: { method: string; args: unknown[]; at: number; hidden: boolean; result?: boolean }[] = [];
const media: { type: string; source: string; lane: string; at: number; hidden: boolean; paused: boolean; currentTime: number; duration: number | null }[] = [];
const platform = new GameAudio();
const channel = platform.channel('cs16-clock-fixture');
const audio = new Proxy(channel, { get(target, key) {
  const method = Reflect.get(target, key);
  if (typeof method !== 'function') return method;
  return (...args: unknown[]) => {
    const call = { method: String(key), args, at: performance.now(), hidden: document.hidden } as typeof calls[number];
    if (['effect', 'playWithFallback', 'spatial', 'stopEffect'].includes(String(key))) calls.push(call);
    const result = method(...args);
    if (result instanceof Promise) void result.then(value => { call.result = Boolean(value); });
    return result;
  };
} }) as GameAudioPort;
for (const type of ['playing', 'pause', 'ended', 'loadedmetadata', 'error']) document.addEventListener(type, event => {
  const element = event.target;
  if (!(element instanceof HTMLAudioElement) || !element.src.includes('/cs16/')) return;
  media.push({ type, source: element.currentSrc || element.src, lane: element.dataset.platformEffectLane ?? '', at: performance.now(), hidden: document.hidden, paused: element.paused, currentTime: element.currentTime, duration: Number.isFinite(element.duration) ? element.duration : null });
}, true);

function App() {
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [ready, setReady] = useState(false);
  const [sound, setSound] = useState(true);
  const [paused, setPaused] = useState(false);
  const [data, setData] = useState(initial);
  const update = (patch: Partial<View>) => setData(current => ({ ...current, frame: current.frame + 1, recover: false, ...patch }));
  Object.assign(window, { cs16ClockAudit: {
    calls, media,
    update,
    cue: (code: number, recover = false) => update({ bombCue: code, recover }),
    planted: (planted: boolean) => setData(current => ({ ...current, frame: current.frame + 1, recover: false, bombCue: 0, bomb: { ...current.bomb, planted, carrier: planted ? -1 : 0, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 } })),
    progress: (kind: string) => setData(current => ({ ...current, frame: current.frame + 1, players: [{ ...current.players[0], action: kind, actionSince: current.frame - 100, defuser: true }, current.players[1]], bomb: { ...current.bomb, planted: kind === 'defuse', defuser: kind === 'defuse' ? 0 : -1, defuseAt: current.frame + 350 } })),
    muted: (muted: boolean) => { platform.setEnabled(!muted); setSound(!muted); },
    paused: (paused: boolean) => setPaused(paused),
    ready: () => ready,
  } });
  return <>
    <button id="unlock" onClick={() => { void platform.unlock(); }}>启用声音</button>
    <button id="fullscreen" disabled={!ready} onClick={() => fullscreen.current?.enter()}>全屏</button>
    <GameFullscreen ref={fullscreen} scope="clock-audio-fixture" enabled={ready} onReadyChange={() => {}} toolbarOverlay>
      <Board room={room} data={data} act={async () => {}} command={async () => {}} inputWindow={64} busy={false} assets="" sound={sound} audioEnabled={sound} audio={audio} clock={{ paused, serverNow: 0, receivedAt: 0 }} onReady={() => setReady(true)} />
    </GameFullscreen>
  </>;
}

createRoot(document.getElementById('root')!).render(<App />);
