import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Battlefield } from "./Canvas";
import { assetURL, freshEvents, pickSound } from "./presentation";
import type { EventCursor } from "./presentation";
import type { MapData, Objective, View, WorldChoice } from "./types";

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
  won: "星光长明",
  lost: "防线失守",
};
const ruleNames: Record<string, string> = {
  march: "林地来袭",
  rush: "疾风突袭",
  armor: "铁甲编队",
  swarm: "萤群涌动",
  shield: "护盾潮汐",
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
  objective,
  enabled,
  finished,
  wave,
  completedWaves,
  route,
  send,
}: {
  objective: Objective;
  enabled: boolean;
  finished: boolean;
  wave: number;
  completedWaves: number;
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
        <h4>{objective.label}</h4>
        <span>
          {finished && objective.status === "available"
            ? "本局未承接"
            : objectiveStatus[objective.status]}
        </span>
      </div>
      <p>{objective.description}</p>
      <p className="sf-objective-reward">
        奖励：
        {objective.kind === "mine"
          ? `每人 ${objective.reward} 金币`
          : objective.kind === "beacon"
            ? "全队防御塔永久增加 8% 射程"
            : `每人 ${objective.reward} 金币，星核恢复 4 点生命`}
      </p>
      {active && (
        <p className="sf-objective-progress">
          {objective.kind === "convoy"
            ? `护送进度 ${convoyProgress}% · 抵达星核后领取奖励`
            : `已守住 ${progress} / ${total} 波 · 守到第 ${objective.endWave} 波${wave === objective.endWave ? "，最后一波" : ""}`}
        </p>
      )}
      {objective.status !== "available" && objective.maxHP > 0 && (
        <div
          className="sf-objective-health"
          data-sf-objective-health={objective.id}
        >
          <span>
            {objective.kind === "convoy" ? "补给队生命" : "据点生命"}{" "}
            {Math.ceil(Math.max(0, objective.hp))} / {objective.maxHP}
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
            ? "目标被摧毁，本次支线已结束。"
            : "本局结束时未完成目标，本次支线已结束。"}
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
  room,
  data,
  act,
  busy,
  assets,
  sound,
  audio,
  clock,
}: BoardProps<View>) {
  const [selected, setSelected] = useState<number | null>(null);
  const [target, setTarget] = useState(() =>
    room.seats.findIndex((_, i) => i !== room.you),
  );
  const [gift, setGift] = useState(50);
  const [now, setNow] = useState(performance.now());
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState("");
  const eventCursor = useRef<EventCursor | null>(null),
    lastSound = useRef(0);
  useEffect(() => {
    setSelected(null);
    setFailure("");
  }, [room.id]);
  useEffect(() => {
    const id = window.setInterval(() => setNow(performance.now()), 500);
    return () => clearInterval(id);
  }, []);
  useEffect(() => {
    const listener = () => {
      if (document.hidden) {
        audio.stop();
      } else {
        eventCursor.current = null;
      }
    };
    document.addEventListener("visibilitychange", listener);
    return () => {
      document.removeEventListener("visibilitychange", listener);
      audio.stop();
    };
  }, [audio]);
  useEffect(() => {
    const [cursor, events] = freshEvents(
      eventCursor.current,
      room.id,
      data,
      Date.now(),
    );
    eventCursor.current = cursor;
    if (!sound || document.hidden) {
      audio.stop();
      return;
    }
    const clip = pickSound(events, Date.now(), lastSound.current);
    if (!clip) return;
    lastSound.current = Date.now();
    let cancelled = false;
    const clipURL = (base: string) =>
      ["expand", "objective", "event"].includes(clip)
        ? `${base.replace(/\/$/, "")}/starfort/world-v2/audio/${clip}.wav`
        : assetURL(base, `audio/${clip}.wav`);
    void audio.play([clipURL(assets)]).then((ok) => {
      if (!ok && assets && !cancelled && !document.hidden)
        void audio.play([clipURL("")]);
    });
    return () => {
      cancelled = true;
    };
  }, [data, room.id, sound, audio, assets]);
  const player = room.you >= 0 ? data.players[room.you] : undefined;
  const selectedTower = data.towers.find((t) => t.pad === selected);
  const selectedPad = data.map.pads.find((pad) => pad.id === selected);
  const upgrade = data.controls.upgrades.find(
    (u) => u.towerId === selectedTower?.id,
  );
  const sale = data.controls.sells.find((s) => s.towerId === selectedTower?.id);
  const locked = busy || pending;
  const active = data.controls.canAct && !room.spectating;
  const send = async (type: string, extra: Record<string, unknown> = {}) => {
    if (locked || room.spectating) return;
    setPending(true);
    setFailure("");
    try {
      await act({ type, epoch: data.epoch, ...extra });
    } catch (error) {
      setFailure(
        error instanceof Error ? error.message : "操作未完成，请重试。",
      );
    } finally {
      setPending(false);
    }
  };
  const remaining = Math.max(
    0,
    Math.ceil(
      (data.nextAt - (clock.serverNow + now - clock.receivedAt)) / 1000,
    ),
  );
  const totalRemaining =
    data.enemies.length +
    Math.max(0, data.waveInfo.total - data.waveInfo.spawned);
  const myTowers = data.towers.filter((t) => t.owner === room.you).length;
  const finished = data.phase === "won" || data.phase === "lost";
  const world = data.world;
  const choosing = !!(world?.choice || world?.boonChoice);
  const serverNow = clock.serverNow + now - clock.receivedAt;
  const activeRoutes = data.map.routes?.filter((route) => route.active) ?? [];
  return (
    <div className="starfort-board" data-phase={data.phase}>
      <div className="sf-overview">
        <div className="sf-base-stat">
          <span>星核生命</span>
          <strong data-sf-base>
            {data.baseHP}
            <small> / {data.baseMaxHP}</small>
          </strong>
          <div
            className="sf-hp-track"
            role="meter"
            aria-label="星核生命"
            aria-valuemin={0}
            aria-valuemax={data.baseMaxHP}
            aria-valuenow={data.baseHP}
          >
            <i
              style={{
                width: `${Math.max(0, (100 * data.baseHP) / data.baseMaxHP)}%`,
              }}
            />
          </div>
        </div>
        <div className="sf-wave-stat">
          <span>
            {data.options.mode === "endless"
              ? "无尽守望"
              : `${data.options.waves} 波闯关`}{" "}
            · {difficultyNames[data.options.difficulty]}
          </span>
          <strong data-sf-wave>第 {data.wave} 波</strong>
          <span>
            {data.phase === "battle"
              ? `敌人剩余 ${totalRemaining}`
              : `已守住 ${data.completedWaves} 波`}
          </span>
        </div>
        <div className="sf-team-stat">
          <span>团队击杀</span>
          <strong>{data.kills}</strong>
          <span>{data.towers.length} 座防御塔</span>
        </div>
      </div>
      <div className={`sf-wave-banner sf-phase-${data.phase}`}>
        <div>
          <strong>{phaseNames[data.phase]}</strong>
          <span>
            {finished
              ? data.phase === "won"
                ? "你们一起守住了星光。"
                : "星核熄灭，记住这次防线，下一次再来。"
              : data.waveInfo.title}
          </span>
        </div>
        <span>
          {data.phase === "prepare"
            ? choosing
              ? "共同选择后整备迎敌"
              : `${remaining} 秒后出波`
            : data.phase === "battle"
              ? data.waveInfo.boss
                ? "首领来袭"
                : (ruleNames[data.waveInfo.rule] ?? "敌潮来袭")
              : data.phase === "paused"
                ? "等待全员同意恢复"
                : `共守住 ${data.completedWaves} 波`}
        </span>
      </div>
      {world && (
        <>
          <section className="sf-world-status" aria-label="战场情报">
            <div className="sf-world-heading" data-sf-world-stage={world.stage}>
              <div>
                <strong>
                  第 {world.stage} 战区 ·{" "}
                  {data.map.regions?.at(-1)?.label ?? "林地防线"}
                </strong>
                <span>
                  初始面积的{" "}
                  {((data.map.width * data.map.height) / (960 * 640)).toFixed(
                    1,
                  )}{" "}
                  倍 · {data.map.pads.length} 个建造点 ·{" "}
                  {data.map.routes?.length ?? 1} 条道路
                </span>
              </div>
              <span>
                {world.stage < 4
                  ? `第 ${[0, 6, 11, 16][world.stage]} 波开启新战区`
                  : "完整战场已展开"}
              </span>
            </div>
            <div className="sf-world-event" data-sf-wave-event={world.event.id}>
              {world.event.id !== "calm" && (
                <Sprite
                  name={`event-${world.event.id === "starlight" ? "star" : world.event.id}`}
                  base={assets}
                  world
                />
              )}
              <div>
                <strong>
                  第 {world.event.wave} 波 · {world.event.label}
                </strong>
                <p>{world.event.description}</p>
              </div>
            </div>
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
              <div className="sf-event-forecast" aria-label="后续波次预告">
                {world.forecast.map((event) => (
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
                <strong>团队成长：</strong>伤害 +{world.boons.power * 12}% ·
                射程 +{world.boons.reach * 8}% · 击杀与过波金币 +
                {world.boons.fortune * 10}%
                <span>已计入防御塔实际属性和全队收益</span>
              </p>
            )}
          </section>
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
        </>
      )}
      <div className="sf-layout">
        <section className="sf-map-section" aria-label="星垒战场">
          <Battlefield
            roomID={room.id}
            data={data}
            assets={assets}
            selected={selected}
            select={setSelected}
          />
          <div className="sf-map-caption">
            <span>
              {finished
                ? "本局战场已保留，可点选防御塔查看"
                : "点选圆形建造点，部署或管理防御塔"}
            </span>
            <span>
              {activeRoutes.length > 1
                ? `${activeRoutes.length} 路来袭`
                : "入口 → 星核"}
            </span>
          </div>
          <p className="sf-wave-description">{data.waveInfo.description}</p>
        </section>
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
                {room.spectating ? "一起守望" : `金币 ${player?.gold ?? 0}`}
              </strong>
            </div>
            <span>{room.spectating ? "只读战场" : `${myTowers} 座塔`}</span>
          </div>
          {selected === null || (finished && !selectedTower) ? (
            <div className="sf-select-hint">
              <span aria-hidden="true">✦</span>
              <strong>{finished ? "守望已结束" : "从一座塔开始"}</strong>
              <p>
                {finished
                  ? `本局守住 ${data.completedWaves} 波，团队击杀 ${data.kills} 名敌人。`
                  : "点选战场上的空建造点。不同的塔搭配，可以应对不同的敌潮。"}
              </p>
              {finished ? (
                data.towers.length > 0 && (
                  <button onClick={() => setSelected(data.towers[0].pad)}>
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
                        {towerNames[selectedTower.kind]} · {selectedTower.level}{" "}
                        级
                      </h3>
                      <p>
                        {room.seats[selectedTower.owner]?.name} 的防御塔
                        {selectedTower.branch
                          ? ` · ${selectedTower.branch === "power" ? "威力" : "攻速"}专精`
                          : ""}
                      </p>
                    </div>
                  </div>
                  <p>
                    {
                      data.catalog.find((t) => t.kind === selectedTower.kind)
                        ?.description
                    }
                  </p>
                  <dl className="sf-tower-stats" aria-label="防御塔实际属性">
                    <div>
                      <dt>伤害</dt>
                      <dd>
                        {selectedTower.effectiveDamage == null
                          ? "—"
                          : Number(selectedTower.effectiveDamage.toFixed(1))}
                      </dd>
                    </div>
                    <div>
                      <dt>射程</dt>
                      <dd>
                        {selectedTower.effectiveRange == null
                          ? "—"
                          : Number(selectedTower.effectiveRange.toFixed(1))}
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
                            item.towerId === selectedTower.id && item.enabled,
                        )
                      }
                      onChange={(event) =>
                        void send("target", {
                          towerId: selectedTower.id,
                          priority: event.target.value,
                        })
                      }
                    >
                      {Object.entries(priorityNames).map(([value, label]) => (
                        <option key={value} value={value}>
                          {label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <p className="sf-target-hint">
                    支援优先会先攻击治疗者、旗手和工兵；没有这类敌人时优先拦截最前方敌人。
                  </p>
                  {finished ? (
                    <p className="sf-owned-note">
                      本局已结束，可查看防御塔与射程复盘。
                    </p>
                  ) : selectedTower.owner === room.you && !room.spectating ? (
                    <div className="sf-tower-actions">
                      {upgrade?.needsBranch ? (
                        <>
                          <p className="sf-branch-hint">选择专精后继续强化</p>
                          <div className="sf-branch-actions">
                            <button
                              data-sf-upgrade="power"
                              disabled={locked || !upgrade.enabled}
                              onClick={() =>
                                void send("upgrade", {
                                  towerId: selectedTower.id,
                                  branch: "power",
                                })
                              }
                            >
                              威力专精 · {upgrade.cost}
                            </button>
                            <button
                              data-sf-upgrade="speed"
                              disabled={locked || !upgrade.enabled}
                              onClick={() =>
                                void send("upgrade", {
                                  towerId: selectedTower.id,
                                  branch: "speed",
                                })
                              }
                            >
                              攻速专精 · {upgrade.cost}
                            </button>
                          </div>
                        </>
                      ) : (
                        <button
                          data-sf-upgrade="level"
                          disabled={locked || !upgrade?.enabled}
                          onClick={() =>
                            void send("upgrade", { towerId: selectedTower.id })
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
                        onClick={() =>
                          void send("sell", { towerId: selectedTower.id })
                        }
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
                      (b) => b.pad === selected && b.kind === tower.kind,
                    );
                    return (
                      <button
                        className="sf-build-card"
                        data-sf-build={tower.kind}
                        key={tower.kind}
                        disabled={locked || !active || !allowed?.enabled}
                        onClick={() =>
                          void send("build", {
                            pad: selected,
                            tower: tower.kind,
                          })
                        }
                      >
                        <Sprite name={`tower-${tower.kind}-1`} base={assets} />
                        <span>
                          <strong>
                            {tower.name}
                            <b>{tower.cost} 金币</b>
                          </strong>
                          <small>{tower.description}</small>
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
      </div>
      {world && world.objectives.length > 0 && (
        <section className="sf-objectives" aria-label="战区支线目标">
          <div className="sf-world-heading">
            <h3>战区支线</h3>
            <span>自由承接 · 全队受益</span>
          </div>
          <p>
            波间由一人支付金币承接。敌人靠近会攻击据点或补给队，需要沿线布塔守护；未承接不会影响正常闯关。
          </p>
          <div className="sf-objective-grid">
            {world.objectives.map((objective) => (
              <ObjectiveCard
                key={objective.id}
                objective={objective}
                finished={finished}
                enabled={
                  !locked &&
                  !room.spectating &&
                  data.phase === "prepare" &&
                  !finished &&
                  !!data.controls.objectives?.some(
                    (item) => item.objectiveId === objective.id && item.enabled,
                  )
                }
                wave={data.wave}
                completedWaves={data.completedWaves}
                route={
                  data.map.routes?.find((route) => route.id === objective.route)
                    ?.points
                }
                send={send}
              />
            ))}
          </div>
        </section>
      )}
      <div className="sf-cooperation">
        <section className="sf-wave-controls" aria-label="波次操作">
          <div>
            <strong>
              {finished
                ? "守望记录"
                : data.phase === "paused"
                  ? "暂停与恢复"
                  : "波次准备"}
            </strong>
            <p>
              {data.phase === "paused"
                ? `${data.players.filter((p) => p.resumeVote).length} / ${data.players.length} 人同意恢复`
                : data.phase === "prepare"
                  ? choosing
                    ? "先完成路线与团队成长选择，再准备迎敌。"
                    : `${data.players.filter((p) => p.ready).length} / ${data.players.length} 人已准备${data.players.some((p) => p.pauseVote) ? ` · ${data.players.filter((p) => p.pauseVote).length} 人申请暂停` : ""}`
                  : data.phase === "battle"
                    ? "防御塔自动作战，现在仍可建造与升级。"
                    : "本局守望结束。"}
            </p>
          </div>
          {!finished && !room.spectating && (
            <div className="sf-control-buttons">
              {data.phase === "paused" ? (
                <button
                  data-sf-resume
                  aria-pressed={player?.resumeVote ?? false}
                  disabled={locked || !data.controls.canResume}
                  onClick={() => void send("resume")}
                >
                  {player?.resumeVote ? "取消恢复投票" : "同意恢复"}
                </button>
              ) : (
                <>
                  <button
                    data-sf-ready
                    aria-pressed={player?.ready ?? false}
                    disabled={locked || choosing || !data.controls.canReady}
                    onClick={() => void send("ready")}
                  >
                    {player?.ready ? "取消准备" : "准备迎敌"}
                  </button>
                  <button
                    className="sf-secondary"
                    data-sf-pause
                    aria-pressed={player?.pauseVote ?? false}
                    disabled={locked || !data.controls.canPause}
                    onClick={() => void send("pause")}
                  >
                    {player?.pauseVote ? "取消暂停投票" : "申请暂停"}
                  </button>
                </>
              )}
            </div>
          )}
        </section>
        {room.seats.length > 1 && !room.spectating && !finished && (
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
        )}
      </div>
      <details className="sf-field-guide">
        <summary>守望手册 · 敌群与塔的搭配</summary>
        <div>
          <p>
            <strong>箭塔</strong>快速点杀 · <strong>炮塔</strong>清理群怪 ·{" "}
            <strong>冰塔</strong>延长敌人在火力区的时间 ·{" "}
            <strong>法师塔</strong>对付重甲。
          </p>
          <p>
            史莱姆是基础敌人；疾行怪移动快；重甲怪抵挡普通伤害；小怪成群出没；护盾怪先承受护盾伤害。精英和首领会在关键波次出现。
          </p>
          <p>
            治疗者恢复附近敌人，旗手为附近敌人加速，分裂怪死亡后留下小怪，工兵会打开捷径。首领半血进入第二阶段并召来护卫；可把关键塔设为优先攻击支援敌人。
          </p>
          <p>
            第 6、11、16 波扩展战场，全队投票选择道路，旧塔继续保留。守过每 5
            波可选择一次团队成长。无尽模式展开全部战区后轮换来袭入口，浓雾、寒潮与多路突袭会改变防线。
          </p>
          <p>
            三级后可选威力或攻速专精。出售会返还部分投入，请先选中自己的塔查看金额。蓝色轮廓表示减速，敌人头顶的细条显示生命和护盾。
          </p>
          <p>
            暂停需要全员同意，且只能在波间进行。进度自动保存；连续 24
            小时无人操作的房间会自动关闭。无尽守望没有固定的结束波数；基地归零时结算。
          </p>
        </div>
      </details>
    </div>
  );
}
