import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  Clock3,
  ScrollText,
  Sparkles,
  X,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { assetURL, nativeButtons, resourceRoot } from "./assets";
import { Sprite } from "./Sprite";
import { position, facing } from "./formation";
import { statusNames } from "./types";
import type { Command, Player, Unit, View, Equipment } from "./types";
import {
  initialInput,
  inspectActor,
  chooseCommand,
  confirmTarget,
  cancelInput,
  readyPlan,
} from "./battle-input";
import type { BattleInput, ChoicePopup, Owner } from "./battle-input";

type Props = BoardProps<View> & {
  time: number;
  send: (a: Record<string, unknown>) => Promise<boolean>;
};
const percent = (value: number, total: number) =>
  Math.max(0, Math.min(100, (value / Math.max(1, total)) * 100));
const commandNames: Record<string, string> = {
  attack: "攻击",
  guard: "防御",
  magic: "咒术",
  items: "道具",
};
export function Battle({
  data,
  room,
  busy,
  time,
  send,
  clock,
  audio,
  sound,
}: Props) {
  const mine = data.players[room.you];
  const root = useRef<HTMLDivElement>(null),
    sending = useRef(false);
  const [ui, setUI] = useState(() => initialInput(data.window)),
    [posting, setPosting] = useState(false),
    [log, setLog] = useState(false);
  useEffect(() => {
    if (ui.window !== data.window) {
      setUI((old) => ({ ...initialInput(data.window), details: old.details }));
      sending.current = false;
    }
  }, [data.window, ui.window]);
  const cancel = () => setUI((old) => cancelInput(old));
  useControllerCancel(root, cancel);
  useEffect(() => {
    const escape = (e: KeyboardEvent) => {
      if (e.key === "Escape") cancel();
    };
    document.addEventListener("keydown", escape);
    return () => document.removeEventListener("keydown", escape);
  }, []);
  const now = clock.serverNow + (clock.paused ? 0 : time - clock.receivedAt),
    elapsed = data.playback ? now - data.playback.startedAt : Infinity;
  const event = data.playback
    ? data.events.find((e) => elapsed >= e.at && elapsed < e.at + e.duration)
    : undefined;
  const progress = event
    ? Math.max(0, Math.min(1, (elapsed - event.at) / event.duration))
    : 0;
  const seen = useRef(-1);
  useEffect(() => {
    const impact = data.events
      .filter((e) => elapsed >= e.at + e.duration / 2)
      .at(-1);
    if (seen.current < 0) {
      seen.current = impact?.id ?? 0;
      return;
    }
    if (!impact || impact.id <= seen.current) return;
    seen.current = impact.id;
    if (sound && ["attack", "counter", "magic", "item"].includes(impact.kind))
      void audio.effect(
        "stoneage-impact",
        [
          `${resourceRoot}/audio/se/${impact.kind === "attack" || impact.kind === "counter" ? "sam_01.wav" : "sap_01.wav"}`,
        ],
        { playbackID: `${data.session}:${impact.id}` },
      );
  }, [event?.id, progress > 0.5, sound, audio, data.session]);
  const canAct =
    data.phase === "planning" &&
    !busy &&
    !posting &&
    !room.spectating &&
    room.you >= 0 &&
    mine?.body.hp > 0 &&
    !mine.submitted &&
    !room.seats[room.you]?.autoPlay;
  const actorName = (id: number) => {
    const p = data.players[(id / 2) | 0];
    return p
      ? id % 2
        ? p.pet.name
        : (room.seats[p.seat]?.name ?? "人物")
      : "目标";
  };
  const legal = (command: Command, owner: Owner, id: number) => {
    if (!mine) return false;
    const p = data.players[(id / 2) | 0],
      u = id % 2 ? p?.pet : p?.body;
    if (!u || u.hp <= 0 || p.body.hp <= 0) return false;
    if (
      command.kind === "guard" ||
      command.kind === "wait" ||
      (command.kind === "skill" && command.id === 2)
    )
      return id === (owner === "player" ? mine.body.id : mine.pet.id);
    let hostile = true;
    if (command.kind === "magic")
      hostile =
        data.self?.magic?.find((m) => m.id === command.id)?.kind === "damage";
    if (command.kind === "item")
      hostile =
        data.self?.inventory?.find((i) => i.item.id === command.id)?.item
          .kind === "dispel";
    return hostile ? p.team !== mine.team : p.team === mine.team;
  };
  const post = async (next: BattleInput) => {
    setUI(next);
    const plan = readyPlan(next, mine.pet.hp > 0, mine.pet.id);
    if (!plan || !canAct || sending.current) return;
    sending.current = true;
    setPosting(true);
    setUI({ ...next, popup: null, pending: null });
    try {
      await send({ type: "submit", plan });
    } finally {
      sending.current = false;
      setPosting(false);
    }
  };
  const finish = (target: number) => {
    if (!ui.pending || !canAct) return;
    void post(confirmTarget(ui, target, legal(ui.pending, ui.owner, target)));
  };
  const select = (kind: string, id = 0) => {
    if (!canAct) return;
    const next = chooseCommand(ui, ui.owner, { kind, id, target: -1 });
    if (kind === "guard" || (kind === "skill" && id === 2)) {
      void post(
        confirmTarget(
          next,
          ui.owner === "player" ? mine.body.id : mine.pet.id,
          true,
        ),
      );
    } else setUI(next);
  };
  const open = (popup: ChoicePopup) => {
    if (canAct)
      setUI((old) => ({ ...old, popup, pending: null, details: null }));
  };
  const detailPlayer =
    ui.details === null ? undefined : data.players[(ui.details / 2) | 0];
  const details =
    detailPlayer && ui.details !== null
      ? ui.details % 2
        ? detailPlayer.pet
        : detailPlayer.body
      : undefined;
  const pendingName = ui.pending
    ? ui.pending.kind === "skill"
      ? data.pool.skills.find((s) => s.id === ui.pending!.id)?.name
      : ui.pending.kind === "magic"
        ? data.self?.magic?.find((m) => m.id === ui.pending!.id)?.name
        : ui.pending.kind === "item"
          ? data.self?.inventory?.find((i) => i.item.id === ui.pending!.id)
              ?.item.name
          : commandNames[ui.pending.kind]
    : "";
  const seconds = mine
    ? Math.max(0, Math.ceil((mine.deadline - now) / 1000))
    : 0;
  const stateText =
    data.phase === "arming"
      ? "等待全员就绪"
      : data.phase === "resolving"
        ? (event?.name ?? "本回合结算")
        : data.phase === "finished"
          ? "战斗结束"
          : room.spectating
            ? "观战中"
            : mine?.body.hp <= 0
              ? "已退场"
              : mine?.submitted
                ? "指令已提交"
                : posting
                  ? "正在提交指令"
                  : room.seats[room.you]?.autoPlay
                    ? "托管中"
                    : "";
  const closeDetails = () => setUI((old) => ({ ...old, details: null }));
  return (
    <div className="sa-battle sa-battle-native" data-stoneage-battle ref={root}>
      <div
        className="sa-battlefield"
        style={{ backgroundImage: `url(${assetURL("battle/battle_00.png")})` }}
        onContextMenu={(e) => {
          e.preventDefault();
          cancel();
        }}
      >
        {data.players.map((p) => {
          const row = data.players.filter(
            (q) => q.team === p.team && q.seat < p.seat,
          ).length;
          return [p.body, p.pet].map((unit, pet) => {
            const pos = position(p.team, row, pet),
              active =
                event?.actor === unit.id &&
                ["attack", "counter"].includes(event.kind),
              hit = event?.target === unit.id && progress >= 0.5;
            let graphic = unit.graphic;
            if (!pet && p.riding)
              graphic =
                data.pool.rides
                  .find((r) => r.id === p.loadout?.ride)
                  ?.riders?.find((r) => r.avatar === p.loadout?.avatar)
                  ?.graphic ?? graphic;
            const target = event
                ? data.players[(event.target / 2) | 0]
                : undefined,
              targetRow = target
                ? data.players.filter(
                    (q) => q.team === target.team && q.seat < target.seat,
                  ).length
                : 0,
              targetPos = target
                ? position(target.team, targetRow, event!.target % 2)
                : pos;
            const move = active ? Math.sin(progress * Math.PI) * 0.65 : 0;
            const allowed =
              !!ui.pending && canAct && legal(ui.pending, ui.owner, unit.id);
            const identity = pet
              ? `${room.seats[p.seat]?.name}的${unit.name}（出战宠）`
              : `${room.seats[p.seat]?.name}（人物）`;
            const description = `${identity}，生命 ${unit.hp}/${unit.maxHp}${!pet ? `，气力 ${unit.mp}/${unit.maxMp}` : ""}${p.riding && !pet ? `，骑宠 ${p.ride.name} 生命 ${p.ride.hp}/${p.ride.maxHp}` : ""}`;
            return (
              <button
                key={unit.id}
                className={`sa-actor team-${p.team} ${allowed ? "targetable" : ""} ${hit ? "hit" : ""} ${unit.hp <= 0 ? "down" : ""}`}
                style={{
                  left: `${pos.x + (targetPos.x - pos.x) * move}%`,
                  top: `${pos.y + (targetPos.y - pos.y) * move}%`,
                  zIndex: Math.round(pos.y + (targetPos.y - pos.y) * move),
                }}
                data-stoneage-unit={unit.id}
                data-stoneage-direction={facing(p.team)}
                data-stoneage-role={pet ? "pet" : "player"}
                data-stoneage-riding={!pet && p.riding ? "true" : undefined}
                title={description}
                aria-label={`${description}，${allowed ? "选择目标" : "查看详情"}`}
                aria-expanded={ui.details === unit.id}
                onClick={() => {
                  if (allowed) finish(unit.id);
                  else setUI((old) => inspectActor(old, unit.id));
                }}
              >
                <Sprite
                  graphic={graphic}
                  direction={facing(p.team)}
                  action={active ? 0 : hit ? 1 : unit.guard ? 4 : 3}
                  progress={active ? progress : undefined}
                  dead={unit.hp <= 0}
                  time={time}
                  size={126}
                />
                <span className="sa-vitals" aria-hidden="true">
                  <span className="sa-hp" data-stoneage-bar="hp">
                    <i style={{ width: `${percent(unit.hp, unit.maxHp)}%` }} />
                  </span>
                  {!pet && (
                    <span className="sa-mp" data-stoneage-bar="mp">
                      <i
                        style={{ width: `${percent(unit.mp, unit.maxMp)}%` }}
                      />
                    </span>
                  )}
                  {p.riding && !pet && (
                    <span className="sa-ride-hp" data-stoneage-bar="ride">
                      <i
                        style={{
                          width: `${percent(p.ride.hp, p.ride.maxHp)}%`,
                        }}
                      />
                    </span>
                  )}
                </span>
                <span className="sa-unit-status">
                  {Object.entries(statusNames).map(
                    ([key, name]) =>
                      Number(unit.status[key as keyof typeof unit.status]) >
                        0 && (
                        <abbr key={key} title={name}>
                          {name[0]}
                        </abbr>
                      ),
                  )}
                </span>
                {hit && event && (
                  <span
                    className={`sa-float ${["hp", "heal", "mp", "cure"].includes(event.effect ?? "") ? "restored" : ""}`}
                    key={event.id}
                  >
                    {event.kind === "miss"
                      ? "回避"
                      : event.amount
                        ? `${["attack", "counter", "poison"].includes(event.kind) || event.effect === "damage" ? "−" : "+"}${event.amount}${event.critical ? "!" : ""}`
                        : event.name}
                  </span>
                )}
              </button>
            );
          });
        })}
        <div
          className={`sa-field-menu ${ui.pending ? "picking" : ""}`}
          data-stoneage-field-menu
        >
          {stateText ? (
            <div className="sa-field-status">{stateText}</div>
          ) : (
            <>
              <div className="sa-command-clock">
                <span
                  className="sa-field-clock"
                  aria-label={`指令剩余 ${seconds} 秒`}
                >
                  <Clock3 size={12} />
                  {seconds}
                </span>
              </div>
              {ui.pending ? (
                <div className="sa-target-prompt">
                  <span>{pendingName}：选择目标</span>
                  <button onClick={cancel} aria-label="取消选择目标">
                    <X size={15} />
                  </button>
                </div>
              ) : ui.popup ? (
                <div
                  className="sa-field-choices"
                  data-stoneage-choices={ui.popup}
                >
                  <header>
                    <button aria-label="返回战斗菜单" onClick={cancel}>
                      <ArrowLeft size={16} />
                    </button>
                    <strong>
                      {ui.popup === "skills"
                        ? "宠物技能"
                        : ui.popup === "magic"
                          ? "咒术"
                          : "道具"}
                    </strong>
                  </header>
                  <div>
                    {ui.popup === "skills"
                      ? data.self?.loadout?.skills.map((id) => {
                          const skill = data.pool.skills.find(
                            (s) => s.id === id,
                          )!;
                          return (
                            <button
                              key={id}
                              data-stoneage-skill={id}
                              title={skill.description}
                              onClick={() => select("skill", id)}
                            >
                              <span>{skill.name}</span>
                              <small>{skill.description}</small>
                            </button>
                          );
                        })
                      : ui.popup === "magic"
                        ? data.self?.magic?.map((m) => (
                            <button
                              key={m.id}
                              disabled={mine.body.mp < m.mp}
                              onClick={() => select("magic", m.id)}
                            >
                              <span>
                                {m.name}
                                <small>气 {m.mp}</small>
                              </span>
                            </button>
                          ))
                        : data.self?.inventory
                            ?.filter((i) => i.count > 0)
                            .map((i) => (
                              <button
                                key={i.item.id}
                                onClick={() => select("item", i.item.id)}
                              >
                                <span>
                                  {i.item.name}
                                  <small>×{i.count}</small>
                                </span>
                                <small>{i.item.description}</small>
                              </button>
                            ))}
                  </div>
                </div>
              ) : ui.owner === "player" ? (
                <div className="sa-native-buttons">
                  {["attack", "magic", "guard", "items"].map((kind) => (
                    <button
                      key={kind}
                      aria-label={commandNames[kind]}
                      data-stoneage-command={kind}
                      disabled={
                        !canAct ||
                        (kind === "magic" && !data.self?.magic?.length)
                      }
                      title={commandNames[kind]}
                      onClick={() =>
                        kind === "magic" || kind === "items"
                          ? open(kind)
                          : select(kind)
                      }
                    >
                      <img src={assetURL(nativeButtons[kind].normal)} alt="" />
                      <span>{commandNames[kind]}</span>
                    </button>
                  ))}
                </div>
              ) : (
                <button
                  className="sa-pet-skills-open"
                  onClick={() => open("skills")}
                >
                  <Sparkles size={17} />
                  选择宠物技能
                </button>
              )}
              {!!ui.player && !!ui.pet && !posting && (
                <button
                  className="sa-scene-retry"
                  data-stoneage-submit
                  onClick={() => void post(ui)}
                >
                  重新提交指令
                </button>
              )}
            </>
          )}
        </div>
        <button
          className="sa-scene-log-button"
          aria-label="战斗记录"
          onClick={() => setLog((v) => !v)}
        >
          <ScrollText size={17} />
        </button>
        {log && (
          <div className="sa-scene-log" role="dialog" aria-label="战斗记录">
            <header>
              <strong>战斗记录</strong>
              <button aria-label="关闭战斗记录" onClick={() => setLog(false)}>
                <X size={17} />
              </button>
            </header>
            <ol>
              {data.events
                .filter(
                  (e) =>
                    !["status", "wait"].includes(e.kind) &&
                    (!data.playback || elapsed >= e.at + e.duration / 2),
                )
                .map((e) => (
                  <li key={e.id}>
                    {e.actor >= 0 ? actorName(e.actor) : "状态"} · {e.name}
                    {e.target >= 0 ? ` → ${actorName(e.target)}` : ""}
                    {e.amount ? ` ${e.amount}` : ""}
                    {e.rideAmount ? `（骑宠 ${e.rideAmount}）` : ""}
                  </li>
                ))}
            </ol>
          </div>
        )}
        {details && detailPlayer && (
          <ActorDetails
            unit={details}
            player={detailPlayer}
            pet={ui.details! % 2 === 1}
            ownerName={room.seats[detailPlayer.seat]?.name ?? "玩家"}
            data={data}
            close={closeDetails}
          />
        )}
        {data.phase === "arming" && (
          <div className="sa-arming">
            <Sparkles />
            <span>正在等待所有玩家就绪</span>
          </div>
        )}
      </div>
    </div>
  );
}
function ActorDetails({
  unit,
  player,
  pet,
  ownerName,
  data,
  close,
}: {
  unit: Unit;
  player: Player;
  pet: boolean;
  ownerName: string;
  data: View;
  close: () => void;
}) {
  const gear: Equipment[] = player.loadout
    ? [
        data.pool.hats.find((g) => g.id === player.loadout!.hat),
        data.pool.weapons.find((g) => g.id === player.loadout!.weapon),
        data.pool.clothes.find((g) => g.id === player.loadout!.clothes),
        ...data.pool.accessories.filter((g) =>
          player.loadout!.accessories.includes(g.id),
        ),
      ].filter((g): g is Equipment => !!g)
    : [];
  return (
    <div
      className="sa-actor-details"
      data-stoneage-details
      data-side={player.team === 0 ? "right" : "left"}
      role="dialog"
      aria-label={pet ? "宠物详情" : "人物详情"}
      onClick={close}
    >
      <header>
        <div>
          <small>
            {player.team === 0 ? "红队" : "蓝队"} ·{" "}
            {pet ? `${ownerName}的出战宠` : "人物"}
          </small>
          <h3>{pet ? unit.name : ownerName}</h3>
        </div>
        <button
          aria-label="关闭详情"
          onClick={(e) => {
            e.stopPropagation();
            close();
          }}
        >
          <X size={18} />
        </button>
      </header>
      <dl>
        <dt>生命</dt>
        <dd>
          {unit.hp} / {unit.maxHp}
        </dd>
        {!pet && (
          <>
            <dt>气力</dt>
            <dd>
              {unit.mp} / {unit.maxMp}
            </dd>
          </>
        )}
        <dt>攻击</dt>
        <dd>{unit.attack}</dd>
        <dt>防御</dt>
        <dd>{unit.defense}</dd>
        <dt>速度</dt>
        <dd>{unit.speed}</dd>
      </dl>
      <div className="sa-detail-elements">
        {unit.elements.map(
          (value, i) =>
            value > 0 && (
              <span key={i}>
                {["地", "水", "火", "风"][i]} {value}
              </span>
            ),
        )}
      </div>
      {pet ? (
        <section>
          <strong>技能</strong>
          <p>
            {player.loadout?.skills
              .map((id) => data.pool.skills.find((s) => s.id === id)?.name)
              .filter(Boolean)
              .join("、")}
          </p>
        </section>
      ) : (
        <>
          <section>
            <strong>人物加点</strong>
            <p>
              {player.loadout?.points
                .map((n, i) => `${["体力", "腕力", "耐力", "速度"][i]} ${n}`)
                .join(" · ")}
            </p>
          </section>
          <section>
            <strong>装备</strong>
            <p>{gear.map((g) => g.name).join("、")}</p>
          </section>
          <section>
            <strong>
              骑宠 · {player.ride.name}
              {!player.riding ? "（已落马）" : ""}
            </strong>
            <p>
              生命 {player.ride.hp} / {player.ride.maxHp}
            </p>
            <p>
              攻击 {player.ride.attack} · 防御 {player.ride.defense} · 速度{" "}
              {player.ride.speed}
            </p>
          </section>
        </>
      )}
    </div>
  );
}
