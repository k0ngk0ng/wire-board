import { createRoot } from 'react-dom/client';
import { useRef, useState } from 'react';
import { Board } from '../web/Board';
import { GameFullscreen, type GameFullscreenHandle } from '../../../web/src/ui/room-fullscreen';
import type { GameAudioPort, GameRoom } from '../../../web/src/game-api/module';
import type { Player, View } from '../web/types';

const seats = Array.from({ length: 10 }, (_, seat) => ({ id: `score-${seat}`, name: seat === 0 ? '经典玩家' : seat === 2 ? '这是一个用于验收截断的很长很长玩家名称' : `玩家 ${seat + 1}`, ready: true, bot: seat >= 4, network: { connected: true, rttMs: seat === 3 ? null : 24 + seat } }));
const room: GameRoom = { id: 'score-fixture', name: '经典房间', kind: 'cs16', host: seats[0].id, capacity: 10, status: 'playing', version: 1, you: 0, seats };
const players: Player[] = seats.map((_, seat) => ({ seat, team: seat % 2 ? 'ct' : 't', character: seat % 2 ? 'urban' : 'terror', x: seat * 256, y: 0, z: 0, vx: 0, vy: 0, vz: 0, yaw: 0, pitch: 0, alive: seat !== 2, visible: true, life: 1, health: seat === 2 ? 0 : 100, armor: 0, money: 16000, weapon: 'glock18', ammo: 20, reserve: 120, inventory: [{ id: 'glock18', ammo: 20, reserve: 120 }], kills: seat % 4, deaths: seat % 3, crouch: false, grounded: true, receivedSeq: 0, appliedSeq: 0 }));
const initial: View = { schema: 1, epoch: 'scoreboard', serverAt: 0, startAt: 0, frame: 1, mapId: 'cs_bloodstrike', rulesSHA256: 'a'.repeat(64), options: { map: 'cs_bloodstrike', roundSeconds: 120, maxRounds: 30, friendlyFire: false, teams: players.map(player => player.team), characters: players.map(player => player.character) }, players, events: [], eventId: 0, round: 4, phase: 'live', phaseEndsAt: 12000, buyAllowed: true, buyZone: true, teamScores: [3, 2], bomb: { carrier: -1, dropped: false, planted: false, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 }, hostages: [], grenades: [], drops: [], finished: false, you: 0, canAct: true };
const audio = new Proxy({}, { get: () => () => Promise.resolve(true) }) as GameAudioPort;
const events: { type: string; trusted: boolean; code?: string; target: string; active: string }[] = [];
const inputs: Record<string, unknown>[] = [];
let revision = 1;
for (const type of ['keydown', 'keyup', 'focusin', 'visibilitychange', 'touchstart', 'touchend']) window.addEventListener(type, event => { events.push({ type, trusted: event.isTrusted, code: event instanceof KeyboardEvent ? event.code : undefined, target: event.target instanceof HTMLElement ? event.target.id || event.target.className : '', active: document.activeElement?.id ?? '' }); }, true);

function App() {
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [ready, setReady] = useState(false);
  const [data, setData] = useState(initial);
  const [spectating, setSpectating] = useState(false);
  const [chat, setChat] = useState(false);
  const [paused, setPaused] = useState(false);
  Object.assign(window, { cs16ScoreboardAudit: {
    events,
    inputs,
    paused: (value: boolean) => setPaused(value),
    chat: (visible: boolean) => setChat(visible),
    scoreKey: (code: string) => window.dispatchEvent(new CustomEvent('controller-settings-changed', { detail: { userId: seats[0].id, model: 'game:cs16', profile: { revision: ++revision, turboHz: 10, keyboard: { scoreboard: code, buy: 'KeyB', forward: 'KeyW', fire: 'KeyJ' } } } })),
    alive: (alive: boolean) => setData(current => ({ ...current, players: current.players.map(player => player.seat === 0 ? { ...player, alive, health: alive ? 100 : 0, life: player.life + 1 } : player) })),
    spectating: (value: boolean) => setSpectating(value),
  } });
  return <>
    <input id="outside-chat" aria-label="页面聊天" /><button id="outside-next">页面下一项</button>
    <button id="fullscreen" disabled={!ready} onClick={() => fullscreen.current?.enter()}>全屏</button>
    <GameFullscreen ref={fullscreen} scope="scoreboard-fixture" enabled={ready} onReadyChange={() => {}} toolbarOverlay>
      <Board room={{ ...room, spectating }} data={data} act={async action => { inputs.push(action); if (action.type === 'input') setData(current => ({ ...current, players: current.players.map(player => player.seat === 0 ? { ...player, receivedSeq: Number(action.seq), appliedSeq: Number(action.seq) } : player) })); }} inputWindow={64} command={async () => {}} busy={false} assets="" sound={false} audio={audio} clock={{ paused, serverNow: 0, receivedAt: 0 }} onReady={() => setReady(true)} />
      <div className="table-chat" hidden={!chat}><input id="fullscreen-chat" aria-label="全屏聊天" /><button id="chat-send">发送</button><div id="editable" contentEditable>编辑内容</div></div>
    </GameFullscreen>
  </>;
}

createRoot(document.getElementById('root')!).render(<App />);
