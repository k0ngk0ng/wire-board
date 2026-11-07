import type { PreparationProps } from "../../../web/src/game-api/module";
import type { GameOptions } from "./types";
import { CharacterPicker } from "./CharacterPicker";
import { ArenaPicker } from "./ArenaPicker";
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
        <ArenaPicker value={options.arena} disabled={busy || !host || !participating}
          onChange={arena => prepare({ type: "arena", arena })} />
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
            <div className="yuyu-role" key={seat}>
              <span>
                {player?.name ?? "等待玩家"}
                {seat === room.you ? "（你）" : ""}
              </span>
              <CharacterPicker
                seat={seat}
                label={`${player?.name ?? "空席"}的角色`}
                value={character}
                disabled={busy || !editable || !player}
                onChange={(character) =>
                  prepare({
                    type: "character",
                    seat,
                    character,
                  })
                }
              />
            </div>
          );
        })}
      </div>
    </section>
  );
}
