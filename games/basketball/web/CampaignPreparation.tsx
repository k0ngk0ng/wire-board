import { useState } from "react";
import type { PreparationProps } from "../../../web/src/game-api/module";
import type { BasketballOptions } from "./module";
import type { CampaignProgress } from "./campaign-types";
import labels from "../selection/labels.json";
import campaignLabels from "../selection/campaign-labels.json";
import { SelectionImage } from "./SelectionImage";
import "./campaign.css";

export const campaignItemName = (id: number) =>
  campaignLabels.items[id]?.name ?? "未装备";
const slots = ["上衣", "短裤", "鞋子"];
type EditorProps = {
  progress: CampaignProgress;
  teammates: [number, number];
  tactic: number;
  opponent?: number;
  opponents?: number[];
  assets: string;
  busy: boolean;
  canEditTeam: boolean;
  canChooseTactic: boolean;
  canEditCharacter: (position: number, character: number) => boolean;
  chooseCharacter: (position: number, character: number) => void;
  send: (action: Record<string, unknown>) => void;
  snapshotKey: string;
  revision: number;
};
export function CampaignPreparationEditor(p: EditorProps) {
  const [target, setTarget] = useState(0);
  const home = !!(p.progress.period & 2);
  const full = p.progress.inventory.filter((i) => i >= 0).length === 16;
  return (
    <section className="basketball-campaign-editor" aria-label="战役准备">
      <p className="basketball-campaign-progress">
        已胜 {p.progress.period / 2} / 14 场 · {home ? "主场" : "客场"}
      </p>
      {p.opponent !== undefined && (
        <section aria-label="选择对手">
          <h3>选择对手</h3>
          <div className="basketball-campaign-opponents">
            {p.opponents?.map((opponent) => (
              <button
                type="button"
                key={opponent}
                data-opponent={opponent}
                aria-pressed={p.opponent === opponent}
                disabled={p.busy || !p.canEditTeam}
                onClick={() => {
                  if (opponent !== p.opponent)
                    p.send({ type: "opponent", opponent });
                }}
              >
                <SelectionImage
                  assets={p.assets}
                  file={`courts/court-${home ? 0 : opponent}.png`}
                  alt={`${labels.teams[opponent].name} · ${labels.courts[home ? 0 : opponent]}球场`}
                />
                <span>
                  {labels.teams[opponent].name}
                  {p.opponent === opponent ? " · 已选" : ""}
                </span>
              </button>
            ))}
          </div>
        </section>
      )}
      <section aria-label="热血队阵容">
        <h3>热血队阵容</h3>
        <div className="basketball-campaign-lineup">
          {p.teammates.map((current, position) => (
            <div data-campaign-position={position} key={position}>
              <h4>
                {position === 0 ? "一号位" : "二号位"} ·{" "}
                {labels.teams[0].characters[current]}
              </h4>
              <div className="basketball-campaign-characters">
                {labels.teams[0].characters.map((name, character) => (
                  <button
                    type="button"
                    key={character}
                    data-character={character}
                    aria-label={name}
                    aria-pressed={character === current}
                    disabled={
                      p.busy || !p.canEditCharacter(position, character)
                    }
                    onClick={() => {
                      if (character !== current)
                        p.chooseCharacter(position, character);
                    }}
                  >
                    <SelectionImage
                      assets={p.assets}
                      file={`players/team-0-character-${character}-portrait.png`}
                      alt=""
                    />
                    <span>{name}</span>
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>
      <section aria-label="共享装备">
        <h3>共享装备</h3>
        <label>
          装备给哪位球员
          <select
            aria-label="装备给哪位球员"
            value={target}
            disabled={p.busy || !p.canEditTeam}
            onChange={(e) => setTarget(Number(e.target.value))}
          >
            {labels.teams[0].characters.map((name, character) => (
              <option key={character} value={character}>
                {name}
              </option>
            ))}
          </select>
        </label>
        <div className="basketball-campaign-inventory">
          {p.progress.inventory.map((item, index) =>
            item < 0 ? null : (
              <button
                type="button"
                key={`${p.snapshotKey}:${index}:${item}`}
                data-inventory-index={index}
                data-revision={p.revision}
                title={campaignLabels.items[item]?.originalName}
                disabled={p.busy || !p.canEditTeam}
                onClick={() =>
                  p.send({
                    type: "equip",
                    character: target,
                    inventoryIndex: index,
                  })
                }
              >
                <strong>{campaignItemName(item)}</strong>
                <span>装备给{labels.teams[0].characters[target]}</span>
              </button>
            ),
          )}
        </div>
        {!p.progress.inventory.some((i) => i >= 0) && (
          <p>库存暂无装备。获胜后可获得奖励。</p>
        )}
        {full && (
          <p className="basketball-campaign-notice">
            库存已满，直接卸下会丢弃该装备；从库存换装可以保留原装备。
          </p>
        )}
        <div className="basketball-campaign-equipment">
          {p.progress.equipment.map((equipment, character) => (
            <div data-equipment-character={character} key={character}>
              <h4>{labels.teams[0].characters[character]}</h4>
              {!equipment.some((item) => item >= 0) && <p>未装备</p>}
              {equipment.map((item, slot) =>
                item < 0 ? null : (
                  <div
                    className="basketball-campaign-slot"
                    key={`${p.snapshotKey}:${slot}:${item}`}
                  >
                    <span>
                      {slots[slot]}：{campaignItemName(item)}
                    </span>
                    {item >= 0 && (
                      <button
                        type="button"
                        disabled={p.busy || !p.canEditTeam}
                        aria-label={`${full ? "丢弃" : "卸下"}${labels.teams[0].characters[character]}的${campaignItemName(item)}`}
                        onClick={() =>
                          p.send({ type: "unequip", character, slot })
                        }
                      >
                        {full ? "卸下并丢弃" : "卸下"}
                      </button>
                    )}
                  </div>
                ),
              )}
            </div>
          ))}
        </div>
      </section>
      <label>
        队友战术
        <select
          aria-label="队友战术"
          value={p.tactic}
          disabled={p.busy || !p.canEditTeam || !p.canChooseTactic}
          onChange={(e) =>
            p.send({ type: "tactic", tactic: Number(e.target.value) })
          }
        >
          {campaignLabels.tactics.map((t) => (
            <option key={t.profile} value={t.profile}>
              {t.name}
            </option>
          ))}
        </select>
      </label>
      {!p.canChooseTactic && <p>一名真人与一名电脑同队时可调整队友战术。</p>}
    </section>
  );
}

export function CampaignWaitingPreparation({
  room,
  options,
  host,
  busy,
  assets,
  prepare,
}: PreparationProps<BasketballOptions>) {
  const c = options.campaign!;
  if (!c.preparation) return <p>正在加载战役准备…</p>;
  const participating = room.you >= 0 && !room.spectating;
  const characters = options.characters ?? [0, 1, 0, 1];
  const teammates: [number, number] = [characters[0], characters[1]];
  const oneHuman =
    options.mode === "solo" ||
    (room.seats.length === 2 && !!room.seats[0].bot !== !!room.seats[1].bot);
  return (
    <CampaignPreparationEditor
      progress={c.preparation}
      teammates={teammates}
      tactic={c.tactic ?? 4}
      opponent={c.opponent}
      opponents={c.availableOpponents}
      assets={assets}
      busy={busy}
      canEditTeam={participating && host}
      canChooseTactic={oneHuman}
      revision={room.version}
      snapshotKey={`${room.id}:${room.version}`}
      canEditCharacter={(position, character) =>
        participating &&
        (host ||
          (room.you === position &&
            !(
              teammates[position ^ 1] === character &&
              room.seats[position ^ 1] &&
              !room.seats[position ^ 1].bot
            )))
      }
      chooseCharacter={(actor, character) =>
        prepare({ type: "character", actor, character })
      }
      send={prepare}
    />
  );
}
