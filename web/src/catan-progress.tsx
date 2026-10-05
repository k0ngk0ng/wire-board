import { useEffect, useState, useRef } from "react";
import type { CSSProperties } from "react";
import type { Act, Room, CatanState } from "./types";
import {
  catanProgressNames,
  catanProgressDescriptions,
} from "./catan-progress-names";
import {
  cityImprovementReason,
  cityTrackColors,
  cityTracks,
} from "./catan-city-state";
import { CatanResource, Bundle } from "./catan-resources";
import { catanSeatColor } from "./catan-player-colors";
import {
  commercialOfferAction,
  newProgressSelection,
  progressGain,
  progressMapMode,
  progressOpponents,
  progressPlayAction,
  progressTrack,
} from "./catan-progress-state";
import type { ProgressSelection } from "./catan-progress-state";
import "./catan-progress.css";

export function CatanProgressArt({
  card,
  assets,
}: {
  card: number;
  assets: string;
}) {
  return assets ? (
    <img
      className="catan-progress-art"
      src={`${assets}/catan/cities-knights/progress-art-${card}-v1.webp`}
      alt=""
    />
  ) : (
    <span className="catan-progress-symbol" aria-hidden="true">
      {["⚙", "⚓", "⚑"][progressTrack(card)]}
    </span>
  );
}
export function CatanProgressHand({
  room,
  act,
  busy,
  assets,
  selection,
  onChange,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  selection: ProgressSelection | null;
  onChange: (s: ProgressSelection | null) => void;
}) {
  const panel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (selection) panel.current?.scrollIntoView({ block: "nearest" });
  }, [selection?.card]);
  const g = room.game!.catan!,
    k = g.citiesKnights,
    p = room.you;
  if (!k || room.spectating || p < 0 || g.players[p]?.eliminated) return null;
  const cards = k.players[p].progress || [],
    publicCards = k.players[p].publicProgress || [];
  const s = selection,
    action = s ? progressPlayAction(room, s) : null;
  const update = (change: Partial<ProgressSelection>) => {
    if (s) onChange({ ...s, ...change });
  };
  const selectedMode = s ? progressMapMode(s.card) : "";
  return (
    <section className="catan-progress-panel" aria-label="你的进步牌">
      <details open={!s}>
        <summary>
          你的进步牌 · {cards.length} 张 <small>仅自己可见</small>
        </summary>
        <div className="catan-progress-hand">
          {[...new Set(cards)].map((card) => (
            <button
              key={card}
              disabled={busy}
              aria-pressed={s?.card === card}
              onClick={() =>
                onChange(s?.card === card ? null : newProgressSelection(card))
              }
              style={
                {
                  "--track-color": cityTrackColors[progressTrack(card)],
                } as CSSProperties
              }
            >
              <strong>{catanProgressNames[card]}</strong>
              <CatanProgressArt card={card} assets={assets} />
              <span>
                {cityTracks[progressTrack(card)]}
                {cards.filter((c) => c === card).length > 1 &&
                  ` ×${cards.filter((c) => c === card).length}`}
                <small>
                  {g.progressPlayable?.includes(card) ? "可使用" : "查看说明"}
                </small>
              </span>
            </button>
          ))}
        </div>
        {!cards.length && <p>尚未持有进步牌。</p>}
        <small>回合结束时最多保留4张；其他玩家回合超限需立即弃置。</small>
        {!!publicCards.length && (
          <p>
            已公开得分：
            {publicCards.map((c) => catanProgressNames[c]).join("、")}（共
            {k.players[p].progressPoints}分）
          </p>
        )}
      </details>
      {s && (
        <div
          ref={panel}
          className="catan-progress-use"
          aria-label={`使用${catanProgressNames[s.card]}`}
        >
          <header>
            <strong>{catanProgressNames[s.card]}</strong>
            <button
              onClick={() => onChange(null)}
              disabled={busy}
              aria-label="取消进步牌选择"
            >
              取消
            </button>
          </header>
          <p>{catanProgressDescriptions[s.card]}</p>
          {!g.progressPlayable?.includes(s.card) ? (
            <p>
              {s.card === 0
                ? "仅在自己的掷骰阶段可用。"
                : "仅在自己的行动阶段可用；征税还需要蛮族已经进攻。"}
            </p>
          ) : (
            <>
              {s.card === 0 && (
                <div className="catan-alchemy-dice">
                  {[0, 1].map((i) => (
                    <fieldset key={i} disabled={busy}>
                      <legend>{i === 0 ? "红骰" : "普通骰"}</legend>
                      {[1, 2, 3, 4, 5, 6].map((n) => (
                        <button
                          key={n}
                          aria-pressed={s.dice[i] === n}
                          onClick={() =>
                            update({
                              dice: s.dice.map((v, j) => (j === i ? n : v)),
                            })
                          }
                        >
                          {n}
                        </button>
                      ))}
                    </fieldset>
                  ))}
                  <small>
                    生产点数合计 {s.dice[0] + s.dice[1]} · 事件骰仍随机
                  </small>
                </div>
              )}
              {s.card === 1 && (
                <div className="catan-city-choice-cards">
                  {cityTracks.map((name, i) => (
                    <button
                      key={i}
                      title={cityImprovementReason(g, p, i, 1)}
                      disabled={busy || !!cityImprovementReason(g, p, i, 1)}
                      aria-pressed={!s.skip && s.color === i}
                      onClick={() => update({ color: i, skip: false })}
                    >
                      {name} {k.players[p].improvements[i]}→
                      {k.players[p].improvements[i] + 1}
                      <CatanResource
                        color={5 + i}
                        count={k.players[p].improvements[i]}
                        assets={assets}
                        small
                      />
                    </button>
                  ))}
                </div>
              )}
              {[13, 14, 15].includes(s.card) && (
                <div className="catan-city-choice-cards">
                  {Array.from({ length: 8 }, (_, i) => i)
                    .filter(
                      (i) =>
                        s.card === 13 ||
                        (s.card === 14 && i < 5) ||
                        (s.card === 15 && i >= 5),
                    )
                    .map((i) => (
                      <button
                        key={i}
                        disabled={busy}
                        aria-pressed={!s.skip && s.color === i}
                        onClick={() => update({ color: i, skip: false })}
                      >
                        <CatanResource color={i} assets={assets} small />
                      </button>
                    ))}
                </div>
              )}
              {[11, 18, 22].includes(s.card) && (
                <div className="catan-city-choice-cards">
                  {progressOpponents(g, p, s.card).map((i) => (
                    <button
                      key={i}
                      disabled={busy}
                      aria-pressed={!s.skip && s.target === i}
                      onClick={() => update({ target: i, skip: false })}
                    >
                      {room.seats[i].name}
                      <small>
                        {s.card === 18
                          ? `${k.players[i].progressCount}张进步牌`
                          : `${g.players[i].publicScore}分`}
                      </small>
                    </button>
                  ))}
                </div>
              )}
              {selectedMode && (
                <>
                  <p>
                    {s.card === 3
                      ? "在地图上选择两个亮起的数字地块。"
                      : s.card === 8
                        ? "按顺序选择一至两名亮起的骑士。再次点击已选骑士可撤回该步及后续选择。"
                        : `在地图上选择亮起的${selectedMode === "progress_tile" ? "地块" : selectedMode === "progress_edge" ? "道路" : "交点"}。`}
                  </p>
                  <p aria-live="polite">
                    {s.picks.length
                      ? `已选 ${s.picks.map((id) => `#${id + 1}`).join(" → ")}`
                      : "尚未选择目标"}
                  </p>
                  {!!s.picks.length && (
                    <button
                      className="subtle"
                      disabled={busy}
                      onClick={() => update({ picks: [] })}
                    >
                      清空目标
                    </button>
                  )}
                </>
              )}
              {s.card === 5 && (
                <>
                  <p>需支付：</p>
                  <Bundle values={[0, 0, 0, 1, 2]} assets={assets} />
                </>
              )}
              {[4, 6].includes(s.card) && (
                <p>
                  按当前地图和银行库存，可领取 {progressGain(g, p, s.card)} 张
                  {s.card === 4 ? "粮食" : "矿石"}。
                </p>
              )}
              {s.card !== 0 && (
                <label className="catan-progress-skip">
                  <input
                    type="checkbox"
                    checked={s.skip}
                    disabled={busy}
                    onChange={(e) => update({ skip: e.target.checked })}
                  />
                  消耗此牌并放弃收益
                </label>
              )}
              {s.skip && <p>此操作仍会失去这张进步牌，牌放回对应牌堆底部。</p>}
              <button
                className="primary wide"
                disabled={busy || !action}
                onClick={() => {
                  if (action) void act(action);
                }}
              >
                {s.skip ? "确认消耗并放弃" : "确认使用"}
              </button>
            </>
          )}
        </div>
      )}
    </section>
  );
}
export function CatanTradePowers({
  room,
  act,
  busy,
  assets,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
}) {
  const [harbor, setHarbor] = useState<number | null>(null),
    [target, setTarget] = useState<number | null>(null),
    [color, setColor] = useState<number | null>(null);
  const g = room.game!.catan!,
    k = g.citiesKnights,
    p = room.you,
    powers = k?.tradePowers;
  const signature = JSON.stringify(powers);
  useEffect(() => {
    setHarbor(null);
    setTarget(null);
    setColor(null);
  }, [room.id, room.game!.phase, signature]);
  if (
    !k ||
    room.spectating ||
    p < 0 ||
    g.players[p]?.eliminated ||
    powers?.player !== p
  )
    return null;
  const action = commercialOfferAction(room, harbor, target, color);
  const mine =
    room.status === "playing" &&
    !room.game!.finished &&
    room.game!.turn === p &&
    room.game!.phase === "catan_turn" &&
    !k.pending;
  return (
    <details
      className="catan-progress-panel"
      open
      aria-label="本行动的贸易能力"
    >
      <summary>本行动的贸易能力</summary>
      {!!powers.fleets.length && (
        <p>
          商船队2:1：
          {powers.fleets.map((i) => (
            <CatanResource key={i} color={i} assets={assets} small />
          ))}
        </p>
      )}
      {powers.harbors.map((targets, i) => (
        <div key={i} className="catan-harbor-offers">
          <strong>商业港 {i + 1}</strong>
          <div className="catan-city-choice-cards">
            {targets
              .filter((t) => !g.players[t].eliminated)
              .map((t) => (
                <button
                  key={t}
                  disabled={busy || !mine}
                  aria-pressed={harbor === i && target === t}
                  onClick={() => {
                    setHarbor(i);
                    setTarget(t);
                  }}
                >
                  {room.seats[t].name}
                </button>
              ))}
          </div>
          {!targets.length && <small>本张牌的交换机会已用完。</small>}
        </div>
      ))}
      {harbor !== null && target !== null && mine && (
        <>
          <p>
            给{room.seats[target].name}
            一张普通资源，换取对方自选的一张商品。对方没有商品时资源退回，本次机会仍消耗。
          </p>
          <div className="catan-city-choice-cards">
            {[0, 1, 2, 3, 4].map((i) => (
              <button
                key={i}
                disabled={busy || !g.players[p].resources?.[i]}
                aria-pressed={color === i}
                onClick={() => setColor(i)}
              >
                <CatanResource
                  color={i}
                  count={g.players[p].resources?.[i]}
                  assets={assets}
                  small
                />
              </button>
            ))}
          </div>
          <button
            className="primary wide"
            disabled={busy || !action}
            onClick={() => {
              if (action) void act(action);
            }}
          >
            确认发起交换
          </button>
        </>
      )}
    </details>
  );
}
export function CatanMerchant({
  game: g,
  assets,
}: {
  game: CatanState;
  assets: string;
}) {
  const m = g.citiesKnights?.merchant,
    t = m && g.tiles[m.tile];
  if (!m || !t) return null;
  return (
    <g transform={`translate(${t.x - 21},${t.y + 17})`} pointerEvents="none">
      <title>商人 · 玩家{m.owner + 1} · 持有获得1分</title>
      <ellipse
        cy="11"
        rx="12"
        ry="5"
        fill={catanSeatColor(g, m.owner)}
        stroke="#fff"
        strokeWidth="2"
      />
      {assets ? (
        <image
          href={`${assets}/catan/cities-knights/merchant-v1.webp`}
          x="-10"
          y="-15"
          width="20"
          height="26"
        />
      ) : (
        <text textAnchor="middle">商</text>
      )}
    </g>
  );
}
