import { useEffect } from "react";
import type { OptionsProps } from "../../../web/src/game-api/module";
import type { GameOptions } from "./types";
import { randomArena, resizeOptions } from "./labels";
import { ArenaPicker } from "./ArenaPicker";
export function OptionsForm({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<GameOptions>) {
  useEffect(() => {
    if (
      value.characters?.length !== capacity ||
      (capacity !== 4 && (value.teamBattle || value.oneVsThree))
    )
      onChange(resizeOptions(value, capacity));
  }, [capacity, value, onChange]);
  return (
    <div className="yuyu-options">
      <ArenaPicker
        value={value.arena ?? randomArena}
        disabled={disabled}
        onChange={(arena) => onChange({ ...value, arena })}
      />
      {capacity === 4 && (
        <label>
          赛制
          <select
            aria-label="赛制"
            value={
              value.oneVsThree ? "solo" : value.teamBattle ? "team" : "free"
            }
            disabled={disabled}
            onChange={(event) =>
              onChange({
                ...value,
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
  );
}
