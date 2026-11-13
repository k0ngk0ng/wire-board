import { geologyLabel } from "./geology-art";
import {
  Heart,
  Waves,
  Coins,
  Timer,
  Users,
  Skull,
  Trophy,
  CircleX,
  CloudFog,
  Snowflake,
  Sparkles,
  Hammer,
  Pause,
  Play,
  Undo2,
  BriefcaseMedical,
  Store,
  ListTodo,
  Info,
  Gift,
  Shield,
  Sun,
} from "lucide-react";
import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { Battlefield } from "./Canvas";
import { MerchantShop } from "./MerchantShop";
import { SkillBadge } from "./SkillBadge";
import type { CombatStatus } from "./combat-timeline";
import { assetURL, pickSound } from "./presentation";
import type { Event, MapData, Objective, View, WorldChoice } from "./types";

function compactAmount(value: number): string {
  if (value >= 100000000) return `${Math.floor(value / 10000000) / 10}亿`;
  if (value >= 10000) return `${Math.floor(value / 1000) / 10}万`;
  return String(value);
}

const towerNames = {
  arrow: "箭塔",
  cannon: "炮塔",
  frost: "冰塔",
  arcane: "法师塔",
};
const phaseNames = {
  prepare: "整备防线",
  battle: "敌潮来袭",
  paused: "守望暂停",
  won: "全队胜利",
  lost: "挑战失败",
};
const difficultyNames = { easy: "简单", normal: "普通", hard: "困难" };
const priorityNames = {
  first: "最接近星核",
  strong: "最强敌人",
  support: "支援敌人",
};
const objectiveStatus = {
  available: "待承接",
  active: "守护中",
  completed: "已完成",
  failed: "未守住",
  skipped: "已跳过",
};

function RoutePreview({ map, label }: { map: MapData; label: string }) {
  const paths = map.routes ?? [{ id: "main", points: map.path, active: true }];
  const base = map.base ?? map.path.at(-1);
  return (
    <svg
      className="sf-route-preview"
      viewBox={`-60 -60 ${map.width + 120} ${map.height + 120}`}
      role="img"
      aria-label={`${label}道路预览`}
    >
      <rect width={map.width} height={map.height} rx="12" fill="#d9e2be" />
      {map.regions?.map((region) => (
        <rect
          key={region.id}
          x={region.x}
          y={region.y}
          width={region.width}
          height={region.height}
          fill={
            region.biome === "canyon"
              ? "#d9c5a1"
              : region.biome === "river"
                ? "#bcd6d0"
                : region.biome === "snow"
                  ? "#dce8e3"
                  : region.biome === "marsh"
                    ? "#b7c9b3"
                    : region.biome === "ruins"
                      ? "#d3cec3"
                      : "#c3d5ac"
          }
          stroke="#869973"
          strokeWidth="3"
        />
      ))}
      {paths.map((route) => (
        <polyline
          key={route.id}
          points={route.points.map((p) => `${p.x},${p.y}`).join(" ")}
          fill="none"
          stroke={route.active ? "#856544" : "#a49b81"}
          strokeWidth="13"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeDasharray={route.active ? undefined : "18 12"}
        />
      ))}
      {map.pads.map((pad) => (
        <circle
          key={pad.id}
          cx={pad.x}
          cy={pad.y}
          r="9"
          fill={pad.terrain === "highground" ? "#825b30" : "#fff5d2"}
          stroke="#485f3f"
          strokeWidth="3"
        />
      ))}
      {paths.map(
        (route) =>
          route.points[0] && (
            <circle
              key={route.id}
              cx={route.points[0].x}
              cy={route.points[0].y}
              r="14"
              fill={route.active ? "#a74631" : "#756e60"}
              stroke="#fff9df"
              strokeWidth="4"
            />
          ),
      )}
      {base && (
        <path
          d={`M ${base.x} ${base.y - 22} l 8 14 16 3 -12 11 2 17 -14 -8 -14 8 2 -17 -12 -11 16 -3 Z`}
          fill="#ffe7a0"
          stroke="#765224"
          strokeWidth="3"
        />
      )}
    </svg>
  );
}

function ChoicePanel({
  choice,
  seats,
  you,
  disabled,
  paused,
  assets,
  serverNow,
  send,
}: {
  choice: WorldChoice;
  seats: { name: string }[];
  you: number;
  disabled: boolean;
  paused: boolean;
  assets: string;
  serverNow: number;
  send: (type: string, extra?: Record<string, unknown>) => Promise<void>;
}) {
  const selected = choice.votes[you];
  const route = choice.kind === "route";
  const seconds = Math.max(0, Math.ceil((choice.deadline - serverNow) / 1000));
  return (
    <section
      className="sf-world-choice"
      aria-label={route ? "战场扩展投票" : "团队成长投票"}
    >
      <div className="sf-world-heading">
        <h3>{choice.title}</h3>
        <span>
          {paused
            ? "恢复后继续选择"
            : seconds > 0
              ? `${seconds} 秒内选择`
              : "正在确定结果"}
        </span>
      </div>
      <p>
        全队共同选择。全员投票后按多数确定；平票选靠前的方案，超时按已有票数确定。确定后重新开始整备倒计时。
      </p>
      <div className="sf-choice-grid">
        {choice.options.map((option) => {
          const voters = seats
            .filter((_, i) => choice.votes[i] === option.id)
            .map((seat) => seat.name);
          return (
            <article
              className={`sf-choice-card${selected === option.id ? " sf-choice-selected" : ""}`}
              key={option.id}
            >
              {option.map && (
                <RoutePreview map={option.map} label={option.label} />
              )}
              <div className="sf-choice-title">
                {route && (
                  <Sprite name={`choice-${option.id}`} base={assets} world />
                )}
                <h4>{option.label}</h4>
              </div>
              <p>{option.description}</p>
              {option.map && (
                <p className="sf-choice-facts">
                  战场面积{" "}
                  {(
                    (option.map.width * option.map.height) /
                    (960 * 640)
                  ).toFixed(1)}{" "}
                  倍 · {option.map.pads.length} 个建造点 ·{" "}
                  {option.map.routes?.length ?? 1} 条道路 ·{" "}
                  {
                    option.map.pads.filter((p) => p.terrain === "highground")
                      .length
                  }{" "}
                  处高地
                </p>
              )}
              <p className="sf-voters">
                {voters.length} 票
                {voters.length ? ` · ${voters.join("、")}` : " · 尚无人选择"}
              </p>
              <button
                data-sf-route-choice={route ? option.id : undefined}
                data-sf-boon-choice={route ? undefined : option.id}
                aria-pressed={selected === option.id}
                disabled={disabled}
                onClick={() =>
                  void send(route ? "route_vote" : "boon_vote", {
                    choice: option.id,
                  })
                }
              >
                {selected === option.id ? "已选择" : `选择${option.label}`}
              </button>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function ObjectiveCard({
  balanced,
  objective,
  enabled,
  finished,
  wave,
  completedWaves,
  routeLabel,
  route,
  send,
}: {
  balanced?: boolean;
  objective: Objective;
  enabled: boolean;
  finished: boolean;
  wave: number;
  completedWaves: number;
  routeLabel?: string;
  route?: MapData["path"];
  send: (type: string, extra?: Record<string, unknown>) => Promise<void>;
}) {
  const active = objective.status === "active";
  const total = Math.max(1, objective.endWave - objective.startWave + 1);
  const progress = Math.max(
    0,
    Math.min(total, completedWaves - objective.startWave + 1),
  );
  const distance =
    route?.reduce(
      (sum, point, i) =>
        i === 0
          ? 0
          : sum +
            Math.hypot(point.x - route[i - 1].x, point.y - route[i - 1].y),
      0,
    ) ?? 0;
  const convoyProgress =
    distance > 0
      ? Math.min(100, Math.round((objective.progress / distance) * 100))
      : 0;
  return (
    <article
      className={`sf-objective-card sf-objective-${objective.status}`}
      data-sf-objective-state={objective.status}
    >
      <div>
        <div>
          <h4>{objective.label}</h4>
          {balanced && routeLabel && <small>{routeLabel}</small>}
        </div>
        <span>
          {finished && objective.status === "available"
            ? "本局未承接"
            : objectiveStatus[objective.status]}
        </span>
      </div>
      <p>{objective.description}</p>
      {!balanced && (
        <p className="sf-objective-reward">
          奖励：
          {objective.kind === "mine"
            ? `每人 ${objective.reward} 金币`
            : objective.kind === "beacon"
              ? `全队防御塔永久增加 ${balanced ? 5 : 8}% 射程`
              : `每人 ${objective.reward} 金币，星核恢复 4 点生命`}
        </p>
      )}
      {active && (
        <p className="sf-objective-progress">
          {objective.kind === "convoy" || objective.kind === "merchant"
            ? `护送进度 ${convoyProgress}% · 抵达星核后领取奖励`
            : `已守住 ${progress} / ${total} 波 · 守到第 ${objective.endWave} 波${wave === objective.endWave ? "，最后一波" : ""}`}
        </p>
      )}
      {(balanced || objective.status !== "available") &&
        objective.maxHP > 0 && (
          <div
            className="sf-objective-health"
            data-sf-objective-health={objective.id}
          >
            <span>
              {objective.kind === "convoy" || objective.kind === "merchant"
                ? "补给队生命"
                : "据点生命"}{" "}
              {balanced
                ? Number(Math.max(0, objective.hp).toFixed(2))
                : Math.ceil(Math.max(0, objective.hp))}{" "}
              / {Number(objective.maxHP.toFixed(2))}
            </span>
            <div
              className="sf-hp-track"
              role="meter"
              aria-label={`${objective.label}生命`}
              aria-valuemin={0}
              aria-valuemax={objective.maxHP}
              aria-valuenow={Math.max(0, objective.hp)}
            >
              <i
                style={{
                  width: `${Math.max(0, (objective.hp / objective.maxHP) * 100)}%`,
                }}
              />
            </div>
          </div>
        )}
      {objective.status === "failed" && (
        <p>
          {objective.hp <= 0
            ? "目标被摧毁，本次任务已结束。"
            : "本局结束时未完成目标，本次任务已结束。"}
          {!!objective.refunded &&
            `已向付款人退还 ${objective.refunded} 金币。`}
        </p>
      )}
      {objective.status === "completed" && <p>奖励已发放给全队。</p>}
      {objective.status === "available" && !finished && (
        <button
          data-sf-objective={objective.id}
          disabled={!enabled}
          onClick={() => void send("objective", { objectiveId: objective.id })}
        >
          承接 · {objective.cost} 金币
        </button>
      )}
    </article>
  );
}
function BaseRepair({
  quote,
  hp,
  maxHP,
  disabled,
  send,
}: {
  quote: NonNullable<View["controls"]["repair"]>;
  hp: number;
  maxHP: number;
  disabled: boolean;
  send: (type: string, extra?: Record<string, unknown>) => Promise<void>;
}) {
  const [amount, setAmount] = useState(1);
  const selected = Math.min(Math.max(1, amount), Math.max(1, quote.maxAmount));
  const enabled = quote.enabled && !disabled;
  return (
    <section
      className="sf-repair-panel"
      data-sf-repair-panel
      aria-label="星核修复"
    >
      <div>
        <h3>星核修复</h3>
        <span>
          ♥ {hp}/{maxHP} · 本波全队剩余 {quote.remaining}/{quote.limit} 点
        </span>
      </div>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (enabled) void send("repair_base", { amount: selected });
        }}
      >
        <label>
          修复生命
          <input
            data-sf-repair-amount
            aria-label="修复生命值"
            type="number"
            inputMode="numeric"
            min={1}
            max={Math.max(1, quote.maxAmount)}
            step={1}
            value={selected}
            disabled={!enabled}
            onChange={(event) => setAmount(Number(event.target.value) || 1)}
          />
        </label>
        <button data-sf-repair type="submit" disabled={!enabled}>
          修复 {selected} 点 · {selected * quote.unitCost} 金币
        </button>
      </form>
    </section>
  );
}
function Sprite({
  name,
  base,
  world = false,
}: {
  name: string;
  base: string;
  world?: boolean;
}) {
  const url = (base: string) =>
    world
      ? `${base.replace(/\/$/, "")}/starfort/world-v2/${name}.webp`
      : assetURL(base, `${name}.webp`);
  return (
    <img
      className="sf-asset"
      src={url(base)}
      alt=""
      aria-hidden="true"
      onError={(e) => {
        const local = url("");
        if (e.currentTarget.getAttribute("src") !== local)
          e.currentTarget.src = local;
      }}
    />
  );
}

export function Board({
  onReady,
  onLoadError,
  room,
  data,
  act,
  busy,
  assets,
  sound,
  audio,
  clock,
}: BoardProps<View>) {
  const [choiceOpen, setChoiceOpen] = useState(false);
  const [selling, setSelling] = useState<number>();
  const [shopOpen, setShopOpen] = useState(false);
  const [movingTower, setMovingTower] = useState(0);
  const shopPrompt = useRef("");
  const [infoOpen, setInfoOpen] = useState(false);
  const [tasksOpen, setTasksOpen] = useState(false);
  const [placing, setPlacing] = useState(false);
  const [claimPoint, setClaimPoint] = useState({ x: 100, y: 100 });
  const taskPrompt = useRef("");
  const offeredTask =
    data.balanceRules === 1 && data.phase === "prepare"
      ? data.world?.objectives?.find((o) => o.status === "available")
      : undefined;
  const [rewardOpen, setRewardOpen] = useState(false);
  const [rewardKind, setRewardKind] = useState("");
  const rewards =
    data.world?.padRewards?.filter((r) => r.owner === room.you) ?? [];
  const storedReward = rewards.find((r) => r.kind === rewardKind) ?? rewards[0];
  const padReward =
    data.revisionRules === 1 && storedReward
      ? {
          ...storedReward,
          kind:
            storedReward.kind === "add" && data.map.pads.length >= 40
              ? ("move" as const)
              : storedReward.kind,
        }
      : data.world?.padReward;
  const shop = data.controls.shop;
  const saleTower = data.towers.find((t) => t.id === selling);
  const confirmedSale = data.controls.sells.find((c) => c.towerId === selling);
  useEffect(() => {
    if (selling && !data.towers.some((t) => t.id === selling))
      setSelling(undefined);
  }, [selling, data.towers]);
  useEffect(() => {
    setSelling(undefined);
    setShopOpen(false);
  }, [room.id]);
  useEffect(() => {
    const key = room.id + ":" + shop?.wave;
    if (
      shop?.enabled &&
      !padReward &&
      shopPrompt.current !== key &&
      !room.spectating
    ) {
      shopPrompt.current = key;
      setShopOpen(true);
    }
    if (!shop) setShopOpen(false);
  }, [room.id, room.spectating, shop?.wave, shop?.enabled, padReward]);
  useEffect(() => {
    if (!padReward) {
      setPlacing(false);
      setRewardOpen(false);
    }
  }, [padReward]);
  const choiceKey = [
    room.id,
    data.world?.choice?.stage,
    data.world?.boonChoice?.stage,
  ].join(":");
  const hasChoice = !!(data.world?.choice || data.world?.boonChoice);
  const [towerDetails, setTowerDetails] = useState(false);
  const [selected, setSelected] = useState<number | null>(null);
  const root = useRef<HTMLDivElement>(null);
  useControllerCancel(root, () => {
    setSelected(null);
    setTowerDetails(false);
  });
  useEffect(() => {
    setChoiceOpen(hasChoice);
    if (hasChoice) setSelected(null);
  }, [choiceKey, hasChoice]);
  const [target, setTarget] = useState(() =>
    room.seats.findIndex((_, i) => i !== room.you),
  );
  const [gift, setGift] = useState(50);
  const [now, setNow] = useState(performance.now());
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState("");
  const [combatStatus, setCombatStatus] = useState({
    room: room.id,
    baseHP: data.baseHP,
  });
  const baseHP =
    combatStatus.room === room.id ? combatStatus.baseHP : data.baseHP;
  const updateCombatStatus = (status: CombatStatus) =>
    setCombatStatus((previous) =>
      previous.room === room.id && previous.baseHP === status.baseHP
        ? previous
        : { room: room.id, baseHP: status.baseHP },
    );
  const lastSound = useRef(0),
    soundSequence = useRef(0);
  useEffect(() => {
    setSelected(null);
    setInfoOpen(false);
    setTasksOpen(false);
    setFailure("");
  }, [room.id]);
  useEffect(() => {
    const key = room.id + ":" + offeredTask?.id;
    if (
      offeredTask &&
      !shop &&
      !padReward &&
      !room.spectating &&
      !data.world?.choice &&
      !data.world?.boonChoice &&
      taskPrompt.current !== key
    ) {
      taskPrompt.current = key;
      setTasksOpen(true);
    }
  }, [
    room.id,
    room.spectating,
    offeredTask?.id,
    shop?.wave,
    padReward,
    data.world?.choice,
    data.world?.boonChoice,
  ]);
  useEffect(() => {
    const id = window.setInterval(() => setNow(performance.now()), 500);
    return () => clearInterval(id);
  }, []);
  useEffect(() => {
    const listener = () => {
      if (document.hidden) {
        audio.stop();
      }
    };
    document.addEventListener("visibilitychange", listener);
    return () => {
      document.removeEventListener("visibilitychange", listener);
      audio.stop();
    };
  }, [audio, room.id]);
  useEffect(() => {
    if (!sound || data.phase === "paused") audio.stop();
  }, [sound, audio, assets, data.phase]);
  const playEvents = (events: Event[], at: number) => {
    if (!sound || document.hidden || data.phase === "paused") return;
    const clip = pickSound(events, at, lastSound.current);
    if (!clip) return;
    lastSound.current = at;
    const clipURL = (base: string) =>
      ["expand", "objective", "event"].includes(clip)
        ? `${base.replace(/\/$/, "")}/starfort/world-v2/audio/${clip}.wav`
        : assetURL(base, `audio/${clip}.wav`);
    const eventID = Math.max(...events.map((event) => event.id));
    void audio.effect(
      `starfort-${clip}`,
      assets ? [clipURL(assets), clipURL("")] : [clipURL("")],
      {
        playbackID: `${++soundSequence.current}:${data.epoch}:${clip}:${eventID}`,
        reuseLoaded: true,
      },
    );
  };
  const player = room.you >= 0 ? data.players[room.you] : undefined;
  const selectedTower = data.towers.find((t) => t.pad === selected);
  const selectedPad = data.map.pads.find((pad) => pad.id === selected);
  const upgrade = data.controls.upgrades.find(
    (u) => u.towerId === selectedTower?.id,
  );
  const sale = data.controls.sells.find((s) => s.towerId === selectedTower?.id);
  const branchChoices = upgrade?.branches ?? [
    { id: "power", name: "威力专精", description: "提高单次伤害" },
    { id: "speed", name: "攻速专精", description: "提高攻击频率" },
  ];

  const locked = busy || pending;
  const active = data.controls.canAct && !room.spectating;
  const send = async (type: string, extra: Record<string, unknown> = {}) => {
    if (locked || room.spectating) return;
    setPending(true);
    setFailure("");
    try {
      await act({ type, epoch: data.epoch, ...extra });
      if (type === "objective" || type === "skip_objective")
        setTasksOpen(false);
    } catch (error) {
      setFailure(
        error instanceof Error ? error.message : "操作未完成，请重试。",
      );
    } finally {
      setPending(false);
    }
  };
  const branchButtons = branchChoices.map((branch) => (
    <button
      key={branch.id}
      className={
        data.specializationRules === 1 ||
        branch.id === "volley" ||
        branch.id === "heart"
          ? "sf-skill-branch"
          : undefined
      }
      data-sf-upgrade={branch.id}
      title={
        branch.id === "volley" || branch.id === "heart"
          ? undefined
          : branch.description
      }
      disabled={locked || !upgrade?.enabled}
      onClick={() =>
        void send("upgrade", { towerId: selectedTower?.id, branch: branch.id })
      }
    >
      <span>
        {branch.name} · {upgrade?.cost}
      </span>
      {(data.specializationRules === 1 ||
        branch.id === "volley" ||
        branch.id === "heart") && <small>{branch.description}</small>}
    </button>
  ));

  const remaining = Math.max(
    0,
    Math.ceil(
      (data.nextAt -
        (clock.serverNow + (clock.paused ? 0 : now - clock.receivedAt))) /
        1000,
    ),
  );
  const totalRemaining =
    data.enemies.length +
    Math.max(0, data.waveInfo.total - data.waveInfo.spawned);
  const myTowers = data.towers.filter((t) => t.owner === room.you).length;
  const finished = data.phase === "won" || data.phase === "lost";
  const world = data.world;
  const choosing = !!(world?.choice || world?.boonChoice);
  const serverNow =
    clock.serverNow + (clock.paused ? 0 : now - clock.receivedAt);
  const activeRoutes = data.map.routes?.filter((route) => route.active) ?? [];
  const geoLabel = geologyLabel(data.geology);
  const WeatherIcon =
    world?.event.id === "frost"
      ? Snowflake
      : world?.event.id === "starlight"
        ? Sparkles
        : world?.event.id === "calm"
          ? Sun
          : CloudFog;
  const ResourceIcon = room.spectating ? Skull : Coins;
  const resourceAmount = room.spectating ? data.kills : (player?.gold ?? 0);
  const readyCount = data.players.filter((p) => p.ready).length;
  const choiceRemaining = choosing
    ? Math.max(
        0,
        Math.ceil(
          ((world?.boonChoice?.deadline ??
            world?.choice?.deadline ??
            serverNow) -
            serverNow) /
            1000,
        ),
      )
    : remaining;
  const countdownLabel =
    data.phase === "prepare"
      ? `${choosing ? "强化选择" : "自动迎敌"} ${choiceRemaining} 秒`
      : data.phase === "battle"
        ? `剩余 ${totalRemaining} 名敌人`
        : phaseNames[data.phase];

  return (
    <div
      ref={root}
      className={`starfort-board${shopOpen && shop ? " sf-has-shop" : ""}`}
      data-phase={data.phase}
    >
      <div className="sf-topbar">
        <div className="sf-hud">
          {geoLabel && (
            <span
              className="sf-geology-notice"
              data-sf-geology={data.geology?.kind}
              role="status"
            >
              {geoLabel}
            </span>
          )}
          <div className="sf-compact-stats">
            <span
              aria-label={`星核生命 ${baseHP}/${data.baseMaxHP}`}
              title={`星核生命 ${baseHP}/${data.baseMaxHP}`}
            >
              <Heart size={14} aria-hidden="true" />
              <strong data-sf-base>{baseHP}</strong>
              <small>/{data.baseMaxHP}</small>
            </span>
            <span
              data-sf-wave
              aria-label={`第 ${data.wave} 波`}
              title={`第 ${data.wave} 波`}
            >
              <Waves size={14} aria-hidden="true" />
              <strong>{data.wave}</strong>
            </span>
            {(data.weatherRules ?? 0) >= 1 &&
              world?.event &&
              (world.event.id !== "calm" || data.revisionRules === 1) && (
                <span
                  data-sf-weather-notice
                  role="status"
                  title={`${world.event.label}：${world.event.description}`}
                  aria-label={`${world.event.label}：${world.event.description}`}
                >
                  <WeatherIcon
                    size={data.revisionRules === 1 ? 42 : 14}
                    aria-hidden="true"
                  />
                  {data.revisionRules === 1 && <span>{world.event.label}</span>}
                </span>
              )}
            <span
              title={`${room.spectating ? "团队击杀" : "金币"} ${resourceAmount}`}
            >
              <ResourceIcon size={14} aria-hidden="true" />
              <strong
                data-sf-own-gold
                data-value={resourceAmount}
                aria-label={`${room.spectating ? "团队击杀" : "金币"} ${resourceAmount}`}
              >
                {compactAmount(resourceAmount)}
              </strong>
            </span>
            {data.phase === "prepare" &&
              !choosing &&
              data.players.length > 1 && (
                <span
                  title={`已准备 ${readyCount}/${data.players.length}`}
                  aria-label={`已准备 ${readyCount}/${data.players.length}`}
                >
                  <Users size={14} aria-hidden="true" />
                  <small>
                    {readyCount}/{data.players.length}
                  </small>
                </span>
              )}
          </div>
          <div
            className="sf-compact-countdown"
            data-sf-countdown
            data-urgent={
              data.phase === "prepare" && !choosing && remaining <= 10
            }
            title={countdownLabel}
            aria-label={countdownLabel}
          >
            {data.phase === "prepare" ? (
              <>
                <Timer size={14} aria-hidden="true" />
                <strong>{choiceRemaining}秒</strong>
              </>
            ) : data.phase === "battle" ? (
              <>
                <Skull size={14} aria-hidden="true" />
                <strong>{totalRemaining}</strong>
              </>
            ) : data.phase === "paused" ? (
              <Pause size={14} aria-hidden="true" />
            ) : data.phase === "won" ? (
              <Trophy size={14} aria-hidden="true" />
            ) : (
              <CircleX size={14} aria-hidden="true" />
            )}
          </div>
        </div>
        <div className="sf-top-controls">
          {!finished && !room.spectating && data.phase !== "battle" && (
            <section className="sf-wave-controls" aria-label="波次操作">
              {!finished && !room.spectating && (
                <div
                  className="sf-control-buttons"
                  data-controller-group="sf-control-buttons"
                >
                  {choosing && (
                    <button
                      data-sf-open-choice
                      aria-label="选择战区方案"
                      title="选择战区方案"
                      onClick={() => setChoiceOpen(true)}
                    >
                      <Sparkles size={18} aria-hidden="true" />
                    </button>
                  )}
                  {data.phase === "paused" ? (
                    <button
                      data-sf-resume
                      aria-label={
                        player?.resumeVote ? "取消恢复投票" : "同意恢复"
                      }
                      title={player?.resumeVote ? "取消恢复投票" : "同意恢复"}
                      aria-pressed={player?.resumeVote ?? false}
                      disabled={locked || !data.controls.canResume}
                      onClick={() => void send("resume")}
                    >
                      {player?.resumeVote ? (
                        <Undo2 size={18} aria-hidden="true" />
                      ) : (
                        <Play size={18} aria-hidden="true" />
                      )}
                    </button>
                  ) : !choosing ? (
                    <>
                      <button
                        data-sf-ready
                        aria-label={player?.ready ? "取消准备" : "准备迎敌"}
                        title={player?.ready ? "取消准备" : "准备迎敌"}
                        aria-pressed={player?.ready ?? false}
                        disabled={locked || choosing || !data.controls.canReady}
                        onClick={() => void send("ready")}
                      >
                        {player?.ready ? (
                          <Undo2 size={18} aria-hidden="true" />
                        ) : (
                          <Play size={18} aria-hidden="true" />
                        )}
                      </button>
                      <button
                        className="sf-secondary"
                        data-sf-pause
                        aria-label={
                          player?.pauseVote ? "取消暂停投票" : "申请暂停"
                        }
                        title={player?.pauseVote ? "取消暂停投票" : "申请暂停"}
                        aria-pressed={player?.pauseVote ?? false}
                        disabled={locked || !data.controls.canPause}
                        onClick={() => void send("pause")}
                      >
                        {player?.pauseVote ? (
                          <Undo2 size={18} aria-hidden="true" />
                        ) : (
                          <Pause size={18} aria-hidden="true" />
                        )}
                      </button>
                    </>
                  ) : null}
                </div>
              )}
            </section>
          )}{" "}
          <div className="sf-field-actions">
            {choosing && room.spectating && (
              <button
                data-sf-open-choice
                aria-label="查看战区方案"
                title="查看战区方案"
                onClick={() => setChoiceOpen(true)}
              >
                <Sparkles size={18} aria-hidden="true" />
              </button>
            )}
            {selected === null ? (
              <button
                className="sf-select-pad"
                aria-label="选择建造点或管理防御塔"
                title="建造与防御塔"
                onClick={() =>
                  setSelected(
                    data.map.pads.find(
                      (p) => !data.towers.some((t) => t.pad === p.id),
                    )?.id ??
                      data.map.pads[0]?.id ??
                      null,
                  )
                }
              >
                <Hammer size={18} aria-hidden="true" />
              </button>
            ) : selectedTower && !towerDetails ? null : (
              <Modal
                title={`建造点 ${selected + 1}`}
                className="sf-command-dialog"
                onClose={() => {
                  if (selectedTower) setTowerDetails(false);
                  else setSelected(null);
                }}
              >
                <section className="sf-command-panel" aria-label="防线操作">
                  <div className="sf-panel-title">
                    <div>
                      <span>
                        {finished
                          ? "本局防线"
                          : room.spectating
                            ? "观战中"
                            : "你的防线"}
                      </span>
                      <strong>
                        {room.spectating
                          ? "一起守望"
                          : `金币 ${player?.gold ?? 0}`}
                      </strong>
                    </div>
                    <span>
                      {room.spectating ? "只读战场" : `${myTowers} 座塔`}
                    </span>
                  </div>
                  {selected === null || (finished && !selectedTower) ? (
                    <div className="sf-select-hint">
                      <span aria-hidden="true">✦</span>
                      <strong>
                        {finished ? "守望已结束" : "从一座塔开始"}
                      </strong>
                      <p>
                        {finished
                          ? `本局守住 ${data.completedWaves} 波，团队击杀 ${data.kills} 名敌人。`
                          : "点选战场上的空建造点。不同的塔搭配，可以应对不同的敌潮。"}
                      </p>
                      {finished ? (
                        data.towers.length > 0 && (
                          <button
                            onClick={() => setSelected(data.towers[0].pad)}
                          >
                            查看防御塔
                          </button>
                        )
                      ) : (
                        <button
                          onClick={() =>
                            setSelected(
                              data.map.pads.find(
                                (p) => !data.towers.some((t) => t.pad === p.id),
                              )?.id ??
                                data.map.pads[0]?.id ??
                                null,
                            )
                          }
                        >
                          选择建造点
                        </button>
                      )}
                    </div>
                  ) : (
                    <>
                      <div className="sf-selection-title">
                        <strong>建造点 {selected + 1}</strong>
                        <button
                          className="sf-text-button"
                          onClick={() => setSelected(null)}
                          aria-label="取消选中建造点"
                        >
                          取消选择
                        </button>
                      </div>
                      {selectedPad?.terrain === "highground" && (
                        <p className="sf-terrain-note">
                          高地建造点 · 射程 +15%，已计入实际属性
                        </p>
                      )}
                      {selectedTower ? (
                        <div className="sf-tower-details">
                          <div className="sf-tower-heading">
                            <Sprite
                              name={`tower-${selectedTower.kind}-${Math.min(3, selectedTower.level)}`}
                              base={assets}
                            />
                            <div>
                              <h3>
                                {towerNames[selectedTower.kind]} ·{" "}
                                {selectedTower.level} 级
                              </h3>
                              <p>
                                {room.seats[selectedTower.owner]?.name} 的防御塔
                                {selectedTower.branch
                                  ? ` · ${selectedTower.branchName ?? (selectedTower.branch === "power" ? "威力专精" : "攻速专精")}`
                                  : ""}
                              </p>
                            </div>
                          </div>
                          <p>
                            {
                              data.catalog.find(
                                (t) => t.kind === selectedTower.kind,
                              )?.description
                            }
                          </p>
                          <dl
                            className="sf-tower-stats"
                            aria-label="防御塔实际属性"
                          >
                            <div>
                              <dt>伤害</dt>
                              <dd>
                                {selectedTower.effectiveDamage == null
                                  ? "—"
                                  : Number(
                                      selectedTower.effectiveDamage.toFixed(1),
                                    )}
                              </dd>
                            </div>
                            <div>
                              <dt>射程</dt>
                              <dd>
                                {selectedTower.effectiveRange == null
                                  ? "—"
                                  : Number(
                                      selectedTower.effectiveRange.toFixed(1),
                                    )}
                              </dd>
                            </div>
                            <div>
                              <dt>攻击间隔</dt>
                              <dd>
                                {selectedTower.effectiveInterval == null
                                  ? "—"
                                  : `${(selectedTower.effectiveInterval / 1000).toFixed(2)} 秒`}
                              </dd>
                            </div>
                          </dl>
                          <label className="sf-target-control">
                            <span>优先攻击</span>
                            <select
                              data-sf-target
                              aria-label="防御塔优先攻击"
                              value={selectedTower.priority ?? "first"}
                              disabled={
                                locked ||
                                !active ||
                                finished ||
                                selectedTower.owner !== room.you ||
                                !data.controls.targets?.some(
                                  (item) =>
                                    item.towerId === selectedTower.id &&
                                    item.enabled,
                                )
                              }
                              onChange={(event) =>
                                void send("target", {
                                  towerId: selectedTower.id,
                                  priority: event.target.value,
                                })
                              }
                            >
                              {Object.entries(priorityNames).map(
                                ([value, label]) => (
                                  <option key={value} value={value}>
                                    {label}
                                  </option>
                                ),
                              )}
                            </select>
                          </label>
                          <p className="sf-target-hint">
                            支援优先会先攻击治疗者、旗手和工兵；没有这类敌人时优先拦截最前方敌人。
                          </p>
                          {finished ? (
                            <p className="sf-owned-note">
                              本局已结束，可查看防御塔与射程复盘。
                            </p>
                          ) : selectedTower.owner === room.you &&
                            !room.spectating ? (
                            <div
                              className="sf-tower-actions"
                              data-controller-group="sf-tower-actions"
                            >
                              {upgrade?.needsBranch ? (
                                <>
                                  <p className="sf-branch-hint">
                                    选择专精后继续强化
                                  </p>
                                  <div
                                    className="sf-branch-actions"
                                    data-controller-group="sf-branch-actions"
                                  >
                                    {branchButtons}
                                  </div>
                                </>
                              ) : (
                                <button
                                  data-sf-upgrade="level"
                                  disabled={locked || !upgrade?.enabled}
                                  onClick={() =>
                                    void send("upgrade", {
                                      towerId: selectedTower.id,
                                    })
                                  }
                                >
                                  升级到 {selectedTower.level + 1} 级 ·{" "}
                                  {upgrade?.cost ?? "—"} 金币
                                </button>
                              )}
                              <button
                                className="sf-secondary"
                                data-sf-sell
                                disabled={locked || !sale?.enabled}
                                onClick={() => setSelling(selectedTower.id)}
                              >
                                出售 · 返还 {sale?.refund ?? 0} 金币
                              </button>
                            </div>
                          ) : (
                            <p className="sf-owned-note">
                              由 {room.seats[selectedTower.owner]?.name}{" "}
                              管理；可赠予金币支援。
                            </p>
                          )}
                        </div>
                      ) : (
                        <div className="sf-build-list">
                          {data.catalog.map((tower) => {
                            const allowed = data.controls.builds.find(
                              (b) =>
                                b.pad === selected && b.kind === tower.kind,
                            );
                            return (
                              <button
                                className="sf-build-card"
                                data-sf-build={tower.kind}
                                key={tower.kind}
                                disabled={
                                  locked || !active || !allowed?.enabled
                                }
                                onClick={() =>
                                  void send("build", {
                                    pad: selected,
                                    tower: tower.kind,
                                  })
                                }
                              >
                                <Sprite
                                  name={`tower-${tower.kind}-1`}
                                  base={assets}
                                />
                                <span>
                                  <strong>
                                    {tower.name}
                                    <b>{tower.cost} 金币</b>
                                  </strong>
                                  <small>{tower.description}</small>
                                  {tower.passive && (
                                    <small>
                                      {tower.passive.name}：
                                      {tower.passive.description}
                                    </small>
                                  )}
                                </span>
                              </button>
                            );
                          })}
                          {!active && (
                            <p>
                              {room.spectating
                                ? "观众不能建造。"
                                : finished
                                  ? "本局已结束。"
                                  : "当前阶段不能建造。"}
                            </p>
                          )}
                        </div>
                      )}
                    </>
                  )}
                  {failure && (
                    <p className="sf-error" role="alert">
                      {failure}
                    </p>
                  )}
                </section>
              </Modal>
            )}
            {data.revisionRules === 1 && !finished && !room.spectating && (
              <>
                {padReward && (
                  <button
                    data-sf-rewards-open
                    aria-label="领取矿场奖励"
                    title={`领取矿场奖励，剩余 ${rewards.reduce((n, r) => n + r.count, 0)} 份`}
                    onClick={() => {
                      setRewardOpen(true);
                      setPlacing(false);
                    }}
                  >
                    <Gift size={18} aria-hidden="true" />
                    <span className="sf-icon-count" aria-hidden="true">
                      {rewards.reduce((n, r) => n + r.count, 0)}
                    </span>
                  </button>
                )}
                {(data.invulnerabilityPacks ?? 0) > 0 && (
                  <button
                    data-sf-invulnerability
                    aria-label="使用星核无敌道具，持续 10 秒"
                    title="星核无敌 10 秒"
                    disabled={
                      locked || !["prepare", "battle"].includes(data.phase)
                    }
                    onClick={() => void send("use_invulnerability")}
                  >
                    <Shield size={18} aria-hidden="true" />
                  </button>
                )}
                {(data.invulnerableUntil ?? 0) > data.tick && (
                  <span data-sf-invulnerable role="status">
                    星核无敌{" "}
                    {Math.ceil(((data.invulnerableUntil ?? 0) - data.tick) / 5)}{" "}
                    秒
                  </span>
                )}
              </>
            )}
            {data.controls.medical && !finished && !room.spectating && (
              <button
                className="sf-secondary"
                data-sf-use-medical
                aria-label={`使用医疗包，剩余 ${data.controls.medical.count} 个，恢复 ${data.controls.medical.heal} 点生命`}
                disabled={locked || !data.controls.medical.enabled}
                onClick={() => void send("use_medical")}
                title={`恢复 ${data.controls.medical.heal} 点星核生命`}
              >
                <BriefcaseMedical size={18} aria-hidden="true" />
                <span className="sf-icon-count" aria-hidden="true">
                  {data.controls.medical.count}
                </span>
              </button>
            )}
            {shop && (
              <button
                className="sf-secondary"
                data-sf-open-shop
                aria-label="商人商店"
                title="商人商店"
                onClick={() => {
                  setTasksOpen(false);
                  setShopOpen(true);
                }}
              >
                <Store size={18} aria-hidden="true" />
              </button>
            )}
            <button
              className="sf-secondary"
              data-sf-open-tasks
              aria-label="任务"
              title="任务"
              onClick={() => {
                setInfoOpen(false);
                setTasksOpen(true);
              }}
            >
              <ListTodo size={18} aria-hidden="true" />
            </button>
            <button
              className="sf-secondary"
              data-sf-open-info
              aria-label="战场情报"
              title="战场情报"
              onClick={() => {
                setTasksOpen(false);
                setInfoOpen(true);
              }}
            >
              <Info size={18} aria-hidden="true" />
            </button>
          </div>
        </div>
      </div>
      {world && choiceOpen && choosing && !finished && (
        <Modal
          title="战区决策"
          className="sf-choice-dialog"
          onClose={() => setChoiceOpen(false)}
        >
          {!finished && world.choice && (
            <ChoicePanel
              choice={world.choice}
              seats={room.seats}
              you={room.you}
              disabled={
                locked ||
                room.spectating ||
                !data.controls.canRouteVote ||
                data.phase !== "prepare"
              }
              paused={data.phase === "paused"}
              assets={assets}
              serverNow={serverNow}
              send={send}
            />
          )}
          {!finished && world.boonChoice && (
            <ChoicePanel
              choice={world.boonChoice}
              seats={room.seats}
              you={room.you}
              disabled={
                locked ||
                room.spectating ||
                !data.controls.canBoonVote ||
                data.phase !== "prepare"
              }
              paused={data.phase === "paused"}
              assets={assets}
              serverNow={serverNow}
              send={send}
            />
          )}
        </Modal>
      )}

      <div className="sf-layout">
        <section className="sf-map-section" aria-label="星垒战场">
          <Battlefield
            placement={placing ? padReward?.kind : undefined}
            onPlace={(point, pad) => {
              void send(
                "pad_reward",
                padReward?.kind === "highground"
                  ? { pad, choice: storedReward?.kind }
                  : {
                      x: point.x,
                      y: point.y,
                      choice: storedReward?.kind,
                      towerId:
                        movingTower ||
                        data.towers.find((t) => t.owner === room.you)?.id ||
                        data.towers[0]?.id,
                    },
              );
            }}
            onReady={onReady}
            onLoadError={onLoadError}
            onEvents={playEvents}
            onCombatState={updateCombatStatus}
            roomID={room.id}
            data={data}
            assets={assets}
            selected={selected}
            towerPanel={
              selectedTower && !towerDetails ? (
                <section
                  className="sf-tower-popover"
                  aria-label="防御塔操作"
                  data-sf-tower-panel
                  data-sf-tower-tools
                >
                  <header>
                    <strong>
                      {towerNames[selectedTower.kind]} · {selectedTower.level}{" "}
                      级
                    </strong>
                    <button
                      className="sf-secondary"
                      data-sf-tower-close
                      aria-label="关闭防御塔操作"
                      onClick={() => setSelected(null)}
                    >
                      ×
                    </button>
                  </header>
                  {selectedTower.passive && (
                    <SkillBadge
                      key={selectedTower.id}
                      passive={selectedTower.passive}
                      kind={selectedTower.kind}
                      phase={data.phase}
                      elapsedMS={data.elapsedMS}
                    />
                  )}
                  <div className="sf-quick-actions">
                    {selectedTower.owner === room.you &&
                      !finished &&
                      (upgrade?.needsBranch ? (
                        <>{branchButtons}</>
                      ) : (
                        <button
                          data-sf-upgrade="level"
                          disabled={locked || !upgrade?.enabled}
                          onClick={() =>
                            void send("upgrade", { towerId: selectedTower.id })
                          }
                        >
                          升级 · {upgrade?.cost ?? "—"}
                        </button>
                      ))}
                    {selectedTower.owner === room.you && !finished && (
                      <button
                        className="sf-secondary"
                        data-sf-sell
                        disabled={locked || !sale?.enabled}
                        onClick={() => setSelling(selectedTower.id)}
                      >
                        出售 · {sale?.refund ?? 0}
                      </button>
                    )}
                    <button
                      className="sf-secondary"
                      data-sf-tower-more
                      onClick={() => setTowerDetails(true)}
                    >
                      详情
                    </button>
                  </div>
                  {failure && <p role="alert">{failure}</p>}
                </section>
              ) : null
            }
            select={(pad, toggle = true) => {
              setTowerDetails(false);
              setSelected((previous) =>
                toggle &&
                previous === pad &&
                data.towers.some((tower) => tower.pad === pad)
                  ? null
                  : pad,
              );
            }}
          />
        </section>
      </div>
      {tasksOpen && (
        <Modal
          title="任务"
          className="sf-tasks-dialog"
          onClose={() => setTasksOpen(false)}
        >
          {data.controls.repair && (
            <BaseRepair
              quote={data.controls.repair}
              hp={baseHP}
              maxHP={data.baseMaxHP}
              disabled={locked || room.spectating || finished}
              send={send}
            />
          )}
          {data.specializationRules === 1 && data.balanceRules !== 1 && (
            <p data-sf-task-refresh>
              下次刷新：守过第 {(Math.floor(data.completedWaves / 15) + 1) * 15}{" "}
              波
            </p>
          )}
          {world && (world.objectives?.length ?? 0) > 0 ? (
            <section
              className="sf-objectives"
              aria-label="任务列表"
              data-sf-task-offer={offeredTask ? offeredTask.id : undefined}
            >
              {data.balanceRules === 1 ? null : (
                <p>
                  波间由一人支付金币承接。敌人靠近会攻击据点或补给队，需要沿线布塔守护；未承接不会影响正常闯关。
                </p>
              )}
              <div className="sf-objective-grid">
                {world.objectives.map((objective) => (
                  <ObjectiveCard
                    balanced={data.balanceRules === 1}
                    key={objective.id}
                    objective={objective}
                    finished={finished}
                    enabled={
                      !locked &&
                      !room.spectating &&
                      data.phase === "prepare" &&
                      !finished &&
                      !!data.controls.objectives?.some(
                        (item) =>
                          item.objectiveId === objective.id && item.enabled,
                      )
                    }
                    wave={data.wave}
                    completedWaves={data.completedWaves}
                    routeLabel={
                      data.map.routes?.find((r) => r.id === objective.route)
                        ?.label
                    }
                    route={
                      data.map.routes?.find(
                        (route) => route.id === objective.route,
                      )?.points
                    }
                    send={send}
                  />
                ))}
              </div>
            </section>
          ) : (
            <p>当前暂无任务。</p>
          )}
          {offeredTask && !room.spectating && (
            <button
              data-sf-skip-task
              className="sf-secondary"
              disabled={locked}
              onClick={() =>
                void send("skip_objective", {
                  objectiveId: offeredTask.id,
                }).then(() => setTasksOpen(false))
              }
            >
              本波跳过
            </button>
          )}
          {failure && (
            <p className="sf-error" role="alert">
              {failure}
            </p>
          )}
        </Modal>
      )}
      {padReward &&
        padReward.owner === room.you &&
        !placing &&
        (data.phase === "prepare" || data.phase === "battle") &&
        (data.revisionRules !== 1 || rewardOpen) &&
        !choosing && (
          <Modal
            title={
              padReward.kind === "add"
                ? "新增建造点"
                : padReward.kind === "move"
                  ? "移动防御塔"
                  : "升级高地"
            }
            onClose={() => {
              if (data.revisionRules === 1) {
                setRewardOpen(false);
              } else setPlacing(true);
            }}
          >
            <div data-sf-pad-reward>
              {rewards.length > 0 && (
                <label>
                  领取奖励
                  <select
                    aria-label="领取奖励"
                    value={storedReward?.kind}
                    onChange={(e) => setRewardKind(e.target.value)}
                  >
                    {rewards.map((r) => (
                      <option key={r.kind} value={r.kind}>
                        {
                          {
                            add: "新增建造点",
                            highground: "升级高地",
                            move: "移动防御塔",
                          }[r.kind]
                        }{" "}
                        · {r.count} 份
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {failure && (
                <p className="sf-error" role="alert">
                  {failure}
                </p>
              )}
              <p>
                {padReward.kind === "add"
                  ? "在地图上选择新建造点。"
                  : padReward.kind === "move"
                    ? "选择一座防御塔，再选择新位置。"
                    : "选择一个普通建造点升级为高地。"}
              </p>
              {padReward.kind === "move" && (
                <label>
                  移动防御塔
                  <select
                    aria-label="移动防御塔"
                    value={
                      movingTower ||
                      data.towers.find((t) => t.owner === room.you)?.id ||
                      data.towers[0]?.id ||
                      0
                    }
                    onChange={(e) => setMovingTower(Number(e.target.value))}
                  >
                    {data.towers.map((t) => (
                      <option key={t.id} value={t.id}>
                        {towerNames[t.kind]} · {t.level} 级 · 建造点 {t.pad + 1}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <button
                data-sf-choose-reward
                onClick={() => {
                  setTasksOpen(false);
                  setPlacing(true);
                }}
              >
                在地图上选择
              </button>
              {padReward.kind === "add" && (
                <details className="sf-reward-coordinates">
                  <summary>按坐标放置</summary>
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      void send("pad_reward", {
                        ...claimPoint,
                        choice: storedReward?.kind,
                      });
                    }}
                  >
                    <label>
                      横坐标
                      <input
                        type="number"
                        min={28}
                        max={data.map.width - 28}
                        step={1}
                        value={claimPoint.x}
                        onChange={(e) =>
                          setClaimPoint({
                            ...claimPoint,
                            x: Number(e.target.value),
                          })
                        }
                      />
                    </label>
                    <label>
                      纵坐标
                      <input
                        type="number"
                        min={28}
                        max={data.map.height - 28}
                        step={1}
                        value={claimPoint.y}
                        onChange={(e) =>
                          setClaimPoint({
                            ...claimPoint,
                            y: Number(e.target.value),
                          })
                        }
                      />
                    </label>
                    <button disabled={locked}>放置建造点</button>
                  </form>
                </details>
              )}
            </div>
          </Modal>
        )}
      {placing && padReward && (
        <div className="sf-placement-toolbar" role="status">
          {failure && (
            <span className="sf-error" role="alert">
              {failure}
            </span>
          )}
          <span>
            {padReward.kind === "add"
              ? "点选新建造点位置"
              : padReward.kind === "move"
                ? "点选防御塔的新位置"
                : "点选普通建造点升级高地"}
          </span>
          <button className="sf-secondary" onClick={() => setPlacing(false)}>
            返回
          </button>
        </div>
      )}
      {selling !== undefined && saleTower && (
        <Modal title="出售防御塔" onClose={() => setSelling(undefined)}>
          <div data-sf-sell-confirm>
            <p>
              出售 {towerNames[saleTower.kind]}（{saleTower.level} 级），返还{" "}
              {confirmedSale?.refund ?? 0} 金币。塔的等级与专精将一并移除。
            </p>
            <div className="sf-sale-actions">
              <button
                data-sf-sell-cancel
                className="sf-secondary"
                onClick={() => setSelling(undefined)}
              >
                取消
              </button>
              <button
                data-sf-sell-accept
                disabled={locked || !confirmedSale?.enabled}
                onClick={() =>
                  void send("sell", { towerId: saleTower.id }).then(() => {
                    setSelling(undefined);
                    setSelected(null);
                  })
                }
              >
                确认出售
              </button>
            </div>
          </div>
        </Modal>
      )}
      {shopOpen && shop && (
        <aside className="sf-shop-dock" aria-label="商人商店">
          <header>
            <h2>商人商店</h2>
            <button aria-label="收起商店" onClick={() => setShopOpen(false)}>
              ×
            </button>
          </header>
          <MerchantShop
            data={data}
            you={room.you}
            locked={!!locked || !!room.spectating}
            send={send}
          />
          {failure && (
            <p className="sf-error" role="alert">
              {failure}
            </p>
          )}
          {data.options.mode === "campaign" &&
            data.completedWaves >= data.options.waves && (
              <button
                data-sf-shop-finish
                disabled={locked || room.spectating}
                onClick={() => void send("close_shop")}
              >
                结束战役
              </button>
            )}
        </aside>
      )}
      {infoOpen && (
        <Modal
          title="战场情报"
          className="sf-info-dialog"
          onClose={() => setInfoOpen(false)}
        >
          <div className="sf-information">
            {world && (
              <details className="sf-field-guide sf-intelligence" open>
                <summary>
                  战场情报
                  {world.event.id !== "calm" ? ` · ${world.event.label}` : ""} ·
                  第 {world.stage} 战区
                </summary>{" "}
                <section className="sf-world-status" aria-label="战场情报">
                  <p className="sf-wave-description">
                    {data.waveInfo.description} · 团队击杀 {data.kills}
                  </p>
                  <div
                    className="sf-world-heading"
                    data-sf-world-stage={world.stage}
                  >
                    <div>
                      <strong>
                        第 {world.stage} 战区 ·{" "}
                        {data.map.regions?.at(-1)?.label ?? "林地防线"}
                      </strong>
                      <span>
                        初始面积的{" "}
                        {(
                          (data.map.width * data.map.height) /
                          (960 * 640)
                        ).toFixed(1)}{" "}
                        倍 · {data.map.pads.length} 个建造点 ·{" "}
                        {data.map.routes?.length ?? 1} 条道路
                      </span>
                    </div>
                    <span>
                      {world.layout === "central"
                        ? "固定路线 · 星核居中"
                        : world.stage < 4
                          ? `第 ${[0, 6, 11, 16][world.stage]} 波开启新战区`
                          : "完整战场已展开"}
                    </span>
                  </div>
                  {world.event.id !== "calm" && (
                    <div
                      className="sf-world-event"
                      data-sf-wave-event={world.event.id}
                    >
                      <Sprite
                        name={`event-${world.event.id === "starlight" ? "star" : world.event.id}`}
                        base={assets}
                        world
                      />
                      <div>
                        <strong>
                          第 {world.event.wave} 波 · {world.event.label}
                        </strong>
                        <p>{world.event.description}</p>
                      </div>
                    </div>
                  )}
                  {activeRoutes.length > 0 && (
                    <p className="sf-entry-alert">
                      <strong>
                        {data.phase === "prepare" ? "即将来袭" : "当前入口"}：
                      </strong>
                      {activeRoutes.map((route) => route.label).join("、")}
                      {world.shortcutWave === data.wave
                        ? " · 工兵已打开捷径，注意漏防"
                        : world.shortcutWave === data.wave + 1
                          ? " · 工兵已打开通道，下一波将出现捷径"
                          : ""}
                    </p>
                  )}
                  {world.forecast?.length > 0 && !finished && (
                    <div
                      className="sf-event-forecast"
                      aria-label="后续波次预告"
                    >
                      {world.forecast
                        .filter((event) => event.id !== "calm")
                        .map((event) => (
                          <div key={event.wave}>
                            <strong>
                              第 {event.wave} 波 · {event.label}
                            </strong>
                            <span>{event.description}</span>
                          </div>
                        ))}
                    </div>
                  )}
                  {Object.values(world.boons).some((value) => value > 0) && (
                    <p className="sf-boons" data-sf-boons>
                      <strong>团队成长：</strong>伤害 +{world.boons.power * 12}%
                      · 射程 +{world.boons.reach * 8}% · 击杀与过波金币 +
                      {world.boons.fortune * 10}%
                      <span>已计入防御塔实际属性和全队收益</span>
                    </p>
                  )}
                </section>
              </details>
            )}
            <div className="sf-cooperation">
              {room.seats.length > 1 && !room.spectating && !finished && (
                <details className="sf-field-guide">
                  <summary>支援队友 · 赠予金币</summary>
                  <form
                    className="sf-gift"
                    onSubmit={(e) => {
                      e.preventDefault();
                      void send("gift", { target, amount: gift });
                    }}
                  >
                    <strong>支援队友</strong>
                    <div>
                      <label>
                        <span className="sr-only">接收金币的队友</span>
                        <select
                          aria-label="接收金币的队友"
                          value={target}
                          onChange={(e) => setTarget(Number(e.target.value))}
                        >
                          {room.seats.map((seat, i) =>
                            i === room.you ? null : (
                              <option key={seat.id} value={i}>
                                {seat.name}
                              </option>
                            ),
                          )}
                        </select>
                      </label>
                      <label>
                        <span className="sr-only">赠予金币数量</span>
                        <input
                          aria-label="赠予金币数量"
                          type="number"
                          min="1"
                          max={player?.gold ?? 0}
                          step="1"
                          value={gift}
                          onChange={(e) => setGift(Number(e.target.value))}
                        />
                      </label>
                      <button
                        data-sf-gift
                        type="submit"
                        disabled={
                          locked ||
                          !active ||
                          !Number.isInteger(gift) ||
                          gift <= 0 ||
                          gift > (player?.gold ?? 0) ||
                          target < 0
                        }
                      >
                        赠予金币
                      </button>
                    </div>
                  </form>
                </details>
              )}
            </div>
            <details className="sf-field-guide">
              <summary>守望手册 · 敌群与塔的搭配</summary>
              <div>
                <p>
                  <strong>箭塔</strong>快速点杀 · <strong>炮塔</strong>清理群怪
                  · <strong>冰塔</strong>延长敌人在火力区的时间 ·{" "}
                  <strong>法师塔</strong>对付重甲。
                </p>
                <p>
                  史莱姆是基础敌人；疾行怪移动快；重甲怪抵挡普通伤害；小怪成群出没；护盾怪先承受护盾伤害。精英和首领会在关键波次出现。
                </p>
                <p>
                  治疗者恢复附近敌人，旗手为附近敌人加速，分裂怪死亡后留下小怪。工兵填坑速度是普通敌人的两倍。首领半血进入第二阶段并召来护卫；可把关键塔设为优先攻击支援敌人。
                </p>
                <p>
                  {world?.layout === "central"
                    ? "星核位于中心，每个参战席位对应一条固定路线，开局后不扩图。每守过 5 波可选择团队成长；天气与敌潮持续变化。"
                    : "第 6、11、16 波扩展战场，全队投票选择道路，旧塔继续保留。每守过 5 波可选择团队成长；无尽模式轮换来袭入口。"}
                </p>
                <p>
                  升到 4
                  级时选择分支，选定后不能切换。新对局的箭塔可选箭雨或一箭穿心，其他塔可选威力或攻速。新对局的敌人等级等于出场波次；精英与首领不会被穿心直接秒杀。永久冰霜不会叠加，临时减速结束后仍保持减速。出售返还金额以塔面板为准。
                </p>
                <p>
                  暂停需要全员同意，且只能在波间进行。进度自动保存；连续 24
                  小时无人操作的房间会自动关闭。无尽守望没有固定的结束波数；基地归零时结算。
                </p>
              </div>
            </details>
          </div>
        </Modal>
      )}
    </div>
  );
}
