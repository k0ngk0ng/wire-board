import { useEffect, useState } from "react";
import type { PreparationProps } from "../../../web/src/game-api/module";
import type { BasketballOptions } from "./module";
import "./preparation.css";

// Only runtime-verified courts and character slots are offered. Asset extraction
// coverage alone does not make an original court or team playable.
const courts = [
  { value: "court-0", id: 0, name: "球场 1" },
  { value: "new-york", id: 1, name: "纽约" },
  { value: "court-2", id: 2, name: "球场 3" },
  { value: "court-3", id: 3, name: "球场 4" },
  { value: "court-4", id: 4, name: "球场 5" },
] as const;
function SelectionImage({
  assets,
  file,
  alt,
  className,
}: {
  assets: string;
  file: string;
  alt: string;
  className?: string;
}) {
  const local = `/basketball/v1/selection/${file}`;
  const remote = assets ? `${assets.replace(/\/$/, "")}${local}` : local;
  const [source, setSource] = useState(remote);
  useEffect(() => setSource(remote), [remote]);
  return (
    <img
      className={className}
      src={source}
      alt={alt}
      draggable={false}
      onError={() => {
        if (source !== local) setSource(local);
      }}
    />
  );
}

export function Preparation({
  room,
  options,
  host,
  busy,
  assets,
  prepare,
}: PreparationProps<BasketballOptions>) {
  const seatActors =
    options.mode === "solo" ? [0] : options.mode === "versus" ? [0, 2] : [0, 1];
  const characters = options.characters ?? [0, 1, 0, 1];
  const teams = options.teams ?? [0, 1];
  const participating = room.you >= 0 && !room.spectating;
  return (
    <section className="basketball-preparation" aria-label="比赛准备">
      <div className="basketball-selection-heading">
        <h3>选择场地</h3>
        <span>{host && participating ? "由房主选择" : "等待房主选择"}</span>
      </div>
      <div className="basketball-court-choices">
        {courts.map((court) => (
          <button
            type="button"
            key={court.value}
            data-court={court.value}
            aria-pressed={options.court === court.value}
            disabled={busy || !host || !participating}
            onClick={() => {
              if (options.court !== court.value)
                prepare({ type: "court", court: court.value });
            }}
          >
            <SelectionImage
              assets={assets}
              file={`courts/court-${court.id}.png`}
              alt={`${court.name}场地预览`}
            />
            <span>
              {court.name}
              <span className="basketball-selection-state">
                {options.court === court.value ? "已选" : ""}
              </span>
            </span>
          </button>
        ))}
      </div>
      <div className="basketball-selection-heading">
        <h3>选择球员</h3>
        <span>选择自己的球员，房主可调整整队阵容</span>
      </div>
      <div className="basketball-roster">
        {[0, 1].map((side) => (
          <section
            className="basketball-team"
            key={side}
            aria-label={`${side === 0 ? "A" : "B"} 队阵容`}
          >
            <h4>{side === 0 ? "A" : "B"} 队</h4>
            {[0, 1].map((position) => {
              const actor = side * 2 + position;
              const seatIndex = seatActors.indexOf(actor);
              const seat = seatIndex >= 0 ? room.seats[seatIndex] : undefined;
              const human = !!seat?.id && !seat.bot && !seat.left;
              const owner = human
                ? seat.name
                : !seat && seatIndex >= 0
                  ? "等待玩家"
                  : "电脑";
              const own = human && seatIndex === room.you;
              const editable = participating && (own || host);
              const team = teams[side];
              const current = characters[actor];
              const teammate = actor ^ 1;
              const teammateSeat = room.seats[seatActors.indexOf(teammate)];
              const teammateHuman =
                !!teammateSeat?.id && !teammateSeat.bot && !teammateSeat.left;
              return (
                <div
                  className="basketball-roster-player"
                  data-actor={actor}
                  key={actor}
                >
                  <div className="basketball-player-owner">
                    <strong>
                      {owner}
                      {own ? "（你）" : ""}
                    </strong>
                    <span>{position === 0 ? "一号位" : "二号位"}</span>
                  </div>
                  <div className="basketball-player-selection">
                    <SelectionImage
                      className="basketball-selected-body"
                      assets={assets}
                      file={`players/team-${team}-character-${current}-body.png`}
                      alt={`${owner}当前球员 ${current + 1}`}
                    />
                    <div
                      className="basketball-character-choices"
                      role="group"
                      aria-label={`${side === 0 ? "A" : "B"} 队${position + 1}号位球员`}
                    >
                      {[0, 1].map((character) => (
                        <button
                          type="button"
                          key={character}
                          data-character={character}
                          aria-label={`球员 ${character + 1}`}
                          aria-pressed={current === character}
                          disabled={
                            busy ||
                            !editable ||
                            (!host &&
                              teammateHuman &&
                              characters[teammate] === character)
                          }
                          title={
                            !host &&
                            teammateHuman &&
                            characters[teammate] === character
                              ? "已由队友选择"
                              : undefined
                          }
                          onClick={() => {
                            if (current !== character)
                              prepare({ type: "character", actor, character });
                          }}
                        >
                          <SelectionImage
                            assets={assets}
                            file={`players/team-${team}-character-${character}-portrait.png`}
                            alt=""
                          />
                          <span>球员 {character + 1}</span>
                          <span className="basketball-selection-state">
                            {current === character ? "已选" : ""}
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              );
            })}
          </section>
        ))}
      </div>
    </section>
  );
}
