import {
  CatanBridge,
  CatanCoins,
  CatanGoldTradePicker,
  CatanRiverBank,
  CatanRiverStart,
} from "./catan-rivers";
import { catanRiverImages } from "./catan-rivers-layout";
import {
  CatanFishLakeNumbers,
  CatanFishingGrounds,
  CatanFishingPanel,
} from "./catan-fishing";
import { fishResponder } from "./catan-fishing-state";
import { catanPortLayout } from "./catan-port-layout";
import { catanProductionNumbers, catanTileProducing } from "./catan-production";
import { CatanFriendlyRobberStatus } from "./catan-friendly-robber";
import {
  CatanCardEventChoice,
  CatanCardEventSummary,
} from "./catan-card-event";
import {
  catanCardEventActor,
  catanCardEventMapMode,
  catanRevealedEvent,
} from "./catan-card-event-state";
import { CatanHarborsStatus } from "./catan-harbors";
import { catanSavedVictoryTarget } from "./catan-rule-context";
import { CatanCityEffects } from "./catan-city-effects";
import {
  CatanProgressHand,
  CatanTradePowers,
  CatanMerchant,
} from "./catan-progress";
import {
  pickProgressTarget,
  progressMapMode,
  progressMapTargets,
} from "./catan-progress-state";
import type { ProgressSelection } from "./catan-progress-state";
import { progressMapChoices } from "./catan-progress-choice-state";
import {
  CatanCityActions,
  CatanCityChoice,
  CatanCityOverview,
  CatanCityPiece,
  CatanCityWall,
} from "./catan-city";
import {
  cityActionCosts,
  cityActionNames,
  cityDiscardLimit,
  cityMetropolisSites,
  cityWallSites,
} from "./catan-city-state";
import { CatanResource, Bundle, ResourcePicker } from "./catan-resources";
import {
  catanCardNames,
  catanCardSupply,
  catanBankReason,
} from "./catan-cards";
import { CatanNewWorldPortChoice } from "./catan-new-world";
import {
  CatanWorldFishChoice,
  CatanWorldFishPreview,
} from "./catan-world-fishing";
import { useEffect, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import {
  ArrowLeftRight,
  Home,
  Castle,
  Route,
  ScrollText,
  Dices,
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
import "./catan-layout.css";
import { CatanDesertRegions, CatanPirate, CatanShip } from "./catan-seafarers";
import {
  CatanClothVillages,
  CatanClothStock,
  CatanClothChoice,
} from "./catan-cloth";
import {
  catanColorIndex,
  catanPieceColors,
  catanSeatColor,
} from "./catan-player-colors";
import {
  CatanPirateProgress,
  CatanFleetPath,
  CatanPirateMarkers,
  CatanEndAction,
} from "./catan-pirate-islands";
import { CatanPirateEffects } from "./catan-pirate-effects";
import {
  CatanWonderMarkers,
  CatanWondersPanel,
  CatanWondersStart,
} from "./catan-wonders";
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
  "#258bb4",
  "#a4a16a",
];

const terrainResourceKeys = ["wood", "brick", "wool", "grain", "ore"];
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
  bridge: [1, 2, 0, 0, 0],
  repair_road: [1, 1, 0, 0, 0],
  ship: [1, 0, 1, 0, 0],
  settlement: [1, 1, 1, 1, 0],
  city: [0, 0, 0, 2, 3],
  buy_dev: [0, 0, 1, 1, 1],
};
const total = (a: number[]) => a.reduce((n, x) => n + x, 0);
export const catanPhases: Record<string, string> = {
  catan_setup_settlement: "选择起始村庄的位置",
  catan_setup_city: "选择起始城市的位置",
  catan_setup_road: "在刚放置的建筑旁修路",
  catan_roll: "掷骰，生产资源",
  catan_turn: "交易、建造，或结束回合",
  catan_discard: "同时选择要弃置的资源",
  catan_robber: "选择强盗的新位置",
  catan_steal: "选择偷取资源的对手",
  catan_cloth_start: "选择初始强盗位置",
  catan_wonders_start: "选择初始强盗位置",
  catan_rivers_start: "选择沼泽中的强盗起点",
  catan_world_ports: "轮流放置随机港口",
  catan_world_fish: "轮流安放随机渔场",
  catan_cloth_steal: "选择偷取资源或布匹",
  catan_roads: "免费修建道路、船只或修复受损道路",
  catan_helper: "等待助手选择",
  catan_gold: "选择金矿出产的资源",
  catan_fish_replace: "选择一枚鱼筹码盲换，或保留现有筹码",
  catan_card_event: "完成事件牌选择，再进行生产",
  catan_fleet_reward: "选择击退海盗的奖励",
  catan_port: "安放领取的港口",
  catan_metropolis: "选择城市安放大都会",
  catan_aqueduct: "选择引水渠补偿资源",
  catan_pillage: "选择一座城市降级",
  catan_defender_reward: "选择一类进步牌作为防御奖励",
  catan_progress_discard: "选择超出上限的进步牌弃置",
  catan_progress_end: "结束行动前将进步手牌弃至四张",
  catan_knight_retreat: "为被驱逐的骑士选择退路",
  catan_guild_dues: "从展示的手牌中选择资源或商品",
  catan_commercial_harbor: "选择用于商业港交换的商品",
  catan_espionage: "从展示的进步牌中选择一张",
  catan_wedding: "选择赠送的资源或商品",
  catan_sabotage: "选择弃置的资源或商品",
  catan_diplomacy: "选择免费重放路线的位置",
  catan_treason_remove: "选择被叛变移除的骑士",
  catan_treason_place: "选择骑士等级和放置位置",
};
export const catanSeafarerPhases: Record<string, string> = {
  catan_setup_road: "在刚放置的村庄旁修路或造船",
  catan_roads: "放置免费的道路或船只",
  catan_robber: "选择移动强盗或海盗",
};
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
  const bank = g.bank.slice(0, 5);
  const due = Math.min(claim.count, total(bank)),
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
          <p>
            可以选择同种或不同的普通资源，不能领取商品。每人限时 120
            秒，超时自动选择。
          </p>
          {mine ? (
            <>
              <ResourcePicker
                label="金矿领取"
                values={take}
                onChange={setTake}
                limits={bank.map((n, i) =>
                  Math.min(n, take[i] + Math.max(0, due - picked)),
                )}
                assets={assets}
                disabled={busy}
              />
              <div className="catan-gold-stock">
                银行库存：
                {bank.map((n, i) => (
                  <span key={i}>
                    {catanNames[i]} {n}
                  </span>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={
                  busy || picked !== due || take.some((n, i) => n > bank[i])
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
function CatanFleetChoice({
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
    actor = g.seafarers?.pirateIslands?.raid?.rewards[0];
  const [collapsed, setCollapsed] = useState(false);
  const [color, setColor] = useState<number | null>(null);
  useEffect(() => {
    setCollapsed(false);
    setColor(null);
  }, [room.id, actor, g.rollId]);
  if (
    actor === undefined ||
    room.status !== "playing" ||
    room.game!.finished ||
    room.game!.phase !== "catan_fleet_reward"
  )
    return null;
  const mine =
    !room.spectating && actor === room.you && !g.players[actor].eliminated;
  return (
    <section className="catan-gold-choice" aria-label="海盗防守奖励">
      <header>
        <strong>
          {mine
            ? "击退海盗：选择一张资源"
            : `${room.seats[actor].name} 正在选择防守奖励`}
        </strong>
        <button
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          <p>奖励领取后继续结算本次掷骰。限时 120 秒，超时自动选择。</p>
          {mine && (
            <>
              <div
                className="catan-fleet-resources"
                role="group"
                aria-label="选择防守奖励资源"
              >
                {g.bank.map((n, i) => (
                  <button
                    key={i}
                    disabled={busy || n <= 0}
                    aria-pressed={color === i}
                    onClick={() => setColor(i)}
                  >
                    <CatanResource color={i} assets={assets} small />
                    <small>库存 {n}</small>
                  </button>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={busy || color === null || g.bank[color] <= 0}
                onClick={() =>
                  color !== null &&
                  void act({ type: "catan_fleet_reward", color })
                }
              >
                确认领取{color !== null ? ` ${catanNames[color]}` : ""}
              </button>
            </>
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
    hand = p?.resources || g.bank.map(() => 0);
  const city = g.citiesKnights;
  const cardCount = g.bank.length;
  const hexSize = g.hexSize || 62;
  const sea = g.seafarers;
  const riverImages = useMemo(() => catanRiverImages(g), [g]);
  const portLayout = useMemo(() => catanPortLayout(g), [g]);
  const pirates = sea?.pirateIslands;
  const devLabel = (i: number) =>
    pirates && i === 4 ? "胜利点卡 · 当作骑士" : devNames[i];
  const pieceScale = Math.max(0.64, hexSize / 62);
  const targetScore =
    g.fishing?.victoryTargets[you] ?? catanSavedVictoryTarget(g);
  const terrainNames = [
    ...catanNames,
    "沙漠",
    "海洋",
    "金矿",
    "未探索迷雾",
    "湖泊",
    "沼泽",
  ];
  const describeDev = (i: number) =>
    pirates && (i === 0 || i === 4)
      ? "将远征航线上最靠近起点的一艘普通船升级为战舰。不移动海盗、不累计骑士军队，也不获得胜利点。"
      : sea?.wonders && i === 4
        ? `自动计入你的私人分数。本剧本还需要已建奇迹等级领先所有对手才能以${targetScore}分获胜；或将奇迹建至4级直接获胜。`
        : i === 4
          ? `自动计入你的私人分数，达到${targetScore}点时在自己的回合获胜。`
          : sea?.cloth && i === 0
            ? "移动强盗并偷资源；建立村落贸易后也可移动海盗，选择偷资源或布匹。累计三名骑士可争夺最大骑士军队。"
            : sea && !sea.wonders && i === 0
              ? "移动强盗或海盗并随机偷取一张资源；累计三名骑士可争夺最大骑士军队。"
              : sea && i === 1
                ? "免费建造两条道路、两艘船，或各一；完成第一段后再放置第二段。"
                : devDescriptions[i];
  const playing = room.status === "playing" && !game.finished;
  const canPlay = playing && !room.spectating && you >= 0 && !p?.eliminated;
  const mine =
    canPlay &&
    game.turn === you &&
    !g.cardEvent &&
    fishResponder(room) === undefined;
  const eventMine = catanCardEventActor(room);
  const eventMode = catanCardEventMapMode(room);
  const portMine =
    canPlay &&
    (sea?.tribe?.pending?.player === you ||
      (game.phase === "catan_world_ports" && game.turn === you));
  const setup = g.setupStep < (g.setupLimit ?? 2 * g.players.length);
  const phase = game.phase;
  const [village, setVillage] = useState<number | null>(null);
  const [mode, setMode] = useState("");
  const [progress, setProgress] = useState<ProgressSelection | null>(null);
  const [wonderFocus, setWonderFocus] = useState<number | null>(null);
  const [chosen, setChosen] = useState<{ type: string; id: number } | null>(
    null,
  );
  const [helperPayment, setHelperPayment] = useState<number[] | null>(null);
  const [helperResource, setHelperResource] = useState(0);
  const [moveFrom, setMoveFrom] = useState<number | null>(null);
  const [dev, setDev] = useState<number | null>(null);
  const [give, setGive] = useState(() => Array(cardCount).fill(0));
  const [take, setTake] = useState(() => Array(cardCount).fill(0));
  const [monopoly, setMonopoly] = useState(0);
  const [goldGive, setGoldGive] = useState(0);
  const [goldTake, setGoldTake] = useState(0);
  useEffect(() => {
    setVillage(null);
    setWonderFocus(null);
    setMode("");
    setProgress(null);
    setHelperPayment(null);
    setMoveFrom(null);
    setChosen(null);
    setDev(null);
    setGoldGive(0);
    setGoldTake(0);
    setGive(Array(cardCount).fill(0));
    setTake(Array(cardCount).fill(0));
  }, [
    room.id,
    game.turn,
    game.round,
    phase,
    g.setupStep,
    g.fishing?.tokens.responder,
    room.you,
    room.spectating,
    cardCount,
    g.cardEvent?.kind,
    g.cardEvent?.players[0],
    g.cardEvent ? g.rollId : undefined,
    city?.pending?.players[0],
    city?.pending?.kind,
    city?.pending ? room.turnDeadline : undefined,
  ]);
  const progressHandKey = city?.players[you]?.progress?.join(",");
  useEffect(() => setProgress(null), [progressHandKey]);
  const progressMode =
    mine && progress && g.progressPlayable?.includes(progress.card)
      ? progressMapMode(progress.card)
      : "";
  const progressTargets =
    progress && progressMode ? progressMapTargets(g, you, progress) : [];
  const cityChoiceMine = canPlay && city?.pending?.players[0] === you;
  const cityChoiceMode =
    cityChoiceMine &&
    ["metropolis", "pillage", "knight_retreat", ...progressMapChoices].includes(
      city?.pending?.kind || "",
    )
      ? city!.pending!.kind
      : "";
  const effective =
    eventMode ||
    cityChoiceMode ||
    progressMode ||
    (phase === "catan_cloth_start" ||
    phase === "catan_wonders_start" ||
    phase === "catan_rivers_start"
      ? "robber_start"
      : phase === "catan_world_fish"
        ? "fish_ground"
        : phase === "catan_port" || phase === "catan_world_ports"
          ? "port"
          : phase === "catan_setup_city"
            ? "city"
            : phase === "catan_setup_settlement"
              ? "settlement"
              : phase === "catan_setup_road" || phase === "catan_roads"
                ? sea &&
                  (mode === "ship" ||
                    (mode !== "repair_road" &&
                      !g.legal.repairRoads?.length &&
                      !g.legal.roads.length &&
                      (g.legal.ships?.length || 0) > 0))
                  ? "ship"
                  : phase === "catan_roads" && g.legal.repairRoads?.length
                    ? "repair_road"
                    : "road"
                : phase === "catan_robber"
                  ? sea &&
                    (city?.chase === "pirate" ||
                      (!city?.chase && mode === "pirate"))
                    ? "pirate"
                    : "robber"
                  : mode);
  const affordable = (type: string) =>
    costs[type].every((n, c) => hand[c] >= n);
  const submit = async (a: Record<string, unknown>) => {
    await act(a);
    setHelperPayment(null);
    setMoveFrom(null);
    setMode("");
    setProgress(null);
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
  const revealedEvent = catanRevealedEvent(g);
  const liveNumber = revealedEvent?.production ?? total(g.dice);
  const discard = canPlay && phase === "catan_discard" && g.discardDue[you] > 0;
  const select = (type: string, id: number) => {
    if (
      (!mine &&
        !eventMine &&
        !cityChoiceMine &&
        !(type === "port" && portMine)) ||
      busy
    )
      return;
    if (type.startsWith("progress_") && progress) {
      setProgress(pickProgressTarget(g, you, progress, id));
      return;
    }
    if (
      (type === "helper_move" ||
        type === "move_ship" ||
        type === "knight_move") &&
      moveFrom === null
    ) {
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
  const cityVertices: Record<string, number[]> = {
    wall: cityWallSites(g, you),
    metropolis: cityMetropolisSites(g, you),
    pillage: g.legal.pillage || [],
    knight_recruit: g.legal.knightRecruit || [],
    knight_activate: g.legal.knightActivate || [],
    knight_promote: g.legal.knightPromote || [],
    knight_chase: g.legal.knightChase || [],
    knight_chase_pirate: g.legal.knightChasePirate || [],
    knight_retreat: g.legal.knightRetreat || [],
    treason_remove:
      city?.knights.filter((n) => n.owner === you).map((n) => n.vertex) || [],
    treason_place: g.treasonPlacements || [],
    knight_move:
      moveFrom === null
        ? Object.entries(g.knightMoves || {})
            .filter(([, v]) => v.length)
            .map(([key]) => Number(key))
        : g.knightMoves?.[moveFrom] || [],
  };
  const selectableVertex = (id: number) =>
    (mine || cityChoiceMine) &&
    ((effective === "progress_vertex" && progressTargets.includes(id)) ||
      (effective === "fish_ground" && !!g.legal.fishGrounds?.includes(id)) ||
      (effective === "settlement" && g.legal.settlements.includes(id)) ||
      (effective === "city" && g.legal.cities.includes(id)) ||
      (!!city && (cityVertices[effective] || []).includes(id)));
  return (
    <div
      className={`catan-board ${sea ? "catan-seafarers" : ""} ${pirates ? "catan-pirate-islands" : ""} ${sea?.wonders ? "catan-wonders" : ""} ${sea?.newWorld ? "catan-new-world" : ""} ${city ? "catan-cities-knights" : ""} ${progress ? "catan-progress-open" : ""} ${city || sea?.wonders || sea?.newWorld || g.fishing || g.rivers ? "catan-map-side-hand" : ""}`}
    >
      <section className="catan-map-panel">
        <div className="catan-map-toolbar">
          <span>
            <Flag size={14} /> {setup ? "起始建设" : `第 ${game.round} 轮`} ·
            {pirates
              ? `收复要塞且${targetScore}分获胜`
              : sea?.wonders
                ? `奇迹4级，或${targetScore}分且领先`
                : `${targetScore}分获胜`}
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
        <CatanHarborsStatus room={room} />
        <CatanFriendlyRobberStatus room={room} />
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
                  cloth: "卡坦布匹",
                  pirate_islands: "海盗群岛",
                  wonders: "卡坦奇迹",
                  new_world: "新世界",
                } as Record<string, string>
              )[sea.scenario] || "航海家"}
              {sea.fog && ` · 待探索 ${sea.fog.remaining} 格`}
              {sea.scenario === "desert" && " · 区域首次定居 +2分"}
            </span>
            {sea.wonders && wonderFocus !== null && (
              <button onClick={() => setWonderFocus(null)}>
                取消定位{g.wonderRules?.find((r) => r.id === wonderFocus)?.name}
              </button>
            )}
            {mine &&
              (phase === "catan_setup_road" ||
                phase === "catan_roads" ||
                (phase === "catan_robber" && !sea.wonders)) && (
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
                        (key === "pirate" &&
                          (!g.legal.pirate?.length ||
                            city?.chase === "robber")) ||
                        (key === "robber" &&
                          (!g.legal.robber?.length ||
                            city?.chase === "pirate")) ||
                        (key === "ship" && !g.legal.ships?.length) ||
                        (key === "road" && !g.legal.roads.length)
                      }
                      onClick={() => {
                        setMode(key);
                        setProgress(null);
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
                    setProgress(null);
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
                {riverImages.map((r, i) => (
                  <clipPath key={i} id={`catan-river-${i}`}>
                    {r.tiles.map((id) => (
                      <polygon key={id} points={poly(id)} />
                    ))}
                  </clipPath>
                ))}
              </defs>
              {riverImages.map((r, i) => {
                const outlet = g.edges[g.rivers!.map.channels[i].outlet],
                  a = g.vertices[outlet.a],
                  b = g.vertices[outlet.b];
                return (
                  <g
                    key={i}
                    clipPath={`url(#catan-river-${i})`}
                    pointerEvents="none"
                    aria-hidden="true"
                  >
                    {r.tiles.map((id) => (
                      <polygon
                        key={id}
                        points={poly(id)}
                        fill={catanColors[g.tiles[id].resource]}
                      />
                    ))}
                    {assets ? (
                      <image
                        href={`${assets}/catan/rivers/river-${r.kind}-v1.webp`}
                        width={r.width}
                        height={r.height}
                        transform={r.transform}
                      />
                    ) : (
                      <polyline
                        points={[
                          ...r.tiles.map(
                            (id) => `${g.tiles[id].x},${g.tiles[id].y}`,
                          ),
                          `${(a.x + b.x) / 2},${(a.y + b.y) / 2}`,
                        ].join(" ")}
                        fill="none"
                        stroke="#63c4dc"
                        strokeWidth={hexSize * 0.15}
                      />
                    )}
                  </g>
                );
              })}
              {g.tiles.map((t) => {
                const available =
                  (eventMine &&
                    effective === "robber_flees" &&
                    !!g.legal.fleeDeserts?.includes(t.id)) ||
                  (mine &&
                    ((effective === "progress_tile" &&
                      progressTargets.includes(t.id)) ||
                      (effective === "robber_start" &&
                        g.legal.robber?.includes(t.id)) ||
                      (effective === "robber" &&
                        (g.legal.robber
                          ? g.legal.robber?.includes(t.id)
                          : t.id !== g.robber &&
                            t.resource !== 6 &&
                            t.resource !== 8)) ||
                      (effective === "pirate" &&
                        !!g.legal.pirate?.includes(t.id)) ||
                      (effective === "helper_desert" &&
                        t.resource === 5 &&
                        (!sea?.cloth || sea.cloth.homeTiles.includes(t.id)))));
                const numbers = catanProductionNumbers(g, t.id);
                const riverImage = riverImages.find((r) =>
                  r.tiles.includes(t.id),
                );
                return (
                  <g
                    key={`${t.id}-${t.resource}`}
                    role={available ? "button" : undefined}
                    tabIndex={available ? 0 : undefined}
                    aria-label={`地块 ${t.id + 1} ${terrainNames[t.resource]} ${numbers.join("、")}${g.robber === t.id ? "，强盗所在" : ""}`}
                    className={`catan-hex terrain-${t.resource} ${available ? "selectable" : ""} ${(!revealedEvent || revealedEvent.productionStarted) && catanTileProducing(g, t.id, liveNumber) ? "producing" : ""}`}
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
                      fill={
                        riverImage ? "transparent" : catanColors[t.resource]
                      }
                    />
                    {assets && t.resource !== 8 && !riverImage && (
                      <image
                        href={
                          t.resource < 6
                            ? `${assets}/catan/terrain-${[...terrainResourceKeys, "desert"][t.resource]}-v1.webp`
                            : t.resource === 9
                              ? `${assets}/catan/fishing/lake${g.fishing?.map.lakes.find((l) => l.tile === t.id)?.numbers.length === 2 ? "-extended" : ""}-v1.webp`
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
                          [
                            "♣",
                            "▰",
                            "♧",
                            "❧",
                            "◆",
                            "☀",
                            "≈",
                            "◆",
                            "？",
                            "≈",
                            "≈",
                          ][t.resource]
                        }
                      </text>
                    )}
                    <polygon
                      className="hex-border"
                      points={poly(t.id)}
                      fill="none"
                    />
                    {t.number > 0 &&
                      numbers.map((number, i) => (
                        <g
                          key={number}
                          className={`catan-number ${number === 6 || number === 8 ? "red" : ""}`}
                          transform={`translate(${t.x + (i - (numbers.length - 1) / 2) * 33 * pieceScale},${t.y + 4}) scale(${pieceScale * (numbers.length > 1 ? 0.78 : 1)})`}
                        >
                          <circle r="19" />
                          <text textAnchor="middle">{number}</text>
                        </g>
                      ))}
                    <CatanFishLakeNumbers
                      g={g}
                      tile={t.id}
                      assets={assets}
                      total={liveNumber}
                    />
                    {g.robber === t.id && (
                      <g
                        className="catan-robber"
                        transform={`translate(${t.x + (numbers.length > 1 ? 0 : 18 * pieceScale)},${t.y - (numbers.length > 1 ? 42 : 20) * pieceScale}) scale(${pieceScale})`}
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
                    {!pirates && sea?.pirate === t.id && (
                      <g
                        transform={`translate(${t.x},${t.y}) scale(${pieceScale})`}
                      >
                        <CatanPirate assets={assets} />
                        <title>海盗封锁本海域船只</title>
                      </g>
                    )}
                    {progressMode === "progress_tile" &&
                      progress?.picks.includes(t.id) && (
                        <polygon
                          points={poly(t.id)}
                          className="catan-picked"
                          fill="none"
                        />
                      )}
                    {(chosen?.type === "robber_start" ||
                      chosen?.type === "pirate" ||
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
              <CatanFishingGrounds
                g={g}
                assets={assets}
                total={liveNumber}
                layer="artwork"
              />
              <CatanDesertRegions game={g} />
              <CatanFleetPath game={g} />
              <CatanTribeRewards game={g} assets={assets} />
              {portLayout.map(({ port, unclaimed, a, b, px, py, size }) => {
                return (
                  <g
                    key={port.edge}
                    className={`catan-port ${unclaimed ? "unclaimed" : ""}`}
                  >
                    <line x1={a.x} y1={a.y} x2={px} y2={py} />
                    <line x1={b.x} y1={b.y} x2={px} y2={py} />
                    {assets ? (
                      <image
                        href={`${assets}/catan/port-${port.resource < 0 ? "any" : terrainResourceKeys[port.resource]}-v1.webp`}
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
              <CatanFishingGrounds
                g={g}
                assets={assets}
                total={liveNumber}
                layer="numbers"
              />
              {sea && !pirates && !sea.wonders && sea.pirate === -1 && (
                <g
                  transform={
                    sea.newWorld
                      ? `translate(${mapMinX + 22},290)`
                      : `translate(340,${mapMaxY + 28})`
                  }
                >
                  <CatanPirate assets={assets} />
                  <title>海盗在外海</title>
                </g>
              )}
              {sea?.newWorld && g.robber === -1 && (
                <g
                  transform={`translate(${mapMinX + mapWidth - 24},320)`}
                  role="img"
                  aria-label="强盗在外框"
                >
                  {assets ? (
                    <image
                      href={`${assets}/catan/robber-v1.webp`}
                      x="-12"
                      y="-18"
                      width="24"
                      height="40"
                    />
                  ) : (
                    <text textAnchor="middle">强盗</text>
                  )}
                </g>
              )}
              <CatanMerchant game={g} assets={assets} />
              {g.edges.map((e) => {
                const a = g.vertices[e.a],
                  b = g.vertices[e.b],
                  ok =
                    effective === "earthquake"
                      ? eventMine && !!g.legal.earthquakeRoads?.includes(e.id)
                      : effective === "port"
                        ? portMine && !!g.legal.ports?.includes(e.id)
                        : effective === "diplomacy"
                          ? cityChoiceMine &&
                            !!g.diplomacyPlacements?.includes(e.id)
                          : mine &&
                            ((effective === "fish_road" &&
                              !!g.fishing?.legal.roads.includes(e.id)) ||
                              (effective === "fish_ship" &&
                                !!g.fishing?.legal.ships?.includes(e.id)) ||
                              (effective === "progress_edge" &&
                                progressTargets.includes(e.id)) ||
                              (effective === "bridge" &&
                                !!g.legal.bridges?.includes(e.id)) ||
                              (effective === "road" &&
                                g.legal.roads.includes(e.id)) ||
                              (effective === "repair_road" &&
                                !!g.legal.repairRoads?.includes(e.id)) ||
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
                                  : (
                                      g.helperRoadMoves?.[moveFrom] || []
                                    ).includes(e.id))));
                const picked =
                  (progressMode === "progress_edge" &&
                    !!progress?.picks.includes(e.id)) ||
                  (chosen?.type === effective && chosen.id === e.id) ||
                  ((effective === "helper_move" || effective === "move_ship") &&
                    moveFrom === e.id);
                return (
                  <g
                    key={e.id}
                    className={`catan-edge ${ok ? "selectable" : ""}`}
                    pointerEvents={ok ? undefined : "none"}
                    role={ok ? "button" : undefined}
                    tabIndex={ok ? 0 : undefined}
                    aria-label={`${e.bridge || g.rivers?.map.bridges.includes(e.id) ? "桥梁" : effective === "port" ? "港口" : e.ship || (e.owner < 0 && (effective === "ship" || effective === "fish_ship" || effective === "move_ship" || (effective === "diplomacy" && city?.pending?.ship))) ? (e.warship ? "战舰" : "船只") : "道路"}位置 ${e.id + 1}${e.owner >= 0 ? "，" + room.seats[e.owner].name + "已占领" : ""}${e.damaged ? "，已受损" : ""}`}
                    onClick={() => ok && select(effective, e.id)}
                    onKeyDown={(ev) => {
                      if (ok && (ev.key === "Enter" || ev.key === " ")) {
                        ev.preventDefault();
                        select(effective, e.id);
                      }
                    }}
                  >
                    {e.owner < 0 && g.rivers?.map.bridges.includes(e.id) && (
                      <line
                        className="catan-bridge-site"
                        x1={a.x + (b.x - a.x) * 0.24}
                        y1={a.y + (b.y - a.y) * 0.24}
                        x2={a.x + (b.x - a.x) * 0.76}
                        y2={a.y + (b.y - a.y) * 0.76}
                      />
                    )}
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
                          {e.bridge ? (
                            <CatanBridge
                              game={g}
                              seat={e.owner}
                              assets={assets}
                            />
                          ) : e.ship ? (
                            <CatanShip
                              assets={assets}
                              player={catanColorIndex(g, e.owner)}
                              warship={e.warship}
                            />
                          ) : (
                            <g transform={e.damaged ? "rotate(90)" : undefined}>
                              <rect
                                x="-22"
                                y="-5"
                                width="44"
                                height="10"
                                rx="2"
                                fill={catanSeatColor(g, e.owner)}
                                stroke="#523829"
                                strokeWidth="1.5"
                              />
                              <path d="M-20-3H20" stroke="#fff" opacity=".45" />
                              {e.damaged && (
                                <path
                                  d="M-3-5L2-1L-2 1L3 5"
                                  stroke="#fff7df"
                                  strokeWidth="3"
                                  fill="none"
                                />
                              )}
                            </g>
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
              <CatanPirateMarkers room={room} assets={assets} />
              <CatanWonderMarkers
                game={g}
                assets={assets}
                setup={setup}
                focus={wonderFocus}
              />
              <CatanWorldFishPreview
                room={room}
                assets={assets}
                vertex={chosen?.type === "fish_ground" ? chosen.id : null}
              />
              {g.vertices.map((v) => {
                const ok = selectableVertex(v.id),
                  picked =
                    (progressMode === "progress_vertex" &&
                      !!progress?.picks.includes(v.id)) ||
                    (chosen?.id === v.id &&
                      (chosen.type === "fish_ground" ||
                        chosen.type === "city" ||
                        chosen.type === "settlement" ||
                        !!cityActionNames[chosen.type]));
                return (
                  <g
                    key={v.id}
                    className={`catan-vertex ${ok ? "selectable" : ""}`}
                    pointerEvents={ok ? undefined : "none"}
                    transform={`translate(${v.x},${v.y}) scale(${pieceScale})`}
                    role={ok ? "button" : undefined}
                    tabIndex={ok ? 0 : undefined}
                    aria-label={`${effective === "fish_ground" ? "渔场凹角" : cityActionNames[effective] || "交点"} ${v.id + 1}${v.level > 0 ? "，" + (v.owner < 0 ? "中立" : room.seats[v.owner].name + "的") + (v.level === 2 ? "城市" : "村庄") : ""}`}
                    onClick={() => ok && select(effective, v.id)}
                    onKeyDown={(e) => {
                      if (ok && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        select(effective, v.id);
                      }
                    }}
                  >
                    {(ok ||
                      (effective === "knight_move" && moveFrom === v.id)) && (
                      <circle
                        r={picked || moveFrom === v.id ? 15 : 11}
                        className={
                          picked || moveFrom === v.id
                            ? "picked-vertex"
                            : "available-vertex"
                        }
                      />
                    )}
                    <g data-city-piece={v.id}>
                      <CatanCityWall game={g} vertex={v.id} assets={assets} />
                      {v.level > 0 &&
                        (assets ? (
                          <image
                            href={`${assets}/catan/${v.level === 2 ? "city" : "settlement"}-${catanPieceColors[catanColorIndex(g, v.owner)]}-v1.webp`}
                            x={v.level === 2 ? -20 : -15}
                            y="-20"
                            width={v.level === 2 ? 40 : 30}
                            height="34"
                            className={
                              g.players[v.owner]?.eliminated
                                ? "eliminated-piece"
                                : ""
                            }
                          />
                        ) : (
                          <path
                            className={
                              g.players[v.owner]?.eliminated
                                ? "eliminated-piece"
                                : ""
                            }
                            d={
                              v.level === 2
                                ? "M-14 10V-5L-5-13L4-5V0H13V10Z"
                                : "M-10 10V-3L0-12L10-3V10Z"
                            }
                            fill={catanSeatColor(g, v.owner)}
                            stroke="#523829"
                            strokeWidth="1.8"
                          />
                        ))}
                      <CatanCityPiece game={g} vertex={v.id} assets={assets} />
                    </g>
                    {ok && <circle r="19" fill="transparent" />}
                  </g>
                );
              })}
              <CatanClothVillages
                room={room}
                assets={assets}
                inspect={setVillage}
              />
              {pirates && <CatanPirateEffects room={room} assets={assets} />}
            </svg>
          </div>
        </div>
        <div className="catan-map-hint">
          {fishResponder(room) !== undefined
            ? canPlay && fishResponder(room) === you
              ? "请在捕鱼面板换筹码或保留 · 可收起面板查看地图"
              : "等待鱼筹码选择 · 滚轮缩放 · 按住拖动"
            : ["fish_road", "fish_ship"].includes(effective) && mine
              ? `点击亮起的${effective === "fish_ship" ? "船只位置" : "道路"}，再到捕鱼面板确认支付`
              : progressMode
                ? "点击地图上亮起的目标，再在进步牌面板确认 · 滚轮缩放 · 按住拖动"
                : cityChoiceMine && cityChoiceMode
                  ? `请在地图上选择${cityActionNames[cityChoiceMode] || "目标"}位置，再确认 · 可收起选择面板`
                  : phase === "catan_cloth_start" ||
                      phase === "catan_wonders_start" ||
                      phase === "catan_rivers_start"
                    ? mine
                      ? `点击亮起的${g.rivers ? "沼泽" : sea?.wonders ? "沙漠" : "12号地块"}，再确认强盗起点`
                      : "等待先手选择强盗起点 · 可缩放拖动"
                    : phase === "catan_cloth_steal"
                      ? "请在海盗面板选择对手与物品 · 可收起查看地图"
                      : phase === "catan_port" || phase === "catan_world_ports"
                        ? portMine
                          ? "选择亮起的海岸位置，再确认安放港口 · 可收起面板查看地图"
                          : "等待港口安放 · 滚轮缩放 · 按住拖动"
                        : phase === "catan_world_fish"
                          ? mine
                            ? "选择亮起的凹角，预览后确认渔场 · 可收起面板查看地图"
                            : "等待渔场安放 · 滚轮缩放 · 按住拖动"
                          : phase === "catan_card_event"
                            ? eventMine
                              ? "请完成事件牌选择 · 可收起面板查看地图"
                              : "等待事件牌回应 · 滚轮缩放 · 按住拖动"
                            : phase === "catan_gold"
                              ? canPlay &&
                                g.goldPending?.claims[0]?.player === you
                                ? "请在金矿面板领取资源 · 可收起面板查看地图"
                                : "等待金矿资源选择 · 滚轮缩放 · 按住拖动"
                              : mine
                                ? effective === "helper_move" ||
                                  effective === "move_ship"
                                  ? moveFrom === null
                                    ? effective === "move_ship"
                                      ? "选择要移动的己方末端旧船"
                                      : "选择要迁移的己方末端道路"
                                    : "选择亮起的新位置，再确认移动"
                                  : effective === "ship"
                                    ? "点击虚线选择船只位置，再确认建造"
                                    : effective === "pirate"
                                      ? "选择另一块海洋，或将海盗移至外海"
                                      : effective === "repair_road"
                                        ? "点击受损道路，再确认修复"
                                        : effective === "road"
                                          ? "点击虚线选择道路，再确认建造"
                                          : effective === "settlement" ||
                                              effective === "city"
                                            ? "点击亮起的交点，再确认建造"
                                            : effective === "robber" ||
                                                effective === "helper_desert"
                                              ? "点击地块选择强盗的新位置"
                                              : "选择右侧行动 · 滚轮缩放 · 按住拖动"
                                : "滚轮缩放 · 按住拖动 · 等待其他玩家行动"}
        </div>
      </section>
      <aside className="catan-actions">
        <CatanCardEventSummary room={room} assets={assets} />
        {!revealedEvent && (
          <div className="catan-dice" key={g.rollId}>
            <span
              className={`${g.rollId ? "rolled" : ""} ${city ? "catan-red-die" : ""}`}
            >
              {g.dice[0] || "—"}
            </span>
            <span className={g.rollId ? "rolled" : ""}>{g.dice[1] || "—"}</span>
            {city && (
              <span className="catan-event-die" title="事件骰">
                {city.eventDie < 0 ? (
                  "—"
                ) : city.eventDie >= 3 ? (
                  <Ship size={20} />
                ) : (
                  ["科学", "贸易", "政治"][city.eventDie]
                )}
              </span>
            )}

            <div>
              <b>{g.rollId ? `点数 ${liveNumber}` : "等待掷骰"}</b>
              <small>
                {g.paired?.second && phase === "catan_turn"
                  ? "② 配对行动 · 不掷骰、不自由交易"
                  : phase === "catan_discard"
                    ? "所有人同时弃牌"
                    : (sea &&
                        !(sea.wonders && phase === "catan_robber") &&
                        catanSeafarerPhases[phase]) ||
                      catanPhases[phase] ||
                      "本局已结束"}
              </small>
            </div>
          </div>
        )}
        <CatanFishingPanel
          key={`${room.id}:${game.phase}:${game.turn}:${game.round}:${g.fishing?.tokens.responder}:${room.you}:${room.spectating}`}
          room={room}
          act={submit}
          busy={busy}
          assets={assets}
          chosen={chosen}
          onMode={(next) => {
            setMode(next);
            setChosen(null);
            setProgress(null);
            setHelperPayment(null);
            setDev(null);
            setMoveFrom(null);
          }}
          showMap={() =>
            viewport.current?.scrollIntoView({
              block: "center",
              behavior: "smooth",
            })
          }
        />
        <CatanProgressHand
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          selection={progress}
          onChange={(next) => {
            if (next?.card !== progress?.card) {
              setMode("");
              setMoveFrom(null);
              setChosen(null);
              setDev(null);
              setHelperPayment(null);
            }
            setProgress(next);
          }}
        />
        <CatanTradePowers room={room} act={act} busy={busy} assets={assets} />
        <CatanCardEventChoice
          key={`${room.id}:${g.rollId}:${g.cardEvent?.kind}:${g.cardEvent?.players[0]}:${room.you}`}
          room={room}
          act={submit}
          busy={busy}
          assets={assets}
          chosen={chosen}
        />
        <CatanCityOverview room={room} assets={assets} />
        {city && <CatanCityEffects room={room} assets={assets} />}
        <CatanCityChoice
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          chosen={chosen}
        />
        <CatanCityActions
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          mode={mode}
          selectMode={(next) => {
            setMode(next);
            setProgress(null);
            setMoveFrom(null);
            setChosen(null);
            setDev(null);
            setHelperPayment(null);
          }}
        />
        <CatanWondersPanel
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          locate={(id) => {
            setWonderFocus(id);
            zoomAt(1);
            viewport.current?.scrollIntoView({ block: "center" });
          }}
        />
        {mine && phase === "catan_roll" && (
          <button
            className="primary wide"
            disabled={busy}
            onClick={() => void submit({ type: "catan_roll" })}
          >
            <Dices size={18} /> 掷骰
          </button>
        )}
        {mine &&
          (phase === "catan_turn" || phase === "catan_roads") &&
          !!g.legal.repairRoads?.length && (
            <button
              className="catan-repair-action"
              disabled={
                busy || (phase !== "catan_roads" && !affordable("repair_road"))
              }
              onClick={() => {
                setMode("repair_road");
                setChosen(null);
                setProgress(null);
                setDev(null);
                setHelperPayment(null);
              }}
            >
              修复受损道路{" "}
              {phase === "catan_roads" ? (
                `· 免费次数剩余 ${g.freeRoads}`
              ) : (
                <Bundle values={costs.repair_road} assets={assets} />
              )}
            </button>
          )}
        {mine && phase === "catan_turn" && (
          <div className="catan-build-menu">
            {(city
              ? (["road", "settlement", "city"] as const)
              : sea
                ? (["road", "ship", "settlement", "city", "buy_dev"] as const)
                : g.rivers
                  ? ([
                      "road",
                      "bridge",
                      "settlement",
                      "city",
                      "buy_dev",
                    ] as const)
                  : (["road", "settlement", "city", "buy_dev"] as const)
            ).map((key) => {
              const Icon = {
                road: Route,
                bridge: Route,
                ship: Ship,
                settlement: Home,
                city: Castle,
                buy_dev: ScrollText,
              }[key];
              const label = {
                road: "修建道路",
                bridge: "建造桥梁",
                ship: "建造船只",
                settlement: "建造村庄",
                city: "升级城市",
                buy_dev: "购买发展卡",
              }[key];
              const available =
                key === "buy_dev"
                  ? g.devRemaining > 0
                  : key === "bridge"
                    ? !!g.legal.bridges?.length
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
                    setProgress(null);
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
                  setProgress(null);
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
                setProgress(null);
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
            <CatanEndAction
              key={`${room.id}:${game.turn}:${game.round}`}
              room={room}
              busy={busy}
              end={() => submit({ type: "catan_end" })}
            />
          </div>
        )}
        {chosen &&
          !progressMapChoices.includes(chosen.type) &&
          !["earthquake", "robber_flees", "fish_road", "fish_ship"].includes(
            chosen.type,
          ) &&
          chosen.type !== "port" &&
          chosen.type !== "fish_ground" &&
          chosen.type !== "robber_start" &&
          (mine || cityChoiceMine) && (
            <section className="catan-confirm" aria-label="确认行动">
              <strong>
                {
                  (
                    {
                      ...cityActionNames,
                      helper_move: "迁移道路",
                      ship: "建造船只",
                      move_ship: "移动船只",
                      pirate: chosen.id === -1 ? "将海盗移至外海" : "移动海盗",
                      helper_desert: "将强盗赶回沙漠",
                      road: "修建道路",
                      repair_road: "修复受损道路",
                      bridge: "建造桥梁",
                      settlement: "建造村庄",
                      city:
                        phase === "catan_setup_city"
                          ? "放置起始城市"
                          : "升级城市",
                      robber: "移动强盗",
                      buy_dev: "购买发展卡",
                    } as Record<string, string>
                  )[chosen.type]
                }
                {chosen.type !== "buy_dev" &&
                  chosen.id >= 0 &&
                  ` #${chosen.id + 1}`}
              </strong>
              {(costs[chosen.type] || cityActionCosts[chosen.type]) &&
                !setup &&
                phase !== "catan_roads" && (
                  <Bundle
                    values={
                      helperPayment ||
                      costs[chosen.type] ||
                      cityActionCosts[chosen.type]
                    }
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
                    setProgress(null);
                  }}
                >
                  取消
                </button>
                <button
                  className="primary"
                  disabled={
                    busy ||
                    (chosen.type === "bridge" &&
                      (!g.legal.bridges?.includes(chosen.id) ||
                        !affordable("bridge")))
                  }
                  onClick={() =>
                    void submit({
                      type:
                        chosen.type === "helper_move" ||
                        chosen.type === "helper_desert"
                          ? "catan_helper"
                          : chosen.type === "knight_chase_pirate"
                            ? "catan_knight_chase"
                            : "catan_" + chosen.type,
                      choice:
                        chosen.type === "helper_desert"
                          ? "desert"
                          : chosen.type === "knight_chase_pirate"
                            ? "pirate"
                            : undefined,
                      color:
                        chosen.type === "helper_desert"
                          ? helperResource
                          : undefined,
                      skill: helperPayment ? "helper" : undefined,
                      tokens: helperPayment || undefined,
                      target:
                        chosen.type === "knight_move" ||
                        chosen.type === "helper_move" ||
                        chosen.type === "move_ship"
                          ? chosen.id
                          : undefined,
                      edge:
                        chosen.type === "helper_move" ||
                        chosen.type === "move_ship"
                          ? moveFrom
                          : chosen.id,
                      vertex:
                        chosen.type === "knight_move" ? moveFrom : chosen.id,
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
            剩余免费建设或修路次数 {g.freeRoads}。
            {g.legal.roads.length === 0 &&
              !g.legal.ships?.length &&
              !g.legal.repairRoads?.length && (
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
            {pirates && <p>可从任一有资源的对手偷取，也可放弃。</p>}
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
        {mine && pirates && phase === "catan_steal" && (
          <button
            className="outline"
            disabled={busy}
            onClick={() => void submit({ type: "catan_skip_steal" })}
          >
            放弃偷取
          </button>
        )}
        {phase === "catan_discard" && (
          <section className="catan-confirm">
            <strong>
              掷出 7 · 超过{city ? "个人弃牌上限" : "七张"}需弃一半
            </strong>
            {city && !room.spectating && (
              <p>
                你的弃牌上限：{cityDiscardLimit(g, you)} 张（含资源与商品）。
              </p>
            )}
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
              limits={catanCardSupply(cardCount, !!g.options?.fiveSix)}
              assets={assets}
              disabled={busy}
            />
            {g.rivers && !g.paired?.second && (
              <CatanGoldTradePicker
                give={goldGive}
                take={goldTake}
                giveLimit={g.rivers.gold[you]}
                takeLimit={Math.max(
                  0,
                  ...g.rivers.gold.filter(
                    (_, i) => i !== you && !g.players[i].eliminated,
                  ),
                )}
                onChange={(give, take) => {
                  setGoldGive(give);
                  setGoldTake(take);
                }}
                disabled={busy}
              />
            )}
            <small>
              {catanBankReason(give, take, hand, g.bank, p.rates) ||
                "可按所选比例与银行交换"}
              <br />
              我的银行比例：
              {p.rates.map((n, c) => `${catanCardNames[c]} ${n}:1`).join(" · ")}
            </small>
            <div className="catan-trade-buttons">
              <button
                className="outline"
                disabled={
                  busy ||
                  goldGive > 0 ||
                  goldTake > 0 ||
                  !!catanBankReason(give, take, hand, g.bank, p.rates)
                }
                title={catanBankReason(give, take, hand, g.bank, p.rates)}
                onClick={() => void submit({ type: "catan_bank", give, take })}
              >
                与银行交换
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  !!g.paired?.second ||
                  !(total(give) + goldGive) ||
                  !(total(take) + goldTake) ||
                  give.some((n, c) => n > hand[c]) ||
                  goldGive > (g.rivers?.gold[you] ?? 0)
                }
                onClick={() =>
                  void submit({
                    type: "catan_trade_offer",
                    give,
                    take,
                    goldGive,
                    goldTake,
                  })
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
            {!!g.trade.goldGive && (
              <CatanCoins count={g.trade.goldGive} assets={assets} />
            )}
            <small>换取</small>
            <Bundle values={g.trade.take} assets={assets} />
            {!!g.trade.goldTake && (
              <CatanCoins count={g.trade.goldTake} assets={assets} />
            )}
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
                      g.trade.take.some((n, c) => n > hand[c]) ||
                      (g.trade.goldTake ?? 0) > (g.rivers?.gold[you] ?? 0)
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
            <strong>{devLabel(dev)}</strong>
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
        <CatanNewWorldPortChoice
          room={room}
          assets={assets}
          busy={busy}
          act={act}
          edge={chosen?.type === "port" ? chosen.id : null}
          clear={() => setChosen(null)}
        />
        <CatanWorldFishChoice
          room={room}
          assets={assets}
          busy={busy}
          act={act}
          vertex={chosen?.type === "fish_ground" ? chosen.id : null}
          clear={() => setChosen(null)}
        />
        <CatanTribePortChoice
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          edge={chosen?.type === "port" ? chosen.id : null}
          clear={() => setChosen(null)}
        />
        <CatanTribeStock room={room} assets={assets} />
        <CatanClothStock room={room} assets={assets} />
        <CatanRiverBank room={room} busy={busy} act={act} assets={assets} />
        <CatanRiverStart
          room={room}
          busy={busy}
          act={act}
          selectedTile={chosen?.type === "robber_start" ? chosen.id : null}
          clearTile={() => setChosen(null)}
        />
        <CatanWondersStart
          room={room}
          busy={busy}
          act={act}
          selectedTile={chosen?.type === "robber_start" ? chosen.id : null}
          clearTile={() => setChosen(null)}
        />
        <CatanClothChoice
          room={room}
          assets={assets}
          busy={busy}
          act={act}
          selectedTile={chosen?.type === "robber_start" ? chosen.id : null}
          clearTile={() => setChosen(null)}
          village={village}
          closeVillage={() => setVillage(null)}
          colors={g.players.map((_, i) => catanSeatColor(g, i))}
        />
        <CatanHelpers
          room={room}
          act={act}
          busy={busy}
          assets={assets}
          onBuild={(kind, payment) => {
            setMode(kind);
            setProgress(null);
            setHelperPayment(payment);
            setMoveFrom(null);
            setDev(null);
            setChosen(kind === "buy_dev" ? { type: kind, id: 0 } : null);
          }}
          onDesert={(color) => {
            setHelperResource(color);
            setMode("helper_desert");
            setProgress(null);
            setMoveFrom(null);
            setHelperPayment(null);
            setChosen(null);
            setDev(null);
          }}
          onMove={() => {
            setMode("helper_move");
            setProgress(null);
            setMoveFrom(null);
            setHelperPayment(null);
            setChosen(null);
            setDev(null);
          }}
        />
        <CatanPirateProgress room={room} />
        <section className="catan-bank">
          <h3>
            {city ? "资源与商品银行" : "资源银行"}{" "}
            {!city && <small>发展卡剩余 {g.devRemaining}</small>}
          </h3>
          <Bundle values={g.bank} assets={assets} showZero />
        </section>
        <CatanFleetChoice room={room} act={act} busy={busy} assets={assets} />
        {!pirates && (
          <div className="catan-awards">
            {!sea?.cloth && (
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
            )}
            {!city && (
              <span>
                <Shield size={16} />
                最大骑士军队{" "}
                <b>
                  {g.armyOwner < 0 ? "至少 3 名" : room.seats[g.armyOwner].name}
                </b>
                <small>+2 分</small>
              </span>
            )}
          </div>
        )}
      </aside>
      {p && !room.spectating && (
        <section className="catan-hand">
          <header>
            <h3>
              {city ? "你的资源与商品" : "你的资源"}{" "}
              <small>{p.resourceCount} 张</small>
            </h3>
            <span>
              {g.rivers && `桥梁 ${p.bridgesLeft}/3 · `}道路 {p.roadsLeft}/15{" "}
              {sea && `· 船只 ${p.shipsLeft}/15 `}· 村庄 {p.settlementsLeft}/5 ·
              城市 {p.citiesLeft}/4 可建
            </span>
          </header>
          <div className="catan-hand-resources">
            {hand.map((n, c) => (
              <CatanResource key={c} color={c} count={n} assets={assets} />
            ))}
          </div>
          {!city && (
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
                          (i === 4 && !pirates) ||
                          (!!pirates &&
                            (i === 0 || i === 4) &&
                            !pirates.fortresses[you]?.route.some(
                              (id) => !g.edges[id].warship,
                            )) ||
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
                          <img
                            src={`${assets}/catan/dev-${i}-v1.webp`}
                            alt=""
                          />
                        )}
                        <span>
                          {devLabel(i)} <b>{n}</b>
                        </span>
                        {(p.newDev?.[i] || 0) > 0 && (i !== 4 || !!pirates) && (
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
          )}
        </section>
      )}
    </div>
  );
}
