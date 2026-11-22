import type { GameRoom } from '../../../web/src/game-api/module';
import type { View } from './types';
import './scoreboard.css';

export function Scoreboard({ room, view, close }: { room: GameRoom; view: View; close: () => void }) {
  return <section className="cs16-scoreboard" aria-label="对局战绩">
    <header><strong>{room.name}</strong><button type="button" data-cs16-score-close aria-label="关闭战绩" onClick={close}>×</button></header>
    <table>
      <colgroup><col className="cs16-score-name" /><col className="cs16-score-status" /><col className="cs16-score-number" /><col className="cs16-score-number" /><col className="cs16-score-ping" /></colgroup>
      <thead><tr><th scope="col">玩家</th><th scope="col" aria-label="状态" /><th scope="col">得分</th><th scope="col">死亡</th><th scope="col">延迟</th></tr></thead>
      {(['t', 'ct'] as const).map((team, index) => {
        const players = view.players.filter(player => player.team === team).sort((left, right) => right.kills - left.kills || left.deaths - right.deaths || left.seat - right.seat);
        return <tbody key={team} data-cs16-score-team={team}>
          <tr className="cs16-score-team"><th scope="row" colSpan={2}>{team === 't' ? '恐怖分子' : '反恐精英'}<span> — {players.length} 人</span></th><td>{view.teamScores[index] ?? 0}</td><td /><td /></tr>
          {players.map(player => {
            const seat = room.seats[player.seat];
            const latency = seat?.network?.rttMs;
            const ping = seat?.bot ? 'BOT' : seat?.network?.connected && typeof latency === 'number' && Number.isFinite(latency) ? String(Math.max(0, Math.round(latency))) : '—';
            const teammate = view.players.find(candidate => candidate.seat === room.you)?.team === team;
            const status = !player.alive ? '阵亡' : teammate && view.bomb.carrier === player.seat ? 'C4' : '';
            return <tr key={player.seat} data-cs16-score-seat={player.seat} data-cs16-score-own={player.seat === room.you && !room.spectating || undefined}><th scope="row" title={seat?.name}>{seat?.name ?? '玩家'}</th><td>{status}</td><td>{player.kills}</td><td>{player.deaths}</td><td>{ping}</td></tr>;
          })}
        </tbody>;
      })}
    </table>
  </section>;
}
