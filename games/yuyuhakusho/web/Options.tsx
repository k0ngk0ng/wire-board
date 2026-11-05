import { useEffect } from "react";
import type { OptionsProps } from "../../../web/src/game-api/module";
import type { GameOptions } from "./types";
import { arenas, defaults } from "./labels";
export function OptionsForm({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<GameOptions>) {
  useEffect(() => {
    if (
      value.characters?.length !== capacity ||
      (capacity !== 4 && value.teamBattle)
    )
      onChange({
        ...value,
        teamBattle: capacity === 4 && !!value.teamBattle,
        characters: defaults(capacity),
      });
  }, [capacity, value, onChange]);
  return (
    <div className="yuyu-options">
      <label>
        场地
        <select
          aria-label="场地"
          value={value.arena ?? 0}
          disabled={disabled}
          onChange={(event) =>
            onChange({ ...value, arena: Number(event.target.value) })
          }
        >
          {arenas.map((arena) => (
            <option key={arena.id} value={arena.id}>
              {arena.name}
            </option>
          ))}
        </select>
      </label>
      {capacity === 4 && (
        <label>
          赛制
          <select
            aria-label="赛制"
            value={value.teamBattle ? "team" : "free"}
            disabled={disabled}
            onChange={(event) =>
              onChange({ ...value, teamBattle: event.target.value === "team" })
            }
          >
            <option value="free">四人混战</option>
            <option value="team">双人组队对战</option>
          </select>
        </label>
      )}
    </div>
  );
}
