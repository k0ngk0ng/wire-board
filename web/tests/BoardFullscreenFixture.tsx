import { useEffect, useRef, useState, type ComponentType } from 'react';
import { createRoot } from 'react-dom/client';
import { Save } from 'lucide-react';
import { GameFullscreen, type GameFullscreenHandle } from '../src/ui/room-fullscreen';
import { RoomToolButton } from '../src/ui/room-tool-button';
import { Chat } from '../src/chat';
import type { BoardProps, GameAudioPort } from '../src/game-api/module';
import type { Room } from '../src/types';
import { Board as Gomoku } from '../../games/gomoku/web/Board';
import { Board as Weiqi } from '../../games/weiqi/web/Board';
import { Board as Chess } from '../../games/chess/web/Board';
import { Board as Xiangqi } from '../../games/xiangqi/web/Board';
import { Board as Junqi } from '../../games/junqi/web/Board';
import fixtures from './fixtures/board-fullscreen.json';
import '../src/style.css';
import '../../games/gomoku/web/style.css';
import '../../games/weiqi/web/style.css';
import '../../games/chess/web/style.css';
import '../../games/xiangqi/web/style.css';
import '../../games/junqi/web/style.css';

const query = new URLSearchParams(location.search);
const current = fixtures.cases.find(value => value.id === query.get('case')) ?? fixtures.cases[0];
const data = structuredClone(current.data);
const probe = { ready: false, mounts: 0, actions: [] as Record<string, unknown>[], keys: [] as string[] };
Object.assign(window, { fullscreenBoardProbe: probe, fullscreenBoardCase: current });
const room = { id: 'fullscreen-layout', kind: current.kind, name: '棋盘布局验收', host: 'player-0', capacity: current.players, status: 'playing', you: current.you, seats: Array.from({ length: current.players }, (value, seat) => { void value; return { id: 'player-' + seat, name: '玩家' + (seat + 1), ready: true }; }), chat: [], game: { data }, version: 1 } as unknown as Room;
const audio = { stop: () => {}, playWithFallback: async () => true } as unknown as GameAudioPort;
const boards = { gomoku: Gomoku, weiqi: Weiqi, chess: Chess, xiangqi: Xiangqi, junqi: Junqi } as unknown as Record<string, ComponentType<BoardProps<unknown>>>;
const assets = (window as unknown as { fullscreenBoardAssets: string }).fullscreenBoardAssets + '/games/' + current.kind + '/assets/public';
const act = async (action: Record<string, unknown>) => { probe.actions.push(action); };

function Board() {
  useEffect(() => { probe.mounts++; probe.ready = true; }, []);
  const ActiveBoard = boards[current.kind];
  if (query.has('collision')) return <section data-collision-stage data-game-focus tabIndex={-1} style={{ position: 'relative', width: '100%', height: '100%', background: '#cedfc0' }}><div data-fullscreen-protected style={{ position: 'absolute', top: 8, left: 8, width: 120, height: 44, background: '#173e35', color: '#fff' }}>重要 HUD</div><button data-game-corner style={{ position: 'absolute', top: 8, right: 8, width: 80, height: 44 }} onClick={() => void act({ type: 'corner' })}>游戏按钮</button></section>;
  return <ActiveBoard room={room} data={data} act={act} command={async () => {}} busy={false} assets={assets} sound={false} audio={audio} clock={{ serverNow: 1700000000000, receivedAt: performance.now(), epoch: 0 }} />;
}

function App() {
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [ready, setReady] = useState(false);
  const [target, setTarget] = useState<HTMLElement | null>(null);
  useEffect(() => {
    if (query.has('safe')) {
      const surface = document.querySelector<HTMLElement>('[data-game-fullscreen-surface]');
      surface?.style.setProperty('--game-fullscreen-safe-top', '44px');
      surface?.style.setProperty('--game-fullscreen-safe-right', '20px');
    }
    const listener = (event: KeyboardEvent) => { if (event.composedPath().some(node => node instanceof Element && node.hasAttribute('data-game-fullscreen-surface'))) probe.keys.push(event.key); };
    document.addEventListener('keydown', listener);
    return () => document.removeEventListener('keydown', listener);
  }, []);
  return <main className="fixture-page" style={{ padding: 16, maxWidth: 1140, margin: 'auto' }}><header data-platform><h1>平台房间</h1><input data-outside defaultValue="区外编辑器" /><button data-enter disabled={!ready} onClick={() => fullscreen.current?.enter()}>全屏</button></header><GameFullscreen ref={fullscreen} scope={'layout-' + current.id} enabled toolbarOverlay={query.has('owned') ? true : undefined} onReadyChange={setReady} onChatTargetChange={setTarget} status={<section className="turn-card"><h3>轮到玩家一</h3><div className="turn-clock"><div className="clock-reading"><time>1:59</time></div></div></section>} toolbarActions={<RoomToolButton label="保存" onClick={() => void act({ type: 'save' })}><Save size={16} /></RoomToolButton>}><Board /></GameFullscreen><Chat room={room} userId={room.seats[room.you].id} send={async () => {}} fullscreenTarget={target} /></main>;
}

createRoot(document.getElementById('root')!).render(<App />);
