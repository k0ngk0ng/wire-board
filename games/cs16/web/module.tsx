import { defineGame } from '../../../web/src/game-api/module';
import { Board } from './Board';
import catalog from './catalog';
import { defaultOptions, itemName } from './content';
import { Options, Preparation } from './Preparation';
import { Rules } from './Rules';
import type { GameOptions, View } from './types';

export default defineGame<View, GameOptions>({
  ...catalog, assetLoading: 'board', asyncBoard: true, preparationOwnsOptions: true, fullscreenToolbarOverlay: true,
  playerLayout: { density: 'compact', groups: (room, data) => ['t', 'ct'].map(team => { const seats = data?.players.filter(player => player.team === team).map(player => player.seat) ?? []; return { id: team, label: `${team === 'ct' ? 'CT · 反恐精英' : 'T · 恐怖分子'} · ${seats.length} 人`, seats }; }) },
  defaultOptions, Board, Options, Preparation, Rules,
  isInputRelease: action => action.type === 'input' && action.cancel === true && action.moveX === 0 && action.moveY === 0 && action.fire === false && action.secondary === false && action.jump === false && action.crouch === false && action.walk === false && action.reload === false && action.use === false && action.drop === false && !action.select,
  PlayerSummary: ({ room, data, seat }) => {
    const player = data.players.find(player => player.seat === seat);
    if (!player) return null;
    return <div className={`cs16-player-summary cs16-summary-${player.team}`}><strong>{player.alive ? '存活' : '阵亡'}{seat === room.you ? ' · 你' : ''}</strong><span>{player.kills} 杀 / {player.deaths} 死</span>{player.health !== undefined && <span>生命 {player.health} · 护甲 {player.armor ?? 0}</span>}{player.money !== undefined && <span>${player.money} · {itemName(player.weapon)}</span>}</div>;
  },
  ResultDetails: ({ room, data }) => <div className="cs16-result-details"><strong>恐怖分子 {data.teamScores[0]} : {data.teamScores[1]} 反恐精英</strong>{['t', 'ct'].map(team => <section key={team}><h3>{team === 'ct' ? '反恐精英' : '恐怖分子'}</h3>{data.players.filter(player => player.team === team).map(player => <div key={player.seat}><span>{room.seats[player.seat]?.name ?? '玩家'}</span><span>{player.kills} 杀 / {player.deaths} 死</span></div>)}</section>)}</div>,
});
