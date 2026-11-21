import type { PreparationProps } from "../../../web/src/game-api/module";
import type { GameOptions } from "./types";
import { CharacterPicker } from "./CharacterPicker";
import { ArenaPicker } from "./ArenaPicker";
import { teamGroups } from "./labels";
import "./network.css";
export function Preparation({
  room,
  options,
  host,
  busy,
  prepare,
}: PreparationProps<GameOptions>) {
  const participating = room.you >= 0 && !room.spectating;
  const groups = teamGroups(options);
  const renderPlayer = (seat: number) => {
    const player = room.seats[seat];
    const editable =
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
          value={options.characters[seat]}
          disabled={busy || !editable || !player}
          onChange={(character) =>
            prepare({ type: "character", seat, character })
          }
        />
      </div>
    );
  };
  return (
    <section className="yuyu-preparation" aria-label="比赛准备">
      <div className="yuyu-room-rules">
        <ArenaPicker
          value={options.arena}
          disabled={busy || !host || !participating}
          onChange={(arena) => prepare({ type: "arena", arena })}
        />
        {room.capacity === 4 && (
          <label>
            赛制
            <select
              aria-label="赛制"
              value={
                options.oneVsThree
                  ? "solo"
                  : options.teamBattle
                    ? "team"
                    : "free"
              }
              disabled={busy || !host || !participating}
              onChange={(event) =>
                prepare({
                  type: "mode",
                  teamBattle: event.target.value !== "free",
                  oneVsThree: event.target.value === "solo",
                })
              }
            >
              <option value="free">四人混战</option>
              <option value="team">双人组队对战</option>
              <option value="solo">一对三</option>
            </select>
          </label>
        )}
      </div>
      {groups.length > 0 ? (
        <div className="yuyu-teams">
          {groups.map((group) => (
            <section
              className="yuyu-team"
              key={group.id}
              data-yuyu-team={group.id}
              aria-label={group.label}
            >
              <h4>{group.label}</h4>
              {group.seats.map(renderPlayer)}
            </section>
          ))}
        </div>
      ) : (
        <div className="yuyu-roster">
          {options.characters.map((_, seat) => renderPlayer(seat))}
        </div>
      )}
    </section>
  );
}
