import { ExplorerPairedTurn } from "./catan-explorer-paired";
import { ExplorerSpiceMission } from "./catan-explorer-spice";
import { ExplorerFullMissions } from "./catan-explorer-full-missions";
import { useEffect, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import { Anchor, Dices, Minus, Plus, RotateCcw, Ship } from "lucide-react";
import type { Act, Room } from "./types";
import {
  CatanCityActions,
  CatanCityChoice,
  CatanCityOverview,
  CatanCityPiece,
  CatanCityWall,
} from "./catan-city";
import { CatanCityEffects } from "./catan-city-effects";
import {
  CatanMerchant,
  CatanProgressHand,
  CatanTradePowers,
} from "./catan-progress";
import {
  pickProgressTarget,
  progressMapMode,
  progressMapTargets,
} from "./catan-progress-state";
import type { ProgressSelection } from "./catan-progress-state";
import { catanProgressNames } from "./catan-progress-names";
import { Bundle, ResourcePicker } from "./catan-resources";
import {
  ExplorerPiece,
  ExplorerFishPiece,
  ExplorerSpicePiece,
  ExplorerCargoPieces,
  ExplorerEffects,
  useExplorerMotion,
} from "./catan-explorer-effects";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import { useRailMapControls } from "./rail-map-controls";
import {
  explorerActionNames,
  explorerActionDescription,
  explorerActionOptionLabel,
  explorerIsHarbor,
  explorerActionKey,
  explorerChoices,
  explorerCanRespond,
  explorerSelectedAction,
  explorerDiscardAction,
  explorerContents,
  explorerShipPosition,
  explorerTarget,
  explorerResources,
  explorerFreightLabel,
  explorerFishContents,
  explorerSpiceContents,
  explorerSpicePoint,
  explorerCargoPoint,
  explorerFarmDescription,
  explorerFishPoint,
  explorerScenarioLabel,
  explorerLairTotal,
  explorerPhaseLabel,
  explorerSupplyLimits,
  explorerCanOffer,
} from "./catan-explorer-state";
import type { ExplorerAction, ExplorerPick } from "./catan-explorer-state";
import "./catan-explorer.css";

const empty = (count = 5) => Array<number>(count).fill(0);
const total = (n: number[]) => n.reduce((sum, x) => sum + x, 0);
const terrainColors = [
  "#436c39",
  "#b36b41",
  "#9baf55",
  "#d6b763",
  "#909496",
  "#d9c390",
  "#4f9eb8",
  "#d5b047",
  "#dbe8e9",
];
const terrainNames = [
  ...explorerResources,
  "沙漠",
  "海洋",
  "金矿",
  "未探索区域",
];
const primaryTypes = [
  "catan_roll",
  "catan_explorer_begin_move",
  "catan_explorer_fish_roll",
  "catan_end",
];
const buttonKeys = (event: KeyboardEvent, click: () => void) => {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    click();
  }
};

export function CatanExplorerBoard({
  room,
  act,
  busy,
  assets,
  resetOpening,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  resetOpening?: () => Promise<void>;
}) {
  const game = room.game!,
    g = game.catan!,
    x = g.explorer!,
    you = room.you;
  const city = g.citiesKnights;
  const motion = useExplorerMotion(room);
  const arriving = motion?.event.cargo?.map((c) => c.unit) || [];
  const hand = g.players[you]?.resources || empty(g.bank.length);
  const [progress, setProgress] = useState<ProgressSelection | null>(null);
  const [pick, setPick] = useState<ExplorerPick | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [mode, setMode] = useState("");
  const [ship, setShip] = useState(-1);
  const [discard, setDiscard] = useState(() => empty(g.bank.length));
  const [give, setGive] = useState(() => empty(g.bank.length)),
    [take, setTake] = useState(() => empty(g.bank.length));
  const [goldGive, setGoldGive] = useState(0),
    [goldTake, setGoldTake] = useState(0);
  const [error, setError] = useState("");
  const [confirmReset, setConfirmReset] = useState(false);
  const canReset =
    !!resetOpening &&
    !!x.setupBlocked &&
    room.status === "playing" &&
    !room.spectating &&
    room.seats[you]?.id === room.host;
  useEffect(() => setConfirmReset(false), [room.id, room.version, canReset]);
  useEffect(() => {
    setPick(null);
    setMode("");
    setShip(-1);
    setProgress(null);
    setDiscard(empty(g.bank.length));
    setGive(empty(g.bank.length));
    setTake(empty(g.bank.length));
    setGoldGive(0);
    setGoldTake(0);
    setError("");
  }, [
    room.id,
    x.sequence,
    game.phase,
    you,
    g.bank.length,
    city?.pending?.players[0],
  ]);
  const can = explorerCanRespond(room),
    allChoices = explorerChoices(room);
  const mapResponses = [
    "metropolis",
    "pillage",
    "knight_retreat",
    "diplomacy",
    "treason_remove",
    "treason_place",
  ];
  const choices =
    city?.pending && !mapResponses.includes(city.pending.kind)
      ? []
      : allChoices;
  const cityManaged = (type: string) =>
    [
      "catan_improvement",
      "catan_wall",
      "catan_knight_recruit",
      "catan_knight_activate",
      "catan_knight_promote",
      "catan_knight_move",
    ].includes(type);
  const activeProgress =
    can &&
    !busy &&
    progress &&
    !progress.skip &&
    game.turn === you &&
    g.progressPlayable?.includes(progress.card)
      ? progress
      : null;
  const progressMode = activeProgress
    ? progressMapMode(activeProgress.card, true)
    : "";
  const kinds = [...new Set(choices.map((a) => a.type))].filter(
    (type) =>
      !primaryTypes.includes(type) && (!city || type !== "catan_improvement"),
  );
  const effective = kinds.includes(mode)
    ? mode
    : kinds.includes("catan_explorer_sail")
      ? "catan_explorer_sail"
      : kinds[0] || "";
  const options = (progress ? [] : choices).filter(
    (a) =>
      a.type === effective &&
      (ship < 0 ||
        (a.type === "catan_explorer_chase"
          ? a.target === ship
          : a.slot === undefined || a.slot === ship)),
  );
  const selected = explorerSelectedAction(room, pick);
  const describe = (a: ExplorerAction) => {
    const description = explorerActionDescription(g, a);
    return a.type === "catan_explorer_pirate_steal" && a.target !== undefined
      ? description.replace(
          `玩家${a.target + 1}`,
          room.seats[a.target]?.name || `玩家${a.target + 1}`,
        )
      : description;
  };
  const select = (a: ExplorerAction) => {
    setProgress(null);
    setPick({ room: room.id, action: a });
    setCollapsed(false);
    setError("");
  };
  const send = async (a: Record<string, unknown>) => {
    if (busy) return;
    setError("");
    try {
      await act(a);
      setPick(null);
      setProgress(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作未成功，请重试");
    }
  };
  const cityAct: Act = async (a) => {
    if (can) await send({ ...a, prompt: x.sequence });
  };
  const size = g.hexSize || 62;
  const minX = Math.min(...g.vertices.map((v) => v.x)) - 30,
    minY = Math.min(...g.vertices.map((v) => v.y)) - 30;
  const width = Math.max(...g.vertices.map((v) => v.x)) - minX + 30,
    height = Math.max(...g.vertices.map((v) => v.y)) - minY + 30;
  const controls = useRailMapControls({
    aspect: width / height,
    minMobileWidth: 0,
    fillMobileHeight: false,
  });
  const targets = new Map<
    string,
    { kind: "edge" | "vertex" | "tile"; id: number; actions: ExplorerAction[] }
  >();
  for (const a of progressMode ? [] : options) {
    const target = explorerTarget(g, a);
    if (!target) continue;
    const key = `${target.kind}-${target.id}`;
    const item = targets.get(key) || { ...target, actions: [] };
    item.actions.push(a);
    targets.set(key, item);
  }
  if (activeProgress && progressMode) {
    const kind =
      progressMode === "progress_tile"
        ? "tile"
        : progressMode === "progress_edge"
          ? "edge"
          : "vertex";
    for (const id of progressMapTargets(g, you, activeProgress)) {
      targets.set(`${kind}-${id}`, { kind, id, actions: [] });
    }
  }
  const selectedTarget = selected ? explorerTarget(g, selected) : null;
  const alternatives = selectedTarget
    ? options.filter((a) => {
        const p = explorerTarget(g, a);
        return p?.kind === selectedTarget.kind && p.id === selectedTarget.id;
      })
    : [];
  const discardAction = explorerDiscardAction(room, discard);
  const due = g.discardDue[you] || 0;
  const trade = g.trade;
  const mayOffer = explorerCanOffer(room);
  const supply = explorerSupplyLimits(g);
  const noProduction = !!g.paired?.second && !x.setup;
  const gold = x.economy.gold[you] || 0;
  const offerValid =
    mayOffer &&
    total(give) + goldGive > 0 &&
    total(take) + goldTake > 0 &&
    goldGive <= gold &&
    [goldGive, goldTake].every(
      (n) =>
        Number.isInteger(n) &&
        n >= 0 &&
        n <= supply.gold + (x.economy.goldIssued || 0),
    ) &&
    give.every((n, i) => n <= hand[i]);
  const phase = explorerPhaseLabel(game.finished ? "finished" : game.phase);
  const scenario = explorerScenarioLabel(g);
  const farms = new Map(x.board.farms?.map((f) => [f.tile, f]));
  const shoals = new Map(x.board.shoals?.map((s) => [s.tile, s.number]));
  const path =
    selected?.type === "catan_explorer_sail"
      ? [x.fleet.positions[selected.slot!], ...selected.targets!]
          .map((id) => {
            const e = g.edges[id],
              a = g.vertices[e.a],
              b = g.vertices[e.b];
            return `${(a.x + b.x) / 2},${(a.y + b.y) / 2}`;
          })
          .join(" ")
      : "";
  return (
    <section
      className={`explorer-board${g.paired ? " has-paired" : ""}${city ? " has-cities-knights" : ""}`}
      aria-label={`探索者与海盗${scenario}`}
    >
      <div className="explorer-map-column">
        <header className="explorer-heading">
          <div>
            <strong>
              探索者与海盗 · {scenario}
              {city ? " ＋ 城市与骑士" : ""}
            </strong>
            <span>
              {phase} · 目标{x.board.target}分
            </span>
          </div>
          <div
            className="explorer-dice"
            role="img"
            aria-label={
              noProduction
                ? "第二位玩家不掷生产骰"
                : g.dice.some(Boolean)
                  ? `骰子${g.dice.join("、")}`
                  : "等待掷骰"
            }
          >
            <Dices size={20} />
            {noProduction
              ? "不掷骰"
              : g.dice.some(Boolean)
                ? g.dice.join(" + ")
                : "待掷骰"}
          </div>
        </header>
        <ExplorerPairedTurn room={room} />
        <div className="explorer-map-tools">
          <span>
            {targets.size
              ? `选择高亮位置：${activeProgress ? catanProgressNames[activeProgress.card] : explorerActionNames[effective]}`
              : "滚轮缩放 · 拖动地图"}
          </span>
          <button
            aria-label="缩小地图"
            disabled={controls.zoom <= 1}
            onClick={() => controls.zoomAt(controls.zoom / 1.2)}
          >
            <Minus size={16} />
          </button>
          <button
            aria-label="放大地图"
            disabled={controls.zoom >= 3}
            onClick={() => controls.zoomAt(controls.zoom * 1.2)}
          >
            <Plus size={16} />
          </button>
          <button aria-label="重置地图" onClick={() => controls.zoomAt(1)}>
            <RotateCcw size={16} />
          </button>
        </div>
        <div
          ref={controls.viewport}
          className={`map-scroll explorer-map-scroll ${controls.dragging ? "is-dragging" : ""}`}
          style={{ "--explorer-map-aspect": width / height } as CSSProperties}
          {...controls.handlers}
        >
          <div className="map-canvas" style={controls.canvasStyle}>
            <svg
              className="explorer-map"
              viewBox={`${minX} ${minY} ${width} ${height}`}
              style={controls.mapStyle}
              role="group"
              aria-label={`${scenario}地图`}
            >
              <defs>
                {g.tiles.map((t) => (
                  <clipPath id={`explorer-hex-${t.id}`} key={t.id}>
                    <polygon
                      points={t.vertices
                        .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
                        .join(" ")}
                    />
                  </clipPath>
                ))}
              </defs>
              {g.tiles.map((t) => (
                <g key={t.id}>
                  <polygon
                    points={t.vertices
                      .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
                      .join(" ")}
                    fill={terrainColors[t.resource]}
                    stroke="#e3d5b4"
                    strokeWidth="1.4"
                  />
                  {assets && t.resource !== 8 && (
                    <image
                      href={`${assets}/catan/${x.board.council?.tile === t.id ? "explorer/council" : shoals.has(t.id) ? "explorer/fish-shoal" : farms.has(t.id) ? `explorer/farm-${farms.get(t.id)!.ability}${farms.get(t.id)!.ability === "pirate" ? `-${farms.get(t.id)!.pirateDie}` : ""}` : t.resource < 6 ? `terrain-${["wood", "brick", "wool", "grain", "ore", "desert"][t.resource]}` : `seafarers/terrain-${t.resource === 6 ? "sea" : "gold"}`}-v1.webp`}
                      x={t.x - (size * Math.sqrt(3)) / 2}
                      y={t.y - size}
                      width={size * Math.sqrt(3)}
                      height={size * 2}
                      clipPath={`url(#explorer-hex-${t.id})`}
                      preserveAspectRatio="xMidYMid slice"
                    />
                  )}
                  {t.resource === 8 && (
                    <text x={t.x} y={t.y + 8} className="explorer-fog">
                      ?
                    </text>
                  )}
                  {shoals.has(t.id) && (
                    <g
                      className="explorer-shoal-number"
                      aria-label={`渔场骰面${shoals.get(t.id)}`}
                    >
                      <rect
                        x={t.x - 13}
                        y={t.y - 28}
                        width="26"
                        height="26"
                        rx="5"
                      />
                      <text x={t.x} y={t.y - 9}>
                        {shoals.get(t.id)}
                      </text>
                    </g>
                  )}
                  {farms.has(t.id) && (
                    <g
                      className="explorer-farm-label"
                      aria-label={explorerFarmDescription(farms.get(t.id)!)}
                    >
                      <rect
                        x={t.x - 31}
                        y={t.y - 28}
                        width="62"
                        height="19"
                        rx="6"
                      />
                      <text x={t.x} y={t.y - 15}>
                        {farms.get(t.id)!.ability === "swift"
                          ? "航速＋1"
                          : farms.get(t.id)!.ability === "gold"
                            ? "换金币"
                            : `${farms.get(t.id)!.pirateDie}点也成功`}
                      </text>
                    </g>
                  )}
                  {t.number > 0 && (
                    <g
                      className={`explorer-number ${[6, 8].includes(t.number) ? "hot" : ""}`}
                    >
                      <circle cx={t.x} cy={t.y} r={size * 0.22} />
                      <text x={t.x} y={t.y + size * 0.095}>
                        {t.number}
                      </text>
                    </g>
                  )}
                  <title>
                    {x.board.council?.tile === t.id
                      ? "议会岛"
                      : shoals.has(t.id)
                        ? `渔场 · 骰面${shoals.get(t.id)}`
                        : farms.has(t.id)
                          ? `香料农场 · ${explorerFarmDescription(farms.get(t.id)!)}`
                          : terrainNames[t.resource]}
                    {t.number > 0 ? ` · ${t.number}` : ""}
                  </title>
                </g>
              ))}
              {x.board.council?.anchors.map((id) => {
                const v = g.vertices[id];
                return (
                  <g
                    key={`council-${id}`}
                    transform={`translate(${v.x},${v.y})`}
                    pointerEvents="none"
                    className="explorer-council-anchor"
                  >
                    <circle r="12" />
                    <Anchor x="-8" y="-8" width="16" height="16" />
                    <title>议会岛交付锚点</title>
                  </g>
                );
              })}
              {g.edges
                .filter((e) => e.owner !== -1)
                .map((e) => {
                  const a = g.vertices[e.a],
                    b = g.vertices[e.b];
                  return (
                    <line
                      key={e.id}
                      x1={a.x * 0.8 + b.x * 0.2}
                      y1={a.y * 0.8 + b.y * 0.2}
                      x2={b.x * 0.8 + a.x * 0.2}
                      y2={b.y * 0.8 + a.y * 0.2}
                      stroke={catanSeatColor(g, e.owner)}
                      strokeWidth="7"
                      strokeLinecap="round"
                    />
                  );
                })}
              {g.vertices
                .filter((v) => v.level > 0)
                .map((v) => (
                  <g
                    key={v.id}
                    transform={`translate(${v.x} ${v.y})`}
                    pointerEvents="none"
                  >
                    <g data-city-piece={city ? v.id : undefined}>
                      {city && (
                        <CatanCityWall game={g} vertex={v.id} assets={assets} />
                      )}
                      {explorerIsHarbor(g, v.id) ? (
                        <ExplorerPiece
                          g={g}
                          assets={assets}
                          player={v.owner}
                          kind="harbor"
                          width={34}
                        />
                      ) : assets ? (
                        <image
                          href={`${assets}/catan/${v.level === 2 ? "city" : "settlement"}-${catanPieceColors[catanColorIndex(g, v.owner)]}-v1.webp`}
                          x="-17"
                          y="-24"
                          width="34"
                          height="32"
                        />
                      ) : (
                        <path
                          d={
                            v.level === 2
                              ? "M-15 8V-8H-5V-18H6V-8H15V8Z"
                              : "M-12 8V-6L0-17L12-6V8Z"
                          }
                          fill={catanSeatColor(g, v.owner)}
                          stroke="#433624"
                          strokeWidth="2"
                        />
                      )}
                      {city && (
                        <CatanCityPiece
                          game={g}
                          vertex={v.id}
                          assets={assets}
                        />
                      )}
                    </g>
                    {explorerContents(g, "harbor", v.id).length > 0 && (
                      <g transform="translate(0,20)">
                        <ExplorerCargoPieces
                          arriving={arriving}
                          g={g}
                          assets={assets}
                          units={explorerContents(g, "harbor", v.id)}
                          spiceCount={
                            explorerSpiceContents(g, "harbor", v.id).length
                          }
                        />
                      </g>
                    )}
                    <title>
                      {v.owner < 0 ? "中立" : room.seats[v.owner]?.name} ·{" "}
                      {explorerIsHarbor(g, v.id)
                        ? "港口"
                        : v.level === 2
                          ? "城市"
                          : "村庄"}{" "}
                      · 位置{v.id + 1}
                    </title>
                  </g>
                ))}
              {x.lairs?.sites.map((site) => {
                const t = g.tiles[site.tile],
                  units = explorerContents(g, "lair", site.tile);
                return (
                  <g key={`lair-${site.tile}`} pointerEvents="none">
                    {!site.resolved && (
                      <g className="explorer-lair-marker">
                        <circle cx={t.x} cy={t.y} r="21" />
                        <text x={t.x} y={t.y + 4}>
                          {site.ready ? "待结算" : "巢穴"}
                        </text>
                      </g>
                    )}
                    {units.map((id, i) => (
                      <g
                        key={id}
                        className={
                          arriving.includes(id)
                            ? "explorer-cargo-arriving"
                            : undefined
                        }
                        transform={`translate(${t.x + (i - (units.length - 1) / 2) * 18},${t.y + 34})`}
                      >
                        <ExplorerPiece
                          g={g}
                          assets={assets}
                          player={Math.floor(id / 11)}
                          kind="crew"
                          width={12}
                        />
                        <title>
                          {room.seats[Math.floor(id / 11)]?.name}的船员
                        </title>
                      </g>
                    ))}
                  </g>
                );
              })}
              {x.pirate && x.pirate.owner >= 0 && g.tiles[x.pirate.tile] && (
                <g
                  key={motion?.event.pirate ? motion.event.id : "pirate"}
                  transform={`translate(${g.tiles[x.pirate.tile].x},${g.tiles[x.pirate.tile].y})`}
                  className={
                    motion?.event.pirate ? "explorer-cargo-arriving" : undefined
                  }
                  pointerEvents="none"
                >
                  <ExplorerPiece
                    g={g}
                    assets={assets}
                    player={x.pirate.owner}
                    kind="pirate"
                    width={44}
                  />
                  <title>{room.seats[x.pirate.owner]?.name}的海盗船</title>
                </g>
              )}
              {path && (
                <polyline
                  points={path}
                  className="explorer-voyage-preview"
                  pointerEvents="none"
                />
              )}
              {x.fleet.positions.map((at, id) => {
                const p = explorerShipPosition(g, id);
                if (
                  at < 0 ||
                  !p ||
                  (motion?.event.kind === "catan_explorer_sail" &&
                    motion.event.ship === id)
                )
                  return null;
                return (
                  <g
                    key={id}
                    transform={`translate(${p.x} ${p.y})`}
                    className="explorer-vessel"
                    pointerEvents="none"
                  >
                    <ExplorerPiece
                      g={g}
                      assets={assets}
                      player={Math.floor(id / 3)}
                      kind="ship"
                    />
                    {explorerContents(g, "ship", id).length > 0 && (
                      <g transform="translate(0,-12)">
                        <ExplorerCargoPieces
                          arriving={arriving}
                          g={g}
                          assets={assets}
                          units={explorerContents(g, "ship", id)}
                          spiceCount={
                            explorerSpiceContents(g, "ship", id).length
                          }
                        />
                      </g>
                    )}
                    <text y="30" className="explorer-ship-label">
                      {(id % 3) + 1}
                    </text>
                    <title>
                      {room.seats[Math.floor(id / 3)]?.name} · 船{(id % 3) + 1}
                    </title>
                  </g>
                );
              })}
              {x.cargo.fish?.map((loc, id) => {
                const p = explorerFishPoint(g, id);
                if (
                  !p ||
                  motion?.event.fish?.some((f) => f.fish === id) ||
                  (loc.kind === "ship" &&
                    motion?.event.kind === "catan_explorer_sail" &&
                    motion.event.ship === loc.index)
                )
                  return null;
                return (
                  <g
                    key={`fish-${id}`}
                    transform={`translate(${p.x},${p.y})`}
                    pointerEvents="none"
                  >
                    <ExplorerFishPiece
                      assets={assets}
                      width={loc.kind === "shoal" ? 34 : 27}
                    />
                    <title>
                      {loc.kind === "ship"
                        ? `${room.seats[Math.floor(loc.index / 3)]?.name}的船${(loc.index % 3) + 1}`
                        : loc.kind === "harbor"
                          ? "港口"
                          : "渔场"}{" "}
                      · 一群鱼
                    </title>
                  </g>
                );
              })}
              {x.cargo.units.map((loc, id) => {
                if (loc.kind !== "farm") return null;
                const p = explorerCargoPoint(g, id, loc);
                if (!p) return null;
                return (
                  <g
                    key={`farm-crew-${id}`}
                    transform={`translate(${p.x},${p.y})`}
                    pointerEvents="none"
                    className={
                      arriving.includes(id)
                        ? "explorer-cargo-arriving"
                        : undefined
                    }
                  >
                    <ExplorerPiece
                      g={g}
                      assets={assets}
                      player={Math.floor(id / 11)}
                      kind="crew"
                      width={11}
                    />
                    <title>
                      {room.seats[Math.floor(id / 11)]?.name} · 永久派驻船员
                    </title>
                  </g>
                );
              })}
              {x.cargo.spice?.map((sack, id) => {
                const p = explorerSpicePoint(g, id);
                if (
                  !p ||
                  motion?.event.spice?.some((s) => s.sack === id) ||
                  (sack.at.kind === "ship" &&
                    motion?.event.kind === "catan_explorer_sail" &&
                    motion.event.ship === sack.at.index)
                )
                  return null;
                return (
                  <g
                    key={`spice-${id}`}
                    transform={`translate(${p.x},${p.y})`}
                    pointerEvents="none"
                  >
                    <ExplorerSpicePiece assets={assets} />
                    <title>
                      {sack.owner < 0 ? "待领取" : room.seats[sack.owner]?.name}{" "}
                      · 一袋香料
                    </title>
                  </g>
                );
              })}
              <ExplorerEffects active={motion} assets={assets} />
              {city && (
                <>
                  <CatanMerchant game={g} assets={assets} />
                  {city.knights.map((n) => (
                    <g
                      key={`${n.owner}-${n.vertex}`}
                      transform={`translate(${g.vertices[n.vertex].x} ${g.vertices[n.vertex].y})`}
                    >
                      <g data-city-piece={n.vertex}>
                        <CatanCityPiece
                          game={g}
                          vertex={n.vertex}
                          assets={assets}
                        />
                      </g>
                    </g>
                  ))}
                </>
              )}
              {!busy &&
                [...targets.entries()]
                  // A docked ship's edge ends at the harbor. Render its hit
                  // area first so it cannot intercept the harbor's center.
                  .sort(([, a], [, b]) => {
                    const layer = { tile: 0, edge: 1, vertex: 2 };
                    return layer[a.kind] - layer[b.kind];
                  })
                  .map(([key, item]) => {
                    const chosen = progressMode
                      ? activeProgress?.picks.includes(item.id)
                      : selectedTarget?.kind === item.kind &&
                        selectedTarget.id === item.id;
                    const click = () =>
                      activeProgress && progressMode
                        ? setProgress(
                            pickProgressTarget(g, you, activeProgress, item.id),
                          )
                        : select(item.actions[0]);
                    const props = {
                      role: "button",
                      tabIndex: 0,
                      "aria-label": `${activeProgress && progressMode ? catanProgressNames[activeProgress.card] : explorerActionNames[effective]}${item.kind === "edge" ? "道路" : item.kind === "tile" ? "地块" : "位置"}${item.id + 1}`,
                      onClick: click,
                      onKeyDown: (e: KeyboardEvent) => buttonKeys(e, click),
                    };
                    if (item.kind === "tile") {
                      const t = g.tiles[item.id];
                      return (
                        <polygon
                          key={key}
                          className={`explorer-target-tile ${chosen ? "picked" : ""}`}
                          points={t.vertices
                            .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
                            .join(" ")}
                          {...props}
                        />
                      );
                    }
                    if (item.kind === "vertex") {
                      const v = g.vertices[item.id];
                      return (
                        <circle
                          key={key}
                          className={`explorer-target ${chosen ? "picked" : ""}`}
                          cx={v.x}
                          cy={v.y}
                          r="14"
                          {...props}
                        />
                      );
                    }
                    const e = g.edges[item.id],
                      a = g.vertices[e.a],
                      b = g.vertices[e.b];
                    return (
                      <g
                        key={key}
                        className={`explorer-target-edge ${chosen ? "picked" : ""}`}
                        {...props}
                      >
                        <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} />
                        <line
                          className="hit"
                          x1={a.x}
                          y1={a.y}
                          x2={b.x}
                          y2={b.y}
                        />
                      </g>
                    );
                  })}
            </svg>
          </div>
        </div>
        <div className="explorer-map-caption">
          未探索：鹦鹉区{x.board.unexplored[0]}块 · 鹅区{x.board.unexplored[1]}
          块{" "}
          <span>
            {city
              ? "城市与港口2分，村庄1分 · 港口每格仍产1资源"
              : "港口2分，村庄1分 · 港口每格仍产1资源"}
          </span>
        </div>
      </div>
      <aside className="explorer-panel">
        {room.spectating ? (
          <p className="explorer-notice">
            正在观战 · 船只与货物公开，手牌仅本人可见
          </p>
        ) : (
          <>
            <div className="explorer-hand">
              <strong>{city ? "你的资源与商品" : "你的资源"}</strong>
              <b className="explorer-gold">金币 {gold}</b>
              <Bundle values={hand} assets={assets} showZero />
            </div>
            {room.seats[you]?.autoPlay && (
              <p className="explorer-notice">电脑正在托管，请先收回操作权。</p>
            )}
          </>
        )}
        {game.phase === "catan_discard" && can && due > 0 && (
          <section className="explorer-discard">
            <ResourcePicker
              label={`归还${city ? "资源与商品" : "资源"}（共${due}张）`}
              values={discard}
              onChange={setDiscard}
              limits={hand}
              assets={assets}
              disabled={busy}
            />
            <button
              disabled={busy || !discardAction}
              onClick={() => discardAction && void send(discardAction)}
            >
              确认归还 {total(discard)}/{due}
            </button>
          </section>
        )}
        {x.setupBlocked && (
          <section className="explorer-notice" role="status">
            <strong>开局暂时无法继续</strong>
            <p>当前已没有合法放置位置，需要房主重新布置起始棋子。</p>
            {canReset && (
              <button disabled={busy} onClick={() => setConfirmReset(true)}>
                重新布置开局
              </button>
            )}
          </section>
        )}
        {canReset && confirmReset && (
          <section
            className="explorer-confirm"
            role="dialog"
            aria-label="重新布置开局"
            onKeyDown={(e) => {
              if (e.key === "Escape") setConfirmReset(false);
            }}
          >
            <strong>重新摆放所有起始棋子？</strong>
            <p>
              所有玩家已放置的起始棋子将收回，由原先手重新开始摆放。地图、先手顺序和隐藏牌堆保持不变。
            </p>
            <div className="explorer-confirm-actions">
              <button disabled={busy} onClick={() => setConfirmReset(false)}>
                取消
              </button>
              <button
                disabled={busy}
                onClick={async () => {
                  if (busy || !resetOpening) return;
                  setError("");
                  try {
                    await resetOpening();
                    setConfirmReset(false);
                  } catch (e) {
                    setError(
                      e instanceof Error ? e.message : "重新布置失败，请重试",
                    );
                  }
                }}
              >
                确认重新布置
              </button>
            </div>
          </section>
        )}
        {x.setup && (
          <p className="explorer-notice">
            开局第{x.setup.step + 1}步
            {x.setupPlacement && x.setupPlacement.owner < 0
              ? "（为中立方放置）"
              : ""}
            ：
            {city
              ? "按顺序放城市、逆序放港口，再放道路和载移民的船。全部完成后领取城市旁的普通起始资源，不领取商品。"
              : "按顺序放港口、逆序放村庄，再放道路和载移民的船。所有人完成后领取起始资源。"}
          </p>
        )}
        {city && (
          <>
            <CatanCityOverview room={room} assets={assets} />
            {city.pending &&
              !["diplomacy", "treason_remove", "treason_place"].includes(
                city.pending.kind,
              ) && (
                <CatanCityChoice
                  room={room}
                  act={cityAct}
                  busy={busy || !can}
                  assets={assets}
                  chosen={null}
                />
              )}
            {!x.setup && !city.pending && (
              <>
                <CatanProgressHand
                  room={room}
                  act={cityAct}
                  busy={busy || !can}
                  assets={assets}
                  selection={progress}
                  onChange={(s) => {
                    setProgress(s);
                    setPick(null);
                    setMode("");
                  }}
                />
                <CatanTradePowers
                  room={room}
                  act={cityAct}
                  busy={busy || !can}
                  assets={assets}
                />
                <CatanCityActions
                  room={room}
                  act={cityAct}
                  busy={busy || !can}
                  assets={assets}
                  mode={progress ? "" : effective.replace(/^catan_/, "")}
                  selectMode={(key) => {
                    setMode(key ? `catan_${key}` : "");
                    setPick(null);
                    setProgress(null);
                    setShip(-1);
                  }}
                />
              </>
            )}
          </>
        )}
        {choices.length > 0 && (
          <>
            <div className="explorer-primary">
              {choices
                .filter((a) => primaryTypes.includes(a.type))
                .map((a) => (
                  <button
                    key={a.type}
                    disabled={busy}
                    onClick={() =>
                      a.type === "catan_roll" ? void send(a) : select(a)
                    }
                  >
                    {explorerActionNames[a.type]}
                  </button>
                ))}
            </div>
            <div className="explorer-modes" role="group" aria-label="探险操作">
              {kinds
                .filter((type) => !city || !cityManaged(type))
                .map((type) => (
                  <button
                    key={type}
                    disabled={busy}
                    aria-pressed={effective === type}
                    onClick={() => {
                      setMode(type);
                      setPick(null);
                      setShip(-1);
                      setProgress(null);
                    }}
                  >
                    {explorerActionNames[type]}
                  </button>
                ))}
            </div>
            {game.phase === "catan_explorer_move" && (
              <div
                className="explorer-fleet"
                role="group"
                aria-label="选择船只"
              >
                <button
                  aria-pressed={ship < 0}
                  onClick={() => {
                    setShip(-1);
                    setPick(null);
                  }}
                >
                  全部船只
                </button>
                {x.fleet.positions.map((at, id) =>
                  at >= 0 && Math.floor(id / 3) === you ? (
                    <button
                      key={id}
                      aria-pressed={ship === id}
                      onClick={() => {
                        setShip(id);
                        setPick(null);
                      }}
                    >
                      <Ship size={16} />船{(id % 3) + 1} ·{" "}
                      {x.fleet.turn?.ships[id].closed
                        ? "已停止"
                        : `${x.fleet.turn?.ships[id].remaining ?? 0}步`}
                      {" · " +
                        explorerFreightLabel(
                          explorerContents(g, "ship", id),
                          explorerFishContents(g, "ship", id),
                          explorerSpiceContents(g, "ship", id),
                        )}
                    </button>
                  ) : null,
                )}
              </div>
            )}
            {targets.size > 0 && (
              <p className="explorer-hint">
                点击地图高亮位置，查看费用与操作后确认。
              </p>
            )}
            <div className="explorer-option-list">
              {options
                .filter((a) => !explorerTarget(g, a))
                .map((a) => (
                  <button
                    key={explorerActionKey(a)}
                    disabled={busy}
                    onClick={() => select(a)}
                  >
                    {describe(a)}
                  </button>
                ))}
            </div>
          </>
        )}
        {selected && (
          <section
            className="explorer-confirm"
            role="dialog"
            aria-label="确认探险操作"
            onKeyDown={(e) => {
              if (e.key === "Escape") setPick(null);
            }}
          >
            <header className="explorer-confirm-heading">
              <strong>{explorerActionNames[selected.type]}</strong>
              <button
                aria-label={collapsed ? "展开操作确认" : "收起操作确认"}
                aria-expanded={!collapsed}
                onClick={() => setCollapsed(!collapsed)}
              >
                {collapsed ? "展开" : "收起"}
              </button>
            </header>
            <div hidden={collapsed}>
              {alternatives.length > 1 && (
                <label>
                  {selected.type === "catan_explorer_unit"
                    ? "选择招募单位与归还货物"
                    : selected.type.startsWith("catan_knight_")
                      ? "选择骑士与目标"
                      : selected.type === "catan_treason_place"
                        ? "选择骑士等级"
                        : "选择船只或舱位"}
                  <select
                    value={explorerActionKey(selected)}
                    onChange={(e) => {
                      const a = alternatives.find(
                        (a) => explorerActionKey(a) === e.target.value,
                      );
                      if (a) select(a);
                    }}
                  >
                    {alternatives.map((a) => (
                      <option
                        key={explorerActionKey(a)}
                        value={explorerActionKey(a)}
                      >
                        {explorerActionOptionLabel(g, a)}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <p>{describe(selected)}</p>
              <div className="explorer-confirm-actions">
                <button disabled={busy} onClick={() => setPick(null)}>
                  取消
                </button>
                <button
                  className="primary"
                  disabled={busy}
                  onClick={() => {
                    const fresh = explorerSelectedAction(room, pick);
                    if (fresh) void send(fresh);
                  }}
                >
                  确认{explorerActionNames[selected.type]}
                </button>
              </div>
            </div>
          </section>
        )}
        {error && (
          <p role="alert" className="explorer-error">
            {error}
          </p>
        )}
        {x.lairs && x.spice && x.fish ? (
          <ExplorerFullMissions
            room={room}
            assets={assets}
            fishRolling={Boolean(motion?.event.fishRoll)}
          />
        ) : (
          <>
            {x.lairs && (
              <section className="explorer-mission" aria-label="巢穴任务进度">
                <strong>巢穴任务</strong>
                {g.players.map((p, id) => (
                  <div key={id} className={p.eliminated ? "retired" : ""}>
                    <span style={{ borderColor: catanSeatColor(g, id) }}>
                      {room.seats[id]?.name}
                    </span>
                    <b>进度 {x.lairs!.progress[id]} / 7</b>
                    <span>
                      任务 {x.lairs!.scores[id]}分
                      {x.lairs!.leader === id ? " · 领先" : ""}
                    </span>
                  </div>
                ))}
                <p>
                  已解放{x.lairs.sites.filter((site) => site.resolved).length} /
                  {explorerLairTotal(g)}处；未攻陷前数字隐藏。
                </p>
                {x.lairs.battle && (
                  <p role="status">
                    巢穴{x.lairs.battle.tile + 1}：
                    {x.lairs.battle.candidates
                      .map((id) => room.seats[id]?.name)
                      .join("、")}
                    掷英雄骰。
                  </p>
                )}
                {x.pirate?.lastChase && (
                  <p>
                    最近驱赶：{room.seats[x.pirate.lastChase.player]?.name}掷出
                    {x.pirate.lastChase.die}，
                    {x.pirate.lastChase.success ? "成功" : "未成功"}。
                  </p>
                )}
              </section>
            )}
            {x.spice && <ExplorerSpiceMission room={room} assets={assets} />}
            {x.fish && (
              <section
                className="explorer-mission explorer-fish-mission"
                aria-label="鱼群任务进度"
              >
                <strong>鱼群任务</strong>
                {g.players.map((p, id) => (
                  <div key={id} className={p.eliminated ? "retired" : ""}>
                    <span style={{ borderColor: catanSeatColor(g, id) }}>
                      {room.seats[id]?.name}
                    </span>
                    <b>进度 {x.fish!.progress[id]} / 7</b>
                    <span>
                      任务 {x.fish!.scores[id]}分
                      {x.fish!.leader === id ? " · 领先" : ""}
                    </span>
                  </div>
                ))}
                <p>
                  供应剩余{" "}
                  {x.cargo.fish?.filter((loc) => loc.kind === "supply")
                    .length ?? 0}{" "}
                  群鱼 · 运到议会岛锚点交付。
                </p>
                {x.fish.lastRoll && (
                  <p
                    role="status"
                    key={x.fish.lastRoll.sequence}
                    className={
                      motion?.event.fishRoll ? "explorer-fish-roll" : undefined
                    }
                  >
                    最近捕鱼骰：{room.seats[x.fish.lastRoll.player]?.name}掷出{" "}
                    {x.fish.lastRoll.die}，
                    {x.fish.lastRoll.spawned >= 0 ? "出现一群鱼" : "未出现鱼群"}
                    。
                  </p>
                )}
              </section>
            )}
          </>
        )}
        {trade && (
          <section className="explorer-trade">
            <strong>{room.seats[trade.from]?.name}提出交易</strong>
            <p>付出</p>
            <Bundle values={trade.give} assets={assets} />
            {!!trade.goldGive && <span>金币 {trade.goldGive}</span>}
            <p>希望收到</p>
            <Bundle values={trade.take} assets={assets} />
            {!!trade.goldTake && <span>金币 {trade.goldTake}</span>}
            {can && game.phase === "catan_turn" && (
              <div className="explorer-trade-buttons">
                {trade.from === you ? (
                  <>
                    <button
                      disabled={busy}
                      onClick={() =>
                        void send({
                          type: "catan_trade_cancel",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      取消报价
                    </button>
                    {trade.responses.map((r, p) =>
                      p !== you && r === 1 ? (
                        <button
                          key={p}
                          disabled={busy}
                          onClick={() =>
                            void send({
                              type: "catan_trade_complete",
                              offer: trade.id,
                              target: p,
                              prompt: x.sequence,
                            })
                          }
                        >
                          与{room.seats[p]?.name}成交
                        </button>
                      ) : null,
                    )}
                  </>
                ) : (
                  <>
                    <button
                      disabled={
                        busy ||
                        trade.responses[you] === 1 ||
                        trade.take.some((n, i) => n > hand[i]) ||
                        (trade.goldTake || 0) > gold
                      }
                      onClick={() =>
                        void send({
                          type: "catan_trade_accept",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      {trade.responses[you] === 1
                        ? "已同意，等待成交"
                        : "同意报价"}
                    </button>
                    <button
                      disabled={busy || trade.responses[you] === -1}
                      onClick={() =>
                        void send({
                          type: "catan_trade_reject",
                          offer: trade.id,
                          prompt: x.sequence,
                        })
                      }
                    >
                      拒绝
                    </button>
                  </>
                )}
              </div>
            )}
          </section>
        )}
        {mayOffer && (
          <details className="explorer-trade">
            <summary>向玩家提出交易</summary>
            <ResourcePicker
              label="我付出"
              values={give}
              onChange={setGive}
              limits={hand}
              assets={assets}
              disabled={busy}
            />
            <label>
              付出金币
              <input
                type="number"
                min="0"
                max={gold}
                value={goldGive}
                disabled={busy}
                onChange={(e) => setGoldGive(Number(e.target.value))}
              />
            </label>
            <ResourcePicker
              label="我需要"
              values={take}
              onChange={setTake}
              limits={g.bank.map((_, i) =>
                i < 5 ? supply.resource : g.players.length > 4 ? 18 : 12,
              )}
              assets={assets}
              disabled={busy}
            />
            <label>
              需要金币
              <input
                type="number"
                min="0"
                max={supply.gold + (x.economy.goldIssued || 0)}
                value={goldTake}
                disabled={busy}
                onChange={(e) => setGoldTake(Number(e.target.value))}
              />
            </label>
            <button
              disabled={busy || !offerValid}
              onClick={() =>
                void send({
                  type: "catan_trade_offer",
                  prompt: x.sequence,
                  give,
                  take,
                  goldGive,
                  goldTake,
                })
              }
            >
              提出报价
            </button>
          </details>
        )}
        {!choices.length &&
          !due &&
          !trade &&
          !city?.pending &&
          room.status === "playing" &&
          game.turn !== you && (
            <p className="explorer-notice">
              等待{room.seats[game.turn]?.name}完成{phase}。
            </p>
          )}
        <details className="explorer-rules">
          <summary>
            <Anchor size={16} /> {scenario}规则与银行
          </summary>
          <p>
            每船4步，可付1羊毛加2步。切换船只后不能回到上一艘；发现地块会停止本船。普通资源探索奖受银行库存限制，缺货时仍揭图停船，不补发金币。船端移民可在合法陆地点定居，船和移民回供应。
          </p>
          <p>
            每个港口和船舱有2格，一枚移民占2格，一名船员占1格
            {x.spice ? "，一袋香料也占1格" : ""}
            。只能通过实际停靠的己方港口装卸。
          </p>
          <p>
            {city
              ? "城市在森林、牧场、矿山生产资源与商品，港口仅生产一份资源。普通资源默认3:1，商品默认4:1，城市与商人优惠以银行选项为准；2金币只能购买普通资源，每行动阶段最多两次。建设结束时进步牌须减至4张，再开始航行。"
              : "非7点未获资源的玩家获1金币补偿。3同类资源可换1其他资源或金币；2金币可买1资源，每回合最多2次。无发展卡、强盗、最长道路或最大军队。"}
          </p>
          {g.paired && (
            <p>
              五六人每轮只生产一次。第一位先完成生产、建设和航行，第二位不掷生产骰，直接建设与航行且只能向银行交易。任一位在自己的阶段达到目标分即获胜。
            </p>
          )}
          {x.spice && (
            <p>
              每座农场只可派驻一次，取得香料后才能在该农场建造；船员永久留驻。航速农场各加1步，海盗农场增加成功骰面，金币农场各提供每回合1次资源换金币。香料轨道六格为1、1、2、2、3、3分，领先额外1分，同进度先到者保留领先。
            </p>
          )}
          {x.fish && (
            <p>
              每航行阶段可掷一次捕鱼骰；一群鱼占2格，可在相邻渔场装船、经己方港口换载，或在议会岛两个锚点交付。装卸不消耗移动点；海盗会清除尚未装船的鱼群。发现渔场得2金币并停止本船移动。
            </p>
          )}
          <Bundle values={g.bank} assets={assets} showZero />
          {x.lairs && (
            <p>
              掷中已解放金矿的点数时，每座相邻建筑产2金币，城市也一样；只获得金币、没有获得资源或商品的玩家仍领取1金币补偿。
            </p>
          )}
          {x.economy.goldRule === "ledger" && (
            <p>
              本站补充规则：金币供应不设上限，实体金币用完后继续记账发放；资源牌与商品仍受银行库存限制。
            </p>
          )}
          <p>
            {x.economy.goldRule === "ledger"
              ? "金币供应不限"
              : `金币银行 ${x.economy.goldBank}`}{" "}
            · 本回合金币购资源 {x.economy.bought}
            /2
          </p>
        </details>
      </aside>
      {city && <CatanCityEffects room={room} assets={assets} />}
    </section>
  );
}
