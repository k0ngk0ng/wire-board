import { CatanProgressArt } from "./catan-progress";
import { CatanProgressChoice } from "./catan-progress-choice";
import { progressChoiceLabels } from "./catan-progress-choice-state";
import { catanProgressNames } from "./catan-progress-names";
import { useEffect, useState } from "react";
import type { CSSProperties } from "react";
import { Castle, Shield, Ship } from "lucide-react";
import type { Act, CatanState, Room } from "./types";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import { CatanResource, Bundle } from "./catan-resources";
import {
  cityActionCosts,
  cityAt,
  cityActionNames,
  cityDefense,
  cityDiscardLimit,
  cityImprovementReason,
  cityTrackColors,
  cityTrackKeys,
  cityTrackPowers,
  cityTracks,
  cityWallSites,
} from "./catan-city-state";
import "./catan-city.css";

export function CatanCitySeat({
  game,
  seat,
}: {
  game: CatanState;
  seat: number;
}) {
  const k = game.citiesKnights;
  if (!k) return null;
  const p = k.players[seat];
  return (
    <span className="catan-city-seat">
      <span>
        最长道路 {game.players[seat].roadLength} · 骑士防御{" "}
        {cityDefense(game, seat)} · 城墙{" "}
        {(cityDiscardLimit(game, seat) - 7) / 2}
      </span>
      <span>
        {p.improvements.map((n, i) => (
          <b
            key={i}
            style={{ color: cityTrackColors[i] }}
            title={cityTrackPowers[i]}
          >
            {cityTracks[i]} {n}
          </b>
        ))}
      </span>
      {k.merchant?.owner === seat && <span>商人 +1 分</span>}
      {!!(p.defenderPoints + p.progressPoints) && (
        <span>
          防御者 {p.defenderPoints} 分 · 公开进步牌 {p.progressPoints} 分
        </span>
      )}
    </span>
  );
}

export function CatanCityPiece({
  game,
  vertex,
  assets,
}: {
  game: CatanState;
  vertex: number;
  assets: string;
}) {
  const k = game.citiesKnights;
  if (!k) return null;
  const n = k.knights.find((n) => n.vertex === vertex);
  const metro = k.metropolises.indexOf(vertex);
  if (!assets)
    return (
      <g pointerEvents="none">
        {n && (
          <>
            <path
              d="M-10 8V-13L0-21L10-13V8Z"
              fill={catanSeatColor(game, n.owner)}
              stroke={n.active ? "#f7cd55" : "#674f2b"}
              strokeWidth="3"
            />
            <text y="4" textAnchor="middle" className="catan-knight-status">
              {n.strength}
              {n.active ? "✓" : "○"}
            </text>
          </>
        )}
        {metro >= 0 && (
          <>
            <path
              d="M-20 6V-22H-10V-13H10V-22H20V6H10V-4H-10V6Z"
              fill={cityTrackColors[metro]}
              stroke="#f8dda0"
              strokeWidth="2"
            />
            <title>{cityTracks[metro]}大都会</title>
          </>
        )}
        {k.fallenCities.includes(vertex) && (
          <text y="21" textAnchor="middle" className="catan-fallen-label">
            待修复
          </text>
        )}
      </g>
    );
  if (n)
    return (
      <g
        className={`catan-city-knight ${n.active ? "active" : "inactive"}`}
        pointerEvents="none"
      >
        <title>
          {n.strength}级骑士 · {n.active ? "已激活" : "未激活"}
        </title>
        <ellipse
          cy="9"
          rx="16"
          ry="8"
          fill={n.active ? "#f7cd55" : "#ede4d1"}
          stroke={catanSeatColor(game, n.owner)}
          strokeWidth="2"
        />
        <image
          href={`${assets}/catan/cities-knights/knight-${catanPieceColors[catanColorIndex(game, n.owner)]}-${n.strength}-v1.webp`}
          x="-14"
          y="-23"
          width="28"
          height="33"
        />
        <text y="17" textAnchor="middle" className="catan-knight-status">
          {n.strength}
          {n.active ? " ✓" : " ○"}
        </text>
      </g>
    );
  return (
    <g pointerEvents="none">
      {metro >= 0 && (
        <>
          <title>{cityTracks[metro]}大都会 · 额外2分</title>
          <image
            href={`${assets}/catan/cities-knights/metropolis-${cityTrackKeys[metro]}-v1.webp`}
            x="-18"
            y="-34"
            width="36"
            height="37"
          />
        </>
      )}
      {k.fallenCities.includes(vertex) && (
        <>
          <title>横置城市 · 按村庄生产，必须优先修复</title>
          <text y="21" textAnchor="middle" className="catan-fallen-label">
            待修复
          </text>
        </>
      )}
    </g>
  );
}

export function CatanCityWall({
  game,
  vertex,
  assets,
}: {
  game: CatanState;
  vertex: number;
  assets: string;
}) {
  if (!game.citiesKnights?.walls.includes(vertex)) return null;
  if (!assets)
    return (
      <rect
        pointerEvents="none"
        x="-23"
        y="5"
        width="46"
        height="10"
        rx="2"
        fill={catanSeatColor(game, game.vertices[vertex].owner)}
        stroke="#473322"
      />
    );
  return (
    <image
      pointerEvents="none"
      href={`${assets}/catan/cities-knights/wall-${catanPieceColors[catanColorIndex(game, game.vertices[vertex].owner)]}-v1.webp`}
      x="-24"
      y="1"
      width="48"
      height="21"
    >
      <title>城墙 · 弃牌上限增加2张</title>
    </image>
  );
}

export function CatanCityOverview({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const g = room.game!.catan!,
    k = g.citiesKnights;
  if (!k) return null;
  const strength = g.vertices.filter(
    (v) => cityAt(g, v.id) && !g.players[v.owner]?.eliminated,
  ).length;
  const attack = k.barbarianPosition >= 7;
  return (
    <section className="catan-city-overview" aria-label="蛮族与进步牌">
      <header>
        <strong>
          <Ship size={16} /> 蛮族船 {k.barbarianPosition}/7
        </strong>
        <span>已进攻 {k.invasions} 次</span>
      </header>
      <div
        className="catan-barbarian-track"
        role="progressbar"
        aria-label="蛮族船前进"
        aria-valuemin={0}
        aria-valuemax={7}
        aria-valuenow={k.barbarianPosition}
      >
        {Array.from({ length: 8 }, (_, i) => (
          <span key={i} className={i <= k.barbarianPosition ? "reached" : ""}>
            {i === 7 ? <Castle size={16} /> : i}
          </span>
        ))}
        <span
          className="catan-barbarian-ship"
          data-city-ship
          style={{ transform: `translateX(${k.barbarianPosition * 100}%)` }}
        >
          {assets ? (
            <img
              src={`${assets}/catan/cities-knights/barbarian-ship-v1.webp`}
              alt="蛮族船"
            />
          ) : (
            <Ship size={20} />
          )}
        </span>
      </div>
      <p>
        <Shield size={15} /> 防御 {cityDefense(g)}{" "}
        <span>／ 蛮族 {strength}</span>{" "}
        {attack
          ? "· 正在结算"
          : cityDefense(g) >= strength
            ? "· 目前可抵御"
            : "· 防御不足"}
      </p>
      {k.invasions === 0 && <small>首次进攻前强盗休眠，掷出7仍需弃牌。</small>}
      <div className="catan-progress-stocks">
        {cityTracks.map((name, i) => (
          <span
            key={name}
            data-progress-deck={i}
            style={{ color: cityTrackColors[i] }}
          >
            {assets && (
              <img
                src={`${assets}/catan/cities-knights/progress-back-${cityTrackKeys[i]}-v1.webp`}
                alt=""
              />
            )}
            {name} <b>{k.progressRemaining[i]}</b>
          </span>
        ))}
      </div>
    </section>
  );
}

export function CatanCityActions({
  room,
  act,
  busy,
  assets,
  mode,
  selectMode,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  mode: string;
  selectMode: (mode: string) => void;
}) {
  const g = room.game!.catan!,
    k = g.citiesKnights,
    you = room.you;
  const [track, setTrack] = useState<number | null>(null);
  useEffect(() => setTrack(null), [room.id, room.game!.turn, room.game!.phase]);
  if (!k || room.spectating || you < 0 || g.players[you]?.eliminated)
    return null;
  const mine =
    room.status === "playing" &&
    !room.game!.finished &&
    room.game!.turn === you &&
    room.game!.phase === "catan_turn" &&
    !k.pending;
  const hand = g.players[you].resources || [];
  const p = k.players[you];
  const actions: [string, number][] = [
    ["wall", cityWallSites(g, you).length],
    ["knight_recruit", g.legal.knightRecruit?.length || 0],
    ["knight_activate", g.legal.knightActivate?.length || 0],
    ["knight_promote", g.legal.knightPromote?.length || 0],
    [
      "knight_move",
      Object.values(g.knightMoves || {}).filter((a) => a.length).length,
    ],
    ["knight_chase", g.legal.knightChase?.length || 0],
    ...(g.seafarers && !g.seafarers.wonders
      ? [
          ["knight_chase_pirate", g.legal.knightChasePirate?.length || 0] as [
            string,
            number,
          ],
        ]
      : []),
  ];
  return (
    <section className="catan-city-actions" aria-label="城市建设与骑士">
      <h3>
        城市建设 <small>弃牌上限 {cityDiscardLimit(g, you)} 张</small>
      </h3>
      <div className="catan-city-tracks">
        {p.improvements.map((level, i) => {
          const reason = cityImprovementReason(g, you, i);
          return (
            <button
              key={i}
              className={track === i ? "selected" : ""}
              disabled={!mine || busy || !!reason}
              title={reason || cityTrackPowers[i]}
              onClick={() => {
                setTrack(i);
                selectMode("");
              }}
              style={{ "--track-color": cityTrackColors[i] } as CSSProperties}
            >
              <strong>
                {cityTracks[i]} <b>{level}/5</b>
              </strong>
              <span
                className="catan-city-levels"
                aria-label={`${cityTracks[i]}${level}级`}
              >
                {[1, 2, 3, 4, 5].map((n) => (
                  <i key={n} className={n <= level ? "built" : ""} />
                ))}
              </span>
              <small>{level >= 5 ? "已满级" : `升至${level + 1}级`}</small>
              <CatanResource
                color={5 + i}
                count={level >= 5 ? 0 : level + 1}
                assets={assets}
                small
              />
            </button>
          );
        })}
      </div>
      {mine && track !== null && (
        <div className="catan-city-upgrade-confirm">
          <strong>
            {cityTracks[track]}：{p.improvements[track]} →{" "}
            {p.improvements[track] + 1}级
          </strong>
          <p>{cityTrackPowers[track]}</p>
          <small>
            {cityImprovementReason(g, you, track) ||
              `支付${p.improvements[track] + 1}张${["纸张", "布料", "钱币"][track]}`}
          </small>
          <div>
            <button disabled={busy} onClick={() => setTrack(null)}>
              取消
            </button>
            <button
              className="primary"
              disabled={busy || !!cityImprovementReason(g, you, track)}
              onClick={async () => {
                await act({ type: "catan_improvement", color: track });
                setTrack(null);
              }}
            >
              确认建设
            </button>
          </div>
        </div>
      )}
      {mine && (
        <div className="catan-build-menu catan-city-build">
          {actions.map(([key, count]) => (
            <button
              key={key}
              className={mode === key ? "selected" : ""}
              disabled={
                busy ||
                !count ||
                (cityActionCosts[key] || []).some((n, c) => n > hand[c])
              }
              onClick={() => {
                setTrack(null);
                selectMode(key);
              }}
            >
              <span>{cityActionNames[key]}</span>
              {cityActionCosts[key] ? (
                <Bundle values={cityActionCosts[key]} assets={assets} />
              ) : (
                <small>
                  {key === "knight_move"
                    ? "选择骑士，再选目的地"
                    : "消耗激活状态"}
                </small>
              )}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

export function CatanCityChoice({
  room,
  act,
  busy,
  assets,
  chosen,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  chosen: { type: string; id: number } | null;
}) {
  const g = room.game!.catan!,
    k = g.citiesKnights,
    q = k?.pending;
  const [color, setColor] = useState<number | null>(null);
  const [cards, setCards] = useState<number[]>([]);
  useEffect(() => {
    setColor(null);
    setCards([]);
  }, [room.id, q?.kind, q?.players[0], q?.track, q?.target, room.game!.phase]);
  if (!q || !k) return null;
  const mine =
    !room.spectating &&
    room.status === "playing" &&
    !room.game!.finished &&
    room.you >= 0 &&
    !g.players[room.you]?.eliminated &&
    q.players[0] === room.you;
  const labels: Record<string, string> = {
    ...progressChoiceLabels,
    aqueduct: "引水渠补偿",
    metropolis: "大都会选址",
    pillage: "蛮族劫掠",
    defender_reward: "防御者奖励",
    progress_discard: "进步牌超限",
    knight_retreat: "骑士退让",
  };
  const hand = k.players[room.you]?.progress || [],
    due = Math.max(0, hand.length - 4);
  return (
    <details
      className="catan-city-choice"
      open
      key={`${q.kind}-${q.players[0]}-${q.track}`}
    >
      <summary>
        {labels[q.kind] || "进步牌选择"} ·{" "}
        {mine ? "轮到你回应" : `等待${room.seats[q.players[0]]?.name}`}
      </summary>
      {mine ? (
        <>
          {q.kind === "aqueduct" && (
            <>
              <p>本次未获生产，自选一张银行现有的普通资源。</p>
              <div className="catan-city-choice-cards">
                {g.bank.slice(0, 5).map((n, i) => (
                  <button
                    key={i}
                    className={color === i ? "selected" : ""}
                    disabled={busy || !n}
                    onClick={() => setColor(i)}
                  >
                    <CatanResource color={i} assets={assets} count={n} small />
                  </button>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || color === null || !g.bank[color]}
                onClick={() => void act({ type: "catan_aqueduct", color })}
              >
                确认领取
              </button>
            </>
          )}
          {q.kind === "metropolis" && (
            <p>
              在地图上选择亮起的城市，安放{cityTracks[q.track]}大都会，再确认。
            </p>
          )}
          {q.kind === "pillage" && (
            <p>选择地图上亮起的一座城市降级为村庄；该处城墙也会归还。</p>
          )}
          {q.kind === "knight_retreat" && (
            <p>
              为被驱逐的{q.knight?.strength}
              级骑士选择亮起的空交点，再确认。保留原激活状态。
            </p>
          )}
          {q.kind === "defender_reward" && (
            <>
              <p>防御贡献并列最高，请选择一类进步牌。</p>
              <div className="catan-city-choice-cards">
                {cityTracks.map((name, i) => (
                  <button
                    key={i}
                    disabled={busy || !k.progressRemaining[i]}
                    className={color === i ? "selected" : ""}
                    onClick={() => setColor(i)}
                  >
                    {assets && (
                      <img
                        src={`${assets}/catan/cities-knights/progress-back-${cityTrackKeys[i]}-v1.webp`}
                        alt=""
                      />
                    )}
                    {name} · {k.progressRemaining[i]}
                  </button>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || color === null || !k.progressRemaining[color]}
                onClick={() =>
                  void act({ type: "catan_defender_reward", color })
                }
              >
                确认抽取
              </button>
            </>
          )}
          {q.kind === "progress_discard" && (
            <>
              <p>请选择 {due} 张进步牌放回牌堆底部。卡名不会公开。</p>
              <div className="catan-city-choice-cards">
                {hand.map((card, i) => (
                  <button
                    key={i}
                    disabled={
                      busy || (!cards.includes(i) && cards.length >= due)
                    }
                    aria-pressed={cards.includes(i)}
                    onClick={() =>
                      setCards(
                        cards.includes(i)
                          ? cards.filter((n) => n !== i)
                          : [...cards, i],
                      )
                    }
                  >
                    <CatanProgressArt card={card} assets={assets} />
                    {catanProgressNames[card] ||
                      g.progressRules?.find((r) => r.id === card)?.name ||
                      `进步牌 ${card + 1}`}
                  </button>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || cards.length !== due}
                onClick={() =>
                  void act({
                    type: "catan_progress_discard",
                    cards: cards.map((i) => hand[i]),
                  })
                }
              >
                确认弃置 {cards.length} 张
              </button>
            </>
          )}
          {progressChoiceLabels[q.kind] && (
            <CatanProgressChoice
              key={`${room.id}:${q.kind}:${q.players[0]}:${q.target}:${room.turnDeadline}`}
              room={room}
              act={act}
              busy={busy}
              assets={assets}
              chosen={chosen}
            />
          )}
          <small>回应限时120秒；可收起本面板查看地图，超时由系统代选。</small>
        </>
      ) : (
        <p>该玩家正在选择，完成后恢复原行动倒计时。</p>
      )}
    </details>
  );
}
