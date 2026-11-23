import type { CSSProperties } from "react";
import type { Room } from "./types";
import { gemColors } from "./colors";
import { gemNames } from "./splendor-orient-state";

export function SplendorFullscreenPlayers({ room }: { room: Room }) {
  const game = room.game!;
  return <div className="splendor-fullscreen-players" aria-label="玩家公开信息" data-fullscreen-protected>
    {game.splendor!.players.map((player, seat) => <section
      key={room.seats[seat].id}
      className="splendor-fullscreen-player"
      data-splendor-public-player={seat}
      data-self={seat === room.you && !room.spectating || undefined}
      aria-current={game.turn === seat && !game.finished ? "true" : undefined}
      aria-label={`${room.seats[seat].name}的公开信息`}
    >
      <header><strong>{room.seats[seat].name}</strong><b>{player.score} 分</b><span data-public-reserved={player.reserved?.length ?? player.reservedCount ?? 0}>预留 {player.reserved?.length ?? player.reservedCount ?? 0}</span></header>
      <div className="splendor-public-resources">
        {player.tokens.map((tokens, color) => <div key={color} style={{ "--public-gem": gemColors[color], "--public-ink": color === 1 || color === 5 ? "#302b20" : "#fff" } as CSSProperties} aria-label={`${gemNames[color]}，${color < 5 ? `永久折扣 ${player.bonus[color]}，` : ""}筹码 ${tokens}`}>
          {color < 5 ? <b data-public-bonus={color} title={`${gemNames[color]}永久折扣`}>{player.bonus[color]}</b> : <span />}
          <span data-public-token={color} title={`${gemNames[color]}筹码`}>{tokens}</span>
        </div>)}
      </div>
    </section>)}
  </div>;
}
