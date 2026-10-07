import type { Room } from "./types";
import { catanSeatColor } from "./catan-player-colors";

export function ExplorerPairedTurn({ room }: { room: Room }) {
  const game = room.game!,
    g = game.catan!,
    pair = g.paired;
  if (!pair) return null;
  const setup = !!g.explorer?.setup;
  const playing = room.status === "playing" && !game.finished && !setup;
  return (
    <section className="explorer-paired" aria-label="本轮两位玩家">
      <ol>
        {[pair.primary, pair.secondary].map((seat, index) => {
          const active = playing && pair.second === (index === 1);
          return (
            <li
              key={index}
              className={active ? "is-active" : ""}
              style={{ borderLeftColor: catanSeatColor(g, seat) }}
              aria-current={active ? "step" : undefined}
            >
              <div>
                <b>{index + 1}</b>
                <strong title={room.seats[seat]?.name}>
                  {room.seats[seat]?.name}
                </strong>
                {active && <em>当前</em>}
              </div>
              <span>
                {index === 0 ? "生产 → 建设 → 航行" : "建设 → 航行 · 不掷骰"}
              </span>
            </li>
          );
        })}
      </ol>
      <p>
        {game.finished || room.status !== "playing"
          ? "本局已结束"
          : setup
            ? "完成开局放置后，按以上顺序行动。"
            : pair.second
              ? "第二位可向银行交易，不能与其他玩家交易；航行结束后两枚标记顺移。"
              : "第一位行动结束后，第二位直接建设、航行；两位各有自己的行动时间。"}
      </p>
    </section>
  );
}
