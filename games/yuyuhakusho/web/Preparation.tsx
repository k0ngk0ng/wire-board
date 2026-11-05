import type { PreparationProps } from "../../../web/src/game-api/module";
import type { GameOptions } from "./types";
import { characters, arenas } from "./labels";
import "./network.css";
export function Preparation({
  room,
  options,
  host,
  busy,
  prepare,
}: PreparationProps<GameOptions>) {
  const participating = room.you >= 0 && !room.spectating;
  return (
    <section className="yuyu-preparation" aria-label="比赛准备">
      <div className="yuyu-room-rules">
        <label>
          场地
          <select
            aria-label="场地"
            value={options.arena}
            disabled={busy || !host || !participating}
            onChange={(event) =>
              prepare({ type: "arena", arena: Number(event.target.value) })
            }
          >
            {arenas.map((arena) => (
              <option key={arena.id} value={arena.id}>
                {arena.name}
              </option>
            ))}
          </select>
        </label>
        {room.capacity === 4 && (
          <label>
            赛制
            <select
              aria-label="赛制"
              value={options.teamBattle ? "team" : "free"}
              disabled={busy || !host || !participating}
              onChange={(event) =>
                prepare({
                  type: "mode",
                  teamBattle: event.target.value === "team",
                })
              }
            >
              <option value="free">四人混战</option>
              <option value="team">双人组队对战</option>
            </select>
          </label>
        )}
      </div>
      <div className="yuyu-roster">
        {options.characters.map((character, seat) => {
          const player = room.seats[seat],
            editable =
              participating && (seat === room.you || (host && !!player?.bot));
          return (
            <label className="yuyu-role" key={seat}>
              <span>
                {player?.name ?? "等待玩家"}
                {seat === room.you ? "（你）" : ""}
              </span>
              <select
                data-character-seat={seat}
                aria-label={`${player?.name ?? "空席"}的角色`}
                value={character}
                disabled={busy || !editable || !player}
                onChange={(event) =>
                  prepare({
                    type: "character",
                    seat,
                    character: Number(event.target.value),
                  })
                }
              >
                {characters.map((name, id) => (
                  <option key={id} value={id}>
                    {name}
                  </option>
                ))}
              </select>
            </label>
          );
        })}
      </div>
    </section>
  );
}
