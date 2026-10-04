import { useEffect, useState } from "react";
import type { CSSProperties } from "react";
import {
  ArrowLeftRight,
  Home,
  Castle,
  Route,
  ScrollText,
  Dices,
  Minus,
  Plus,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  Shield,
  Flag,
  Ship,
  Move,
} from "lucide-react";
import type { Act, Room } from "./types";
import { PlayerName } from "./profiles";
import { useRailMapControls } from "./rail-map-controls";
import "./catan.css";
import "./catan-gold.css";
import "./catan-seafarers.css";
import { CatanDesertRegions, CatanPirate, CatanShip } from "./catan-seafarers";
import { CatanHelpers } from "./catan-helpers";
import {
  CatanTribePortChoice,
  CatanTribeRewards,
  CatanTribeStock,
} from "./catan-tribe";
export const catanNames = ["木材", "砖块", "羊毛", "粮食", "矿石"];
export const catanColors = [
  "#286540",
  "#bb633c",
  "#8cac48",
  "#dbad43",
  "#7a8390",
  "#dfca95",
  "#5db3d2",
  "#d6b05b",
  "#eaf3f4",
];
export const catanPlayerColors = [
  "#3078be",
  "#c94737",
  "#f5eee1",
  "#e5902e",
  "#794287",
  "#388146",
];
const resourceKeys = ["wood", "brick", "wool", "grain", "ore"];
const devNames = ["骑士", "道路建设", "丰收", "垄断", "胜利点"];
const devDescriptions = [
  "移动强盗并随机偷取一张资源；累计三名骑士可争夺最大骑士军队。",
  "免费修建两条道路，仍需符合连接与棋子数量限制。",
  "从银行领取两张资源，可选择相同种类。",
  "指定一种资源，其他玩家交出该种类的全部资源。",
  "自动计入你的私人分数，达到十点时在自己的回合获胜。",
];
const costs: Record<string, number[]> = {
  road: [1, 1, 0, 0, 0],
  ship: [1, 0, 1, 0, 0],
  settlement: [1, 1, 1, 1, 0],
  city: [0, 0, 0, 2, 3],
  buy_dev: [0, 0, 1, 1, 1],
};
const total = (a: number[]) => a.reduce((n, x) => n + x, 0);
export const catanPhases: Record<string, string> = {
  catan_setup_settlement: "选择起始村庄的位置",
  catan_setup_road: "在刚放置的村庄旁修路",
  catan_roll: "掷骰，生产资源",
  catan_turn: "交易、建造，或结束回合",
  catan_discard: "同时选择要弃置的资源",
  catan_robber: "选择强盗的新位置",
  catan_steal: "选择偷取资源的对手",
  catan_roads: "放置免费的道路",
  catan_helper: "等待助手选择",
  catan_gold: "选择金矿出产的资源",
  catan_port: "安放领取的港口",
};
export const catanSeafarerPhases: Record<string, string> = {
  catan_setup_road: "在刚放置的村庄旁修路或造船",
  catan_roads: "放置免费的道路或船只",
  catan_robber: "选择移动强盗或海盗",
};
export function CatanResource({
  color,
  count,
  assets = "",
  small = false,
}: {
  color: number;
  count?: number;
  assets?: string;
  small?: boolean;
}) {
  return (
    <span
      className={`catan-resource ${small ? "small" : ""}`}
      style={{ "--resource-color": catanColors[color] } as CSSProperties}
    >
      <span className="catan-resource-art">
        {assets ? (
          <img
            src={`${assets}/catan/${small ? "icon" : "resource"}-${resourceKeys[color]}-v1.webp`}
            alt=""
          />
        ) : (
          <span>{["♣", "▰", "♧", "❧", "◆"][color]}</span>
        )}
      </span>
      <span>{catanNames[color]}</span>
      {count !== undefined && <b>{count}</b>}
    </span>
  );
}
function Bundle({
  values,
  assets,
  showZero = false,
}: {
  values: number[];
  assets: string;
  showZero?: boolean;
}) {
  return (
    <div className="catan-bundle">
      {values.map(
        (n, c) =>
          (n > 0 || showZero) && (
            <CatanResource key={c} color={c} count={n} assets={assets} small />
          ),
      )}
    </div>
  );
}
function ResourcePicker({
  label,
  values,
  onChange,
  limits,
  assets,
  disabled,
}: {
  label: string;
  values: number[];
  onChange: (n: number[]) => void;
  limits: number[];
  assets: string;
  disabled: boolean;
}) {
  return (
    <fieldset className="catan-picker" disabled={disabled}>
      <legend>
        {label} <b>{total(values)}</b>
      </legend>
      {values.map((n, c) => (
        <div key={c}>
          <CatanResource color={c} assets={assets} small />
          <div className="catan-stepper">
            <button
              type="button"
              aria-label={`${label}减少${catanNames[c]}`}
              disabled={n === 0}
              onClick={() =>
                onChange(values.map((x, i) => (i === c ? x - 1 : x)))
              }
            >
              <Minus size={13} />
            </button>
            <output>{n}</output>
            <button
              type="button"
              aria-label={`${label}增加${catanNames[c]}`}
              disabled={n >= limits[c]}
              onClick={() =>
                onChange(values.map((x, i) => (i === c ? x + 1 : x)))
              }
            >
              <Plus size={13} />
            </button>
          </div>
        </div>
      ))}
    </fieldset>
  );
}
function CatanGoldChoice({
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
  const g = room.game!.catan!,
    claim = g.goldPending?.claims[0];
  const [take, setTake] = useState([0, 0, 0, 0, 0]);
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => {
    setTake([0, 0, 0, 0, 0]);
    setCollapsed(false);
  }, [room.id, claim?.player, g.rollId, g.setupStep]);
  if (
    !claim ||
    room.status !== "playing" ||
    room.game!.finished ||
    room.game!.phase !== "catan_gold"
  )
    return null;
  const mine =
    !room.spectating &&
    claim.player === room.you &&
    !g.players[room.you]?.eliminated;
  const due = Math.min(claim.count, total(g.bank)),
    picked = total(take);
  return (
    <section className="catan-gold-choice" aria-label="金矿资源选择">
      <header>
        <strong>
          {mine
            ? `金矿：选择 ${due} 张资源`
            : `${room.seats[claim.player].name} 正在选择金矿资源`}
        </strong>
        <button
          type="button"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          <p>可以选择同种或不同资源。每人限时 120 秒，超时自动选择。</p>
          {mine ? (
            <>
              <ResourcePicker
                label="金矿领取"
                values={take}
                onChange={setTake}
                limits={g.bank.map((n, i) =>
                  Math.min(n, take[i] + Math.max(0, due - picked)),
                )}
                assets={assets}
                disabled={busy}
              />
              <div className="catan-gold-stock">
                银行库存：
                {g.bank.map((n, i) => (
                  <span key={i}>
                    {catanNames[i]} {n}
                  </span>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={
                  busy || picked !== due || take.some((n, i) => n > g.bank[i])
                }
                onClick={() => void act({ type: "catan_gold", take })}
              >
                确认领取 {picked} / {due}
              </button>
            </>
          ) : (
            <p>选择完成后继续当前回合。</p>
          )}
        </div>
      )}
    </section>
  );
}
export function CatanBoard({
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
  const game = room.game!,
    g = game.catan!,
    you = room.you,
    p = g.players[you],
    hand = p?.resources || [0, 0, 0, 0, 0];
  const hexSize = g.hexSize || 62;
  const sea = g.seafarers;
  const pieceScale = sea ? Math.max(0.64, hexSize / 62) : 1;
  const targetScore = sea?.victoryPoints || 10;
  const terrainNames = [...catanNames, "沙漠", "海洋", "金矿", "未探索迷雾"];
  const describeDev = (i: number) =>
    i === 4
      ? `自动计入你的私人分数，达到${targetScore}点时在自己的回合获胜。`
      : sea && i === 0
        ? "移动强盗或海盗并随机偷取一张资源；累计三名骑士可争夺最大骑士军队。"
        : sea && i === 1
          ? "免费建造两条道路、两艘船，或各一；完成第一段后再放置第二段。"
          : devDescriptions[i];
  const playing = room.status === "playing" && !game.finished;
  const canPlay = playing && !room.spectating && you >= 0 && !p?.eliminated;
  const mine = canPlay && game.turn === you;
  const portMine = canPlay && sea?.tribe?.pending?.player === you;
  const setup = g.setupStep < 2 * g.players.length;
  const phase = game.phase;
  const [mode, setMode] = useState("");
  const [chosen, setChosen] = useState<{ type: string; id: number } | null>(
    null,
  );
  const [helperPayment, setHelperPayment] = useState<number[] | null>(null);
  const [helperResource, setHelperResource] = useState(0);
  const [moveFrom, setMoveFrom] = useState<number | null>(null);
  const [dev, setDev] = useState<number | null>(null);
  const [give, setGive] = useState([0, 0, 0, 0, 0]);
  const [take, setTake] = useState([0, 0, 0, 0, 0]);
  const [monopoly, setMonopoly] = useState(0);
  useEffect(() => {
    setMode("");
    setHelperPayment(null);
    setMoveFrom(null);
    setChosen(null);
    setDev(null);
    setGive([0, 0, 0, 0, 0]);
    setTake([0, 0, 0, 0, 0]);
  }, [room.id, game.turn, game.round, phase, g.setupStep]);
  const effective =
    phase === "catan_port"
      ? "port"
      : phase === "catan_setup_settlement"
        ? "settlement"
        : phase === "catan_setup_road" || phase === "catan_roads"
          ? sea &&
            (mode === "ship" ||
              (!g.legal.roads.length && (g.legal.ships?.length || 0) > 0))
            ? "ship"
            : "road"
          : phase === "catan_robber"
            ? sea && mode === "pirate"
              ? "pirate"
              : "robber"
            : mode;
  const affordable = (type: string) =>
    costs[type].every((n, c) => hand[c] >= n);
  const submit = async (a: Record<string, unknown>) => {
    await act(a);
    setHelperPayment(null);
    setMoveFrom(null);
    setMode("");
    setChosen(null);
    setDev(null);
  };
  const mapMinX = sea ? Math.min(...g.vertices.map((v) => v.x)) - 45 : 0;
  const mapMinY = sea ? Math.min(...g.vertices.map((v) => v.y)) - 45 : -15;
  const mapMaxY = sea ? Math.max(...g.vertices.map((v) => v.y)) : 530;
  const mapWidth = sea
    ? Math.max(...g.vertices.map((v) => v.x)) + 45 - mapMinX
    : 680;
  // Keep room for ports and the pirate on the frame, even while it is at sea,
  // so moving the pirate never changes the view or the player's zoom center.
  const mapHeight = sea ? mapMaxY + 56 - mapMinY : 620;
  const mapAspect = mapWidth / mapHeight;
  const control = useRailMapControls({ aspect: mapAspect, minMobileWidth: 0 });
  const { viewport, zoom, zoomAt, dragging } = control;
  const liveNumber = total(g.dice);
  const discard = canPlay && phase === "catan_discard" && g.discardDue[you] > 0;
  const select = (type: string, id: number) => {
    if ((!mine && !(type === "port" && portMine)) || busy) return;
    if ((type === "helper_move" || type === "move_ship") && moveFrom === null) {
      setMoveFrom(id);
      return;
    }
    setChosen({ type, id });
    setDev(null);
  };
  const poly = (id: number) =>
    g.tiles[id].vertices
      .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
      .join(" ");
  const selectableVertex = (id: number) =>
    mine &&
    ((effective === "settlement" && g.legal.settlements.includes(id)) ||
      (effective === "city" && g.legal.cities.includes(id)));
  return (
    <div className={`catan-board ${sea ? "catan-seafarers" : ""}`}>
      <section className="catan-map-panel">
        <div className="catan-map-toolbar">
          <span>
            <Flag size={14} /> {setup ? "起始建设" : `第 ${game.round} 轮`} ·
            {targetScore}分获胜
          </span>
          <div>
            <button
              aria-label="缩小卡坦岛地图"
              disabled={zoom <= 1}
              onClick={() => zoomAt(zoom / 1.2)}
            >
              <ZoomOut size={17} />
            </button>
            <button aria-label="重置卡坦岛地图" onClick={() => zoomAt(1)}>
              {Math.round(zoom * 100)}%
            </button>
            <button
              aria-label="放大卡坦岛地图"
              disabled={zoom >= 3}
              onClick={() => zoomAt(zoom * 1.2)}
            >
              <ZoomIn size={17} />
            </button>
          </div>
        </div>
        {sea && (
          <div className="catan-sea-tools">
            <span>
              {(
                {
                  shores: "驶向新海岸",
                  islands: "四岛",
                  six_islands: "六岛",
                  fog: "迷雾岛",
                  desert: "穿越沙漠",
                  tribe: "遗忘的部落",
                } as Record<string, string>
              )[sea.scenario] || "航海家"}
              {sea.fog && ` · 待探索 ${sea.fog.remaining} 格`}
              {sea.scenario === "desert" && " · 区域首次定居 +2分"}
            </span>
            {mine &&
              (phase === "catan_setup_road" ||
                phase === "catan_roads" ||
                phase === "catan_robber") && (
                <div
                  role="group"
                  aria-label={
                    phase === "catan_robber"
                      ? "选择强盗或海盗"
                      : "选择道路或船只"
                  }
                >
                  {(phase === "catan_robber"
                    ? (["robber", "pirate"] as const)
                    : (["road", "ship"] as const)
                  ).map((key) => (
                    <button
                      key={key}
                      aria-pressed={effective === key}
                      disabled={
                        busy ||
                        (key === "ship" && !g.legal.ships?.length) ||
                        (key === "road" && !g.legal.roads.length)
                      }
                      onClick={() => {
                        setMode(key);
                        setChosen(null);
                        setMoveFrom(null);
                      }}
                    >
                      {
                        {
                          road: "道路",
                          ship: "船只",
                          robber: "强盗",
                          pirate: "海盗",
                        }[key]
                      }
                    </button>
                  ))}
                </div>
              )}
            {mine &&
              phase === "catan_robber" &&
              g.legal.pirate?.includes(-1) && (
                <button
                  disabled={busy}
                  onClick={() => {
                    setMode("pirate");
                    select("pirate", -1);
                  }}
                >
                  海盗移至外海
                </button>
              )}
          </div>
        )}
        <div
          className={`map-scroll catan-map-scroll ${dragging ? "is-dragging" : ""}`}
          ref={viewport}
          style={{ "--catan-map-aspect": mapAspect } as CSSProperties}
          {...control.handlers}
        >
          <div className="map-canvas" style={control.canvasStyle}>
            <svg
              className="catan-map"
              viewBox={`${mapMinX} ${mapMinY} ${mapWidth} ${mapHeight}`}
              role="group"
              aria-label="卡坦岛地图，可缩放拖动，选择地块、道路或交点"
              style={control.mapStyle}
            >
              <defs>
                {g.tiles.map((t) => (
                  <clipPath key={t.id} id={`catan-hex-${t.id}`}>
                    <polygon points={poly(t.id)} />
                  </clipPath>
                ))}
              </defs>
              {g.tiles.map((t) => {
                const available =
                  mine &&
                  ((effective === "robber" &&
                    (g.legal.robber
                      ? g.legal.robber.includes(t.id)
                      : t.id !== g.robber &&
                        t.resource !== 6 &&
                        t.resource !== 8)) ||
                    (effective === "pirate" &&
                      !!g.legal.pirate?.includes(t.id)) ||
                    (effective === "helper_desert" && t.resource === 5));
                return (
                  <g
                    key={`${t.id}-${t.resource}`}
                    role={available ? "button" : undefined}
                    tabIndex={available ? 0 : undefined}
                    aria-label={`地块 ${t.id + 1} ${terrainNames[t.resource]} ${t.number || ""}${g.robber === t.id ? "，强盗所在" : ""}`}
                    className={`catan-hex terrain-${t.resource} ${available ? "selectable" : ""} ${t.number > 0 && liveNumber === t.number && g.robber !== t.id ? "producing" : ""}`}
                    onClick={() => available && select(effective, t.id)}
                    onKeyDown={(e) => {
                      if (available && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        select(effective, t.id);
                      }
                    }}
                  >
                    <polygon
                      points={poly(t.id)}
                      fill={catanColors[t.resource]}
                    />
                    {assets && t.resource !== 8 && (
                      <image
                        href={
                          t.resource < 6
                            ? `${assets}/catan/terrain-${[...resourceKeys, "desert"][t.resource]}-v1.webp`
                            : `${assets}/catan/seafarers/terrain-${t.resource === 6 ? "sea" : "gold"}-v1.webp`
                        }
                        x={t.x - (hexSize * Math.sqrt(3)) / 2}
                        y={t.y - hexSize}
                        width={hexSize * Math.sqrt(3)}
                        height={hexSize * 2}
                        preserveAspectRatio="xMidYMid slice"
                        clipPath={`url(#catan-hex-${t.id})`}
                        pointerEvents="none"
                      />
                    )}
                    {(!assets || t.resource === 8) && (
                      <text
                        className="terrain-symbol"
                        x={t.x}
                        y={t.y - 20}
                        textAnchor="middle"
                      >
                        {
                          ["♣", "▰", "♧", "❧", "◆", "☀", "≈", "◆", "？"][
                            t.resource
                          ]
                        }
                      </text>
                    )}
                    <polygon
                      className="hex-border"
                      points={poly(t.id)}
                      fill="none"
                    />
                    {t.number > 0 && (
                      <g
                        className={`catan-number ${t.number === 6 || t.number === 8 ? "red" : ""}`}
                        transform={`translate(${t.x},${t.y + 4}) scale(${pieceScale})`}
                      >
                        <circle r="19" />
                        <text textAnchor="middle">{t.number}</text>
                      </g>
                    )}
                    {g.robber === t.id && (
                      <g
                        className="catan-robber"
                        transform={`translate(${t.x + 18 * pieceScale},${t.y - 20 * pieceScale}) scale(${pieceScale})`}
                      >
                        {assets ? (
                          <image
                            href={`${assets}/catan/robber-v1.webp`}
                            x="-14"
                            y="-16"
                            width="28"
                            height="46"
                          />
                        ) : (
                          <path d="M-10 7Q-13-2-6-6A8 8 0 1 1 6-6Q13-2 10 7L15 28Q0 35-15 28Z" />
                        )}
                        <title>强盗阻止本地块生产</title>
                      </g>
                    )}
                    {sea?.pirate === t.id && (
                      <g
                        transform={`translate(${t.x},${t.y}) scale(${pieceScale})`}
                      >
                        <CatanPirate assets={assets} />
                        <title>海盗封锁本海域船只</title>
                      </g>
                    )}
                    {(chosen?.type === "pirate" ||
                      chosen?.type === "robber" ||
                      chosen?.type === "helper_desert") &&
                      chosen.id === t.id && (
                        <polygon
                          points={poly(t.id)}
                          className="catan-picked"
                          fill="none"
                        />
                      )}
                  </g>
                );
              })}
              <CatanDesertRegions game={g} />
              <CatanTribeRewards game={g} assets={assets} />
              {[...g.ports, ...(sea?.tribe?.ports || [])].map((port) => {
                const unclaimed = sea?.tribe?.ports?.some(
                  (p) => p.edge === port.edge,
                );
                const e = g.edges[port.edge],
                  a = g.vertices[e.a],
                  b = g.vertices[e.b],
                  x = (a.x + b.x) / 2,
                  y = (a.y + b.y) / 2,
                  land = sea
                    ? g.tiles.find(
                        (t) =>
                          (e.tiles?.includes(t.id) ||
                            (t.vertices.includes(e.a) &&
                              t.vertices.includes(e.b))) &&
                          t.resource !== 6 &&
                          t.resource !== 8,
                      )
                    : undefined,
                  angle = land
                    ? Math.atan2(y - land.y, x - land.x)
                    : Math.atan2(y - 290, x - 340),
                  offset = sea ? hexSize * 0.62 : 34,
                  size = sea ? hexSize * 1.02 : 52,
                  px = x + offset * Math.cos(angle),
                  py = y + offset * Math.sin(angle);
                return (
                  <g
                    key={port.edge}
                    className={`catan-port ${unclaimed ? "unclaimed" : ""}`}
                  >
                    <line x1={a.x} y1={a.y} x2={px} y2={py} />
                    <line x1={b.x} y1={b.y} x2={px} y2={py} />
                    {assets ? (
                      <image
                        href={`${assets}/catan/port-${port.resource < 0 ? "any" : resourceKeys[port.resource]}-v1.webp`}
                        x={px - size / 2}
                        y={py - size / 2}
                        width={size}
                        height={size}
                      />
                    ) : (
                      <circle cx={px} cy={py} r="20" fill="#f4e6c5" />
                    )}
                    <title>
                      {unclaimed ? "待领取：造船或移船到此领取。" : "已安放："}
                      {port.resource < 0
                        ? "通用港口，3:1"
                        : catanNames[port.resource] + "港口，2:1"}
                    </title>
                  </g>
                );
              })}
              {sea && sea.pirate === -1 && (
                <g transform={`translate(340,${mapMaxY + 28})`}>
                  <CatanPirate assets={assets} />
                  <title>海盗在外海</title>
                </g>
              )}
              {g.edges.map((e) => {
                const a = g.vertices[e.a],
                  b = g.vertices[e.b],
                  ok =
                    effective === "port"
                      ? portMine && !!g.legal.ports?.includes(e.id)
                      : mine &&
                        ((effective === "road" &&
                          g.legal.roads.includes(e.id)) ||
                          (effective === "ship" &&
                            !!g.legal.ships?.includes(e.id)) ||
                          (effective === "move_ship" &&
                            (moveFrom === null
                              ? Object.hasOwn(g.shipMoves || {}, e.id)
                              : (g.shipMoves?.[moveFrom] || []).includes(
                                  e.id,
                                ))) ||
                          (effective === "helper_move" &&
                            (moveFrom === null
                              ? Object.hasOwn(g.helperRoadMoves || {}, e.id)
                              : (g.helperRoadMoves?.[moveFrom] || []).includes(
                                  e.id,
                                ))));
                const picked =
                  (chosen?.type === effective && chosen.id === e.id) ||
                  ((effective === "helper_move" || effective === "move_ship") &&
                    moveFrom === e.id);
                return (
                  <g
                    key={e.id}
                    className={`catan-edge ${ok ? "selectable" : ""}`}
                    role={ok ? "button" : undefined}
                    tabIndex={ok ? 0 : undefined}
                    aria-label={`${effective === "port" ? "港口" : e.ship || (e.owner < 0 && (effective === "ship" || effective === "move_ship")) ? "船只" : "道路"}位置 ${e.id + 1}${e.owner >= 0 ? "，" + room.seats[e.owner].name + "已占领" : ""}`}
                    onClick={() => ok && select(effective, e.id)}
                    onKeyDown={(ev) => {
                      if (ok && (ev.key === "Enter" || ev.key === " ")) {
                        ev.preventDefault();
                        select(effective, e.id);
                      }
                    }}
                  >
                    {(ok ||
                      ((effective === "helper_move" ||
                        effective === "move_ship") &&
                        moveFrom === e.id)) && (
                      <line
                        x1={a.x + (b.x - a.x) * 0.2}
                        y1={a.y + (b.y - a.y) * 0.2}
                        x2={b.x + (a.x - b.x) * 0.2}
                        y2={b.y + (a.y - b.y) * 0.2}
                        className={picked ? "picked-road" : "available-road"}
                      />
                    )}
                    {e.owner >= 0 && (
                      <g
                        transform={`translate(${(a.x + b.x) / 2},${(a.y + b.y) / 2}) rotate(${(((Math.atan2(b.y - a.y, b.x - a.x) * 180) / Math.PI + 270) % 180) - 90})`}
                        className={
                          g.players[e.owner].eliminated
                            ? "eliminated-piece"
                            : ""
                        }
                      >
                        <g transform={`scale(${pieceScale})`}>
                          {e.ship ? (
                            <CatanShip assets={assets} player={e.owner} />
                          ) : (
                            <>
                              <rect
                                x="-22"
                                y="-5"
                                width="44"
                                height="10"
                                rx="2"
                                fill={catanPlayerColors[e.owner]}
                                stroke="#523829"
                                strokeWidth="1.5"
                              />
                              <path d="M-20-3H20" stroke="#fff" opacity=".45" />
                            </>
                          )}
                        </g>
                      </g>
                    )}
                    {ok && effective === "port" && (
                      <g
                        pointerEvents="none"
                        transform={`translate(${(a.x + b.x) / 2},${(a.y + b.y) / 2}) scale(${pieceScale})`}
                      >
                        <circle
                          r="13"
                          fill={picked ? "#fff5ac" : "#f9ce58"}
                          stroke="#65461f"
                          strokeWidth={picked ? 3 : 1.5}
                        />
                        <text
                          y="1"
                          textAnchor="middle"
                          dominantBaseline="central"
                          fill="#39291b"
                          fontSize="18"
                        >
                          ⚓
                        </text>
                      </g>
                    )}
                    {ok && (
                      <rect
                        x={(a.x + b.x) / 2 - 11}
                        y={(a.y + b.y) / 2 - 11}
                        width="22"
                        height="22"
                        fill="transparent"
                      />
                    )}
                    {ok && (
                      <line
                        x1={a.x + (b.x - a.x) * 0.22}
                        y1={a.y + (b.y - a.y) * 0.22}
                        x2={b.x + (a.x - b.x) * 0.22}
                        y2={b.y + (a.y - b.y) * 0.22}
                        stroke="transparent"
                        strokeWidth="22"
                      />
                    )}
                  </g>
                );
              })}
              {g.vertices.map((v) => {
                const ok = selectableVertex(v.id),
                  picked =
                    chosen?.id === v.id &&
                    (chosen.type === "city" || chosen.type === "settlement");
                return (
                  <g
                    key={v.id}
                    className={`catan-vertex ${ok ? "selectable" : ""}`}
                    transform={`translate(${v.x},${v.y}) scale(${pieceScale})`}
                    role={ok ? "button" : undefined}
                    tabIndex={ok ? 0 : undefined}
                    aria-label={`交点 ${v.id + 1}${v.level > 0 ? "，" + room.seats[v.owner].name + (v.level === 2 ? "的城市" : "的村庄") : ""}`}
                    onClick={() => ok && select(effective, v.id)}
                    onKeyDown={(e) => {
                      if (ok && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        select(effective, v.id);
                      }
                    }}
                  >
                    {ok && (
                      <circle
                        r={picked ? 15 : 11}
                        className={
                          picked ? "picked-vertex" : "available-vertex"
                        }
                      />
                    )}
                    {v.level > 0 &&
                      (assets ? (
                        <image
                          href={`${assets}/catan/${v.level === 2 ? "city" : "settlement"}-${["blue", "red", "white", "orange", "purple", "green"][v.owner]}-v1.webp`}
                          x={v.level === 2 ? -20 : -15}
                          y="-20"
                          width={v.level === 2 ? 40 : 30}
                          height="34"
                          className={
                            g.players[v.owner].eliminated
                              ? "eliminated-piece"
                              : ""
                          }
                        />
                      ) : (
                        <path
                          className={
                            g.players[v.owner].eliminated
                              ? "eliminated-piece"
                              : ""
                          }
                          d={
                            v.level === 2
                              ? "M-14 10V-5L-5-13L4-5V0H13V10Z"
                              : "M-10 10V-3L0-12L10-3V10Z"
                          }
                          fill={catanPlayerColors[v.owner]}
                          stroke="#523829"
                          strokeWidth="1.8"
                        />
                      ))}
                    {ok && <circle r="16" fill="transparent" />}
                  </g>
                );
              })}
            </svg>
          </div>
        </div>
        <div className="catan-map-hint">
          {phase === "catan_port"
            ? portMine
              ? "选择亮起的海岸位置，再确认安放港口 · 可收起面板查看地图"
              : "等待港口安放 · 滚轮缩放 · 按住拖动"
            : phase === "catan_gold"
              ? canPlay && g.goldPending?.claims[0]?.player === you
                ? "请在金矿面板领取资源 · 可收起面板查看地图"
                : "等待金矿资源选择 · 滚轮缩放 · 按住拖动"
              : mine
                ? effective === "helper_move" || effective === "move_ship"
                  ? moveFrom === null
                    ? effective === "move_ship"
                      ? "选择要移动的己方末端旧船"
                      : "选择要迁移的己方末端道路"
                    : "选择亮起的新位置，再确认移动"
                  : effective === "ship"
                    ? "点击虚线选择船只位置，再确认建造"
                    : effective === "pirate"
                      ? "选择另一块海洋，或将海盗移至外海"
                      : effective === "road"
                        ? "点击虚线选择道路，再确认建造"
                        : effective === "settlement" || effective === "city"
                          ? "点击亮起的交点，再确认建造"
                          : effective === "robber" ||
                              effective === "helper_desert"
                            ? "点击地块选择强盗的新位置"
                            : "选择右侧行动 · 滚轮缩放 · 按住拖动"
                : "滚轮缩放 · 按住拖动 · 等待其他玩家行动"}
        </div>
      </section>
      <aside className="catan-actions">
        <div className="catan-dice" key={g.rollId}>
          <span className={g.rollId ? "rolled" : ""}>{g.dice[0] || "—"}</span>
          <span className={g.rollId ? "rolled" : ""}>{g.dice[1] || "—"}</span>
          <div>
            <b>{g.rollId ? `点数 ${liveNumber}` : "等待掷骰"}</b>
            <small>
              {g.paired?.second && phase === "catan_turn"
                ? "② 配对行动 · 不掷骰、不自由交易"
                : phase === "catan_discard"
                  ? "所有人同时弃牌"
                  : (sea && catanSeafarerPhases[phase]) ||
                    catanPhases[phase] ||
                    "本局已结束"}
            </small>
          </div>
        </div>
        {mine && phase === "catan_roll" && (
          <button
            className="primary wide"
            disabled={busy}
            onClick={() => void submit({ type: "catan_roll" })}
          >
            <Dices size={18} /> 掷骰
          </button>
        )}
        {mine && phase === "catan_turn" && (
          <div className="catan-build-menu">
            {(sea
              ? (["road", "ship", "settlement", "city", "buy_dev"] as const)
              : (["road", "settlement", "city", "buy_dev"] as const)
            ).map((key) => {
              const Icon = {
                road: Route,
                ship: Ship,
                settlement: Home,
                city: Castle,
                buy_dev: ScrollText,
              }[key];
              const label = {
                road: "修建道路",
                ship: "建造船只",
                settlement: "建造村庄",
                city: "升级城市",
                buy_dev: "购买发展卡",
              }[key];
              const available =
                key === "buy_dev"
                  ? g.devRemaining > 0
                  : key === "ship"
                    ? (g.legal.ships?.length || 0) > 0
                    : key === "road"
                      ? g.legal.roads.length > 0
                      : key === "city"
                        ? g.legal.cities.length > 0
                        : g.legal.settlements.length > 0;
              return (
                <button
                  key={key}
                  className={mode === key ? "selected" : ""}
                  disabled={busy || !affordable(key) || !available}
                  onClick={() => {
                    setHelperPayment(null);
                    setMoveFrom(null);
                    setMode(key);
                    setChosen(key === "buy_dev" ? { type: key, id: 0 } : null);
                    setDev(null);
                  }}
                >
                  <span>
                    <Icon size={17} />
                    {label}
                  </span>
                  <Bundle values={costs[key]} assets={assets} />
                </button>
              );
            })}
            {sea && (
              <button
                className={mode === "move_ship" ? "selected" : ""}
                disabled={busy || !Object.keys(g.shipMoves || {}).length}
                onClick={() => {
                  setMode("move_ship");
                  setMoveFrom(null);
                  setChosen(null);
                  setHelperPayment(null);
                  setDev(null);
                }}
              >
                <span>
                  <Move size={17} />
                  移动船只
                </span>
                <small>
                  {sea.movedShip ? "本阶段已移动" : "末端旧船 · 每阶段一次"}
                </small>
              </button>
            )}
            <button
              className={mode === "trade" ? "selected" : ""}
              disabled={busy}
              onClick={() => {
                setHelperPayment(null);
                setMoveFrom(null);
                setMode(mode === "trade" ? "" : "trade");
                setChosen(null);
                setDev(null);
              }}
            >
              <span>
                <ArrowLeftRight size={17} />
                资源交易
              </span>
              <small>
                {g.paired?.second
                  ? "银行或港口（配对行动）"
                  : "银行、港口或其他玩家"}
              </small>
            </button>
            <button
              className="catan-end"
              disabled={busy}
              onClick={() => void submit({ type: "catan_end" })}
            >
              结束回合 →
            </button>
          </div>
        )}
        {chosen && chosen.type !== "port" && mine && (
          <section className="catan-confirm" aria-label="确认行动">
            <strong>
              {
                (
                  {
                    helper_move: "迁移道路",
                    ship: "建造船只",
                    move_ship: "移动船只",
                    pirate: chosen.id === -1 ? "将海盗移至外海" : "移动海盗",
                    helper_desert: "将强盗赶回沙漠",
                    road: "修建道路",
                    settlement: "建造村庄",
                    city: "升级城市",
                    robber: "移动强盗",
                    buy_dev: "购买发展卡",
                  } as Record<string, string>
                )[chosen.type]
              }
              {chosen.type !== "buy_dev" &&
                chosen.id >= 0 &&
                ` #${chosen.id + 1}`}
            </strong>
            {costs[chosen.type] && !setup && phase !== "catan_roads" && (
              <Bundle
                values={helperPayment || costs[chosen.type]}
                assets={assets}
              />
            )}
            <div>
              <button
                className="subtle"
                disabled={busy}
                onClick={() => {
                  setChosen(null);
                  setHelperPayment(null);
                  setMoveFrom(null);
                  setMode("");
                }}
              >
                取消
              </button>
              <button
                className="primary"
                disabled={busy}
                onClick={() =>
                  void submit({
                    type:
                      chosen.type === "helper_move" ||
                      chosen.type === "helper_desert"
                        ? "catan_helper"
                        : "catan_" + chosen.type,
                    choice:
                      chosen.type === "helper_desert" ? "desert" : undefined,
                    color:
                      chosen.type === "helper_desert"
                        ? helperResource
                        : undefined,
                    skill: helperPayment ? "helper" : undefined,
                    tokens: helperPayment || undefined,
                    target:
                      chosen.type === "helper_move" ||
                      chosen.type === "move_ship"
                        ? chosen.id
                        : undefined,
                    edge:
                      chosen.type === "helper_move" ||
                      chosen.type === "move_ship"
                        ? moveFrom
                        : chosen.id,
                    vertex: chosen.id,
                    tile: chosen.id,
                  })
                }
              >
                确认
              </button>
            </div>
          </section>
        )}
        {mine && phase === "catan_roads" && (
          <p className="catan-note">
            剩余免费{sea ? "道路／船只" : "道路"} {g.freeRoads} 段。
            {g.legal.roads.length === 0 && !g.legal.ships?.length && (
              <button
                onClick={() => void submit({ type: "catan_skip_roads" })}
                disabled={busy}
              >
                结束道路建设
              </button>
            )}
          </p>
        )}
        {mine && phase === "catan_steal" && (
          <section className="catan-confirm">
            <strong>选择偷取一张资源的对手</strong>
            {g.victims.map((i) => (
              <button
                className="outline"
                key={i}
                disabled={busy}
                onClick={() => void submit({ type: "catan_steal", target: i })}
              >
                {room.seats[i].name} · {g.players[i].resourceCount} 张资源
              </button>
            ))}
          </section>
        )}
        {phase === "catan_discard" && (
          <section className="catan-confirm">
            <strong>掷出 7 · 超过七张需弃一半</strong>
            {discard ? (
              <>
                <ResourcePicker
                  label={`弃置（需要 ${g.discardDue[you]} 张）`}
                  values={give}
                  onChange={setGive}
                  limits={hand}
                  assets={assets}
                  disabled={busy}
                />
                <button
                  className="primary wide"
                  disabled={busy || total(give) !== g.discardDue[you]}
                  onClick={() =>
                    void submit({ type: "catan_discard", tokens: give })
                  }
                >
                  确认弃置 {total(give)} 张
                </button>
              </>
            ) : (
              <p>
                等待{" "}
                {g.discardDue
                  .map((n, i) => (n > 0 ? room.seats[i].name : ""))
                  .filter(Boolean)
                  .join("、")}{" "}
                弃牌
              </p>
            )}
            <small>超时由系统随机弃牌，弃置种类不会公开。</small>
          </section>
        )}
        {mine && phase === "catan_turn" && mode === "trade" && (
          <section className="catan-trade-editor">
            <ResourcePicker
              label="给出"
              values={give}
              onChange={setGive}
              limits={hand}
              assets={assets}
              disabled={busy}
            />
            <ResourcePicker
              label="换取"
              values={take}
              onChange={setTake}
              limits={Array(5).fill(g.options?.fiveSix ? 24 : 19)}
              assets={assets}
              disabled={busy}
            />
            <small>
              我的银行比例：
              {p.rates.map((n, c) => `${catanNames[c]} ${n}:1`).join(" · ")}
            </small>
            <div className="catan-trade-buttons">
              <button
                className="outline"
                disabled={busy || !total(give) || !total(take)}
                onClick={() => void submit({ type: "catan_bank", give, take })}
              >
                与银行交换
              </button>
              <button
                className="primary"
                disabled={
                  busy || !!g.paired?.second || !total(give) || !total(take)
                }
                onClick={() =>
                  void submit({ type: "catan_trade_offer", give, take })
                }
              >
                向玩家提议
              </button>
            </div>
          </section>
        )}
        {g.trade && playing && (
          <section className="catan-offer">
            <strong>{room.seats[g.trade.from].name} 的交易提议</strong>
            <small>给出</small>
            <Bundle values={g.trade.give} assets={assets} />
            <small>换取</small>
            <Bundle values={g.trade.take} assets={assets} />
            {canPlay &&
              (mine ? (
                <>
                  <div className="catan-offer-responses">
                    {g.trade.responses.map(
                      (answer, i) =>
                        i !== you &&
                        !g.players[i].eliminated && (
                          <div key={i}>
                            <PlayerName user={room.seats[i]} />
                            <span>
                              {answer === 1
                                ? "愿意交易"
                                : answer === -1
                                  ? "已拒绝"
                                  : "等待回应"}
                            </span>
                            {answer === 1 && (
                              <button
                                className="primary"
                                disabled={busy}
                                onClick={() =>
                                  void submit({
                                    type: "catan_trade_complete",
                                    offer: g.trade!.id,
                                    target: i,
                                  })
                                }
                              >
                                与其成交
                              </button>
                            )}
                          </div>
                        ),
                    )}
                  </div>
                  <button
                    className="subtle"
                    disabled={busy}
                    onClick={() => void submit({ type: "catan_trade_cancel" })}
                  >
                    撤回提议
                  </button>
                </>
              ) : (
                <div className="catan-trade-buttons">
                  <button
                    className="outline"
                    disabled={busy}
                    onClick={() =>
                      void submit({
                        type: "catan_trade_reject",
                        offer: g.trade!.id,
                      })
                    }
                  >
                    拒绝
                  </button>
                  <button
                    className="primary"
                    disabled={
                      busy ||
                      g.trade.responses[you] === 1 ||
                      g.trade.take.some((n, c) => n > hand[c])
                    }
                    onClick={() =>
                      void submit({
                        type: "catan_trade_accept",
                        offer: g.trade!.id,
                      })
                    }
                  >
                    {g.trade.responses[you] === 1
                      ? "已接受，等待成交"
                      : "接受提议"}
                  </button>
                </div>
              ))}
          </section>
        )}
        {dev !== null && mine && (
          <section className="catan-confirm">
            <strong>{devNames[dev]}</strong>
            <p>{describeDev(dev)}</p>
            {dev === 2 && (
              <ResourcePicker
                label="丰收领取"
                values={take}
                onChange={setTake}
                limits={g.bank}
                assets={assets}
                disabled={busy}
              />
            )}{" "}
            {dev === 3 && (
              <label>
                收取的资源
                <select
                  value={monopoly}
                  onChange={(e) => setMonopoly(Number(e.target.value))}
                >
                  {catanNames.map((n, i) => (
                    <option key={n} value={i}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <div>
              <button className="subtle" onClick={() => setDev(null)}>
                取消
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  (dev === 2 && total(take) !== Math.min(2, total(g.bank)))
                }
                onClick={() =>
                  void submit({
                    type: "catan_dev",
                    card: dev,
                    take,
                    color: monopoly,
                  })
                }
              >
                使用发展卡
              </button>
            </div>
          </section>
        )}
        <CatanGoldChoice room={room} act={act} busy={busy} assets={assets} />
        <CatanTribePortChoice
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          edge={chosen?.type === "port" ? chosen.id : null}
          clear={() => setChosen(null)}
        />
        <CatanTribeStock room={room} assets={assets} />
        <CatanHelpers
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          onBuild={(kind, payment) => {
            setMode(kind);
            setHelperPayment(payment);
            setMoveFrom(null);
            setDev(null);
            setChosen(kind === "buy_dev" ? { type: kind, id: 0 } : null);
          }}
          onDesert={(color) => {
            setHelperResource(color);
            setMode("helper_desert");
            setMoveFrom(null);
            setHelperPayment(null);
            setChosen(null);
            setDev(null);
          }}
          onMove={() => {
            setMode("helper_move");
            setMoveFrom(null);
            setHelperPayment(null);
            setChosen(null);
            setDev(null);
          }}
        />
        <section className="catan-bank">
          <h3>
            资源银行 <small>发展卡剩余 {g.devRemaining}</small>
          </h3>
          <Bundle values={g.bank} assets={assets} showZero />
        </section>
        <div className="catan-awards">
          <span>
            <Route size={16} />
            {sea ? "最长路线" : "最长道路"}{" "}
            <b>
              {g.longestOwner < 0
                ? "至少 5 段"
                : room.seats[g.longestOwner].name}
            </b>
            <small>+2 分</small>
          </span>
          <span>
            <Shield size={16} />
            最大骑士军队{" "}
            <b>
              {g.armyOwner < 0 ? "至少 3 名" : room.seats[g.armyOwner].name}
            </b>
            <small>+2 分</small>
          </span>
        </div>
      </aside>
      {p && !room.spectating && (
        <section className="catan-hand">
          <header>
            <h3>
              你的资源 <small>{p.resourceCount} 张</small>
            </h3>
            <span>
              道路 {p.roadsLeft}/15 {sea && `· 船只 ${p.shipsLeft}/15 `}· 村庄{" "}
              {p.settlementsLeft}/5 · 城市 {p.citiesLeft}/4 可建
            </span>
          </header>
          <div className="catan-hand-resources">
            {hand.map((n, c) => (
              <CatanResource key={c} color={c} count={n} assets={assets} />
            ))}
          </div>
          <div className="catan-development">
            <strong>发展卡</strong>
            {p.dev?.some((n) => n > 0) ? (
              p.dev.map(
                (n, i) =>
                  n > 0 && (
                    <button
                      key={i}
                      title={describeDev(i)}
                      className={`catan-dev-card dev-${i}`}
                      disabled={
                        i === 4 ||
                        !mine ||
                        busy ||
                        g.playedDev ||
                        n - (p.newDev?.[i] || 0) <= 0 ||
                        !["catan_roll", "catan_turn"].includes(phase)
                      }
                      onClick={() => {
                        setDev(i);
                        setChosen(null);
                        setTake([0, 0, 0, 0, 0]);
                      }}
                    >
                      {assets && (
                        <img src={`${assets}/catan/dev-${i}-v1.webp`} alt="" />
                      )}
                      <span>
                        {devNames[i]} <b>{n}</b>
                      </span>
                      {(p.newDev?.[i] || 0) > 0 && i !== 4 && (
                        <small>{p.newDev?.[i]} 张新购</small>
                      )}
                    </button>
                  ),
              )
            ) : (
              <small>尚未持有发展卡</small>
            )}
            <span className="catan-private-score">
              你的总分 <b>{p.score}</b>
              {p.score > p.publicScore && (
                <small>含 {p.score - p.publicScore} 点秘密胜利点</small>
              )}
            </span>
          </div>
        </section>
      )}
    </div>
  );
}
