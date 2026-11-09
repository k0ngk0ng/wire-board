import { adjustElementPoints } from "./element-input";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  CheckCheck,
  ChevronRight,
  FlaskConical,
  Heart,
  LockKeyhole,
  Shield,
  Sparkles,
  Swords,
  WandSparkles,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  assetURL,
  loadAssets,
  neededAssets,
  petPreview,
  resourceRoot,
  avatarAppearances,
  previewDirection,
} from "./assets";
import { Sprite } from "./Sprite";
import { Battle } from "./Battle";
import { statusNames, weaponNames } from "./types";
import type {
  BattleEvent,
  Command,
  Equipment,
  Item,
  Loadout,
  PetDefinition,
  Plan,
  Player,
  Unit,
  View,
} from "./types";

const bytes = (n: number) =>
  n < 1024 * 1024
    ? `${(n / 1024).toFixed(0)} KB`
    : `${(n / 1024 / 1024).toFixed(1)} MB`;
const percent = (now: number, max: number) =>
  Math.max(0, Math.min(100, (100 * now) / Math.max(1, max)));
function useTime() {
  const [time, setTime] = useState(performance.now());
  useEffect(() => {
    let frame = 0,
      last = 0;
    const tick = (at: number) => {
      if (at - last > 32) {
        setTime(at);
        last = at;
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, []);
  return time;
}
function recommendedLoadout(v: View): Loadout {
  const p = v.pool,
    seat = Math.max(0, v.viewer),
    ride = p.rides[seat % p.rides.length];
  const reliable = p.pets.filter((p) => p.loyalty >= 80);
  const pets = reliable.length
    ? reliable
    : [p.pets.reduce((a, b) => (a.loyalty >= b.loyalty ? a : b))];
  const skills = [1, 2],
    families = new Set(["attack", "guard"]);
  for (const skill of p.skills) {
    if (skills.length >= 7) break;
    if (!families.has(skill.family)) {
      skills.push(skill.id);
      families.add(skill.family);
    }
  }
  return {
    pet: pets[seat % pets.length].id,
    ride: ride.id,
    avatar: ride.riders![0].avatar,
    points: [140, 120, 100, 120],
    elements: [10, 0, 0, 0],
    skills,
    hat: p.hats[seat % p.hats.length].id,
    weapon: p.weapons[seat % p.weapons.length].id,
    clothes: p.clothes[seat % p.clothes.length].id,
    accessories: p.accessories.slice(0, 2).map((g) => g.id),
    hpItem: p.hpItems[0].id,
    mpItem: p.mpItems[0].id,
    cureItem: p.cureItems[0].id,
    tactics: p.tactics.slice(0, 2).map((i) => i.id),
  };
}
export function Board(props: BoardProps<View>) {
  const { data, room, onReady, onLoadProgress, onLoadError, act } = props;
  const [loaded, setLoaded] = useState(""),
    [attempt, setAttempt] = useState(0),
    [error, setError] = useState(""),
    [progress, setProgress] = useState({ loaded: 0, total: 0 });
  const stage = data.phase === "selection" ? "selection" : "battle",
    key = `${data.session}:${stage}`;
  const resources = useMemo(() => neededAssets(data), [key]); // The pool and locked loadouts are immutable within each stage.
  const sent = useRef("");
  useEffect(() => {
    const controller = new AbortController();
    setError("");
    setLoaded("");
    // The host has already read and verified these code files. Include those
    // completed bytes while adding streamed/cached StoneAge image bytes.
    void fetch("/game-load-manifest.json", {
      cache: "force-cache",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw Error("无法读取游戏程序清单");
        const manifest = (await response.json()) as {
          games: Record<string, { bytes: number }[]>;
        };
        const codeBytes = (manifest.games.stoneage ?? []).reduce(
          (sum, file) => sum + file.bytes,
          0,
        );
        return loadAssets(
          resources,
          controller.signal,
          (imageBytes, imageTotal) => {
            const loaded = codeBytes + imageBytes,
              total = codeBytes + imageTotal;
            setProgress({ loaded, total });
            onLoadProgress?.(
              total ? (loaded / total) * 100 : 0,
              stage === "selection" ? "读取本局候选" : "准备战斗动画",
              { loaded, total },
            );
          },
        );
      })
      .then(() => {
        if (controller.signal.aborted) return;
        setLoaded(key);
        onReady?.();
      })
      .catch((e: unknown) => {
        if (controller.signal.aborted) return;
        const message = e instanceof Error ? e.message : "石器素材未能加载";
        setError(message);
        onLoadError?.(message);
      });
    return () => controller.abort();
  }, [key, attempt, resources, onReady, onLoadProgress, onLoadError, stage]);
  useEffect(() => {
    if (
      loaded !== key ||
      data.phase !== "arming" ||
      room.spectating ||
      room.you < 0 ||
      data.players[room.you]?.battleReady ||
      sent.current === key
    )
      return;
    sent.current = key;
    void act({
      type: "battle_ready",
      session: data.session,
      window: data.window,
    }).catch((e: unknown) => {
      sent.current = "";
      setError(e instanceof Error ? e.message : "战斗就绪未能确认");
    });
  }, [
    loaded,
    key,
    data.phase,
    data.window,
    data.session,
    data.players,
    room.you,
    room.spectating,
    act,
  ]);
  if (loaded !== key || error)
    return (
      <div className="sa-loading" data-stoneage="loading" role="status">
        <Sparkles size={30} />
        <strong>{error || "正在准备本局石器资源"}</strong>
        <progress value={progress.loaded} max={Math.max(1, progress.total)} />
        <span>
          {progress.total
            ? `${Math.floor((progress.loaded / progress.total) * 100)}% · ${bytes(progress.loaded)} / ${bytes(progress.total)}`
            : "读取资源索引…"}
        </span>
        {error && (
          <button onClick={() => setAttempt((v) => v + 1)}>重试加载</button>
        )}
      </div>
    );
  return <Arena {...props} key={data.session} />;
}
function Arena(props: BoardProps<View>) {
  const { data, act, room, busy, audio } = props,
    time = useTime();
  const [error, setError] = useState("");
  const send = async (action: Record<string, unknown>) => {
    setError("");
    try {
      await act({ ...action, session: data.session, window: data.window });
      return true;
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作未能完成");
      return false;
    }
  };
  const battle = data.phase !== "selection";
  const elapsed = data.playback
    ? props.clock.serverNow +
      (props.clock.paused ? 0 : time - props.clock.receivedAt) -
      data.playback.startedAt
    : -1;
  const activeName = data.events.find(
    (e) => elapsed >= e.at && elapsed < e.at + e.duration,
  )?.name;
  useEffect(() => {
    if (battle)
      void audio.music([`${resourceRoot}/audio/bgm/sabgm_b0.wav`], {
        loop: true,
        playbackID: `stoneage:${data.session}`,
      });
    return () => audio.stopMusic();
  }, [battle, data.session, audio]);
  return (
    <div className="sa-root" data-stoneage={data.phase}>
      <div className="sa-game-heading">
        <div>
          <span className="sa-eyebrow">
            {battle
              ? `第 ${data.round} 回合`
              : `${data.players.length / 2}v${data.players.length / 2} · 公共候选`}
          </span>
          <strong>
            {battle
              ? data.phase === "arming"
                ? "全员就绪后开战"
                : data.phase === "resolving"
                  ? (activeName ?? "战斗进行中")
                  : data.phase === "finished"
                    ? "本场战斗结束"
                    : "下达战斗指令"
              : "选择你的战斗组合"}
          </strong>
        </div>
        <div className="sa-teams">
          {[0, 1].map((team) => (
            <span className={`sa-team team-${team}`} key={team}>
              {team === 0 ? "红队" : "蓝队"}
              <b>
                {
                  data.players.filter(
                    (p) => p.team === team && (battle ? p.body.hp > 0 : true),
                  ).length
                }
              </b>
            </span>
          ))}
        </div>
      </div>
      {error && (
        <p className="sa-error" role="alert">
          {error}
        </p>
      )}
      {battle ? (
        <Battle {...props} time={time} send={send} />
      ) : (
        <Selection {...props} time={time} send={send} />
      )}
    </div>
  );
}
type ExtendedProps = BoardProps<View> & {
  time: number;
  send: (a: Record<string, unknown>) => Promise<boolean>;
};
function Selection({ data, room, busy, time, send }: ExtendedProps) {
  const [draft, setDraft] = useState<Loadout>(() =>
      structuredClone(data.self?.loadout ?? recommendedLoadout(data)),
    ),
    [tab, setTab] = useState("pets");
  const content = useRef<HTMLDivElement>(null);
  useEffect(() => {
    content.current?.scrollTo({ top: 0 });
  }, [tab]);
  const mine = data.players[room.you],
    disabled = busy || room.spectating || room.you < 0 || mine?.locked;
  const pool = data.pool,
    pet = pool.pets.find((p) => p.id === draft.pet)!,
    ride = pool.rides.find((p) => p.id === draft.ride)!;
  const update = (value: Partial<Loadout>) =>
    setDraft((old) => ({ ...old, ...value }));
  const toggle = (
    field: "accessories" | "skills" | "tactics",
    id: number,
    limit: number,
  ) => {
    const current = draft[field];
    if (current.includes(id)) {
      update({ [field]: current.filter((v) => v !== id) });
      return;
    }
    let next = current;
    if (field === "skills") {
      const family = pool.skills.find((s) => s.id === id)!.family;
      next = current.filter(
        (v) => pool.skills.find((s) => s.id === v)?.family !== family,
      );
    }
    if (next.length >= limit) return;
    update({ [field]: [...next, id] });
  };
  const lock = () => send({ type: "lock", loadout: draft });
  return (
    <div className="sa-selection" data-stoneage-selection>
      <aside className="sa-roster-preview">
        <div
          className="sa-companion sa-character-preview"
          data-stoneage-character-preview={draft.avatar}
        >
          <div className="sa-companion-image">
            <Sprite
              graphic={draft.avatar}
              direction={previewDirection}
              progress={0}
              time={time}
              size={156}
            />
          </div>
          <span>人物</span>
          <Elements
            values={(draft.elements ?? [10, 0, 0, 0]).map((v) => v * 10)}
          />
        </div>
        <div className="sa-companion">
          <div className="sa-companion-image">
            <Sprite
              graphic={pet.graphic}
              direction={previewDirection}
              progress={0}
              time={time}
              size={156}
            />
          </div>
          <span>出战宠</span>
          <strong>{pet.name}</strong>
          <span>忠诚 {pet.loyalty}</span>
          <Elements values={pet.elements} />
          <PetStats pet={pet} />
        </div>
        <div className="sa-companion">
          <div className="sa-companion-image ride">
            <Sprite
              graphic={
                ride.riders!.find((r) => r.avatar === draft.avatar)!.graphic
              }
              direction={previewDirection}
              progress={0}
              time={time}
              size={130}
            />
          </div>
          <span>骑宠</span>
          <strong>{ride.name}</strong>
          <span>忠诚 {ride.loyalty}</span>
          <Elements values={ride.elements} />
          <PetStats pet={ride} />
        </div>
        <div className="sa-points">
          <div className="sa-section-line">
            <strong>人物加点</strong>
            <span data-stoneage-points-left>
              剩余 {data.pointBudget - draft.points.reduce((a, b) => a + b, 0)}{" "}
              / {data.pointBudget}
            </span>
          </div>
          {["体力", "腕力", "耐力", "速度"].map((name, i) => (
            <label key={name}>
              {name}
              <input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                value={draft.points[i]}
                aria-label={name + "加点"}
                disabled={disabled}
                onChange={(e) => {
                  if (!/^\d*$/.test(e.target.value)) return;
                  const points = [...draft.points];
                  points[i] = Math.max(
                    0,
                    Math.min(
                      data.pointBudget,
                      Math.trunc(Number(e.target.value)),
                    ),
                  );
                  update({ points });
                }}
              />
            </label>
          ))}
        </div>
        <div className="sa-selection-seats">
          {data.players.map((p) => (
            <span key={p.seat} className={`team-${p.team}`}>
              <i />
              {room.seats[p.seat]?.name}
              {p.locked ? (
                <LockKeyhole size={13} />
              ) : (
                <span className="sa-pending">选配中</span>
              )}
            </span>
          ))}
        </div>
      </aside>
      <div className="sa-choice-panel">
        <nav className="sa-tabs" aria-label="选配类别">
          {[
            ["pets", "出战宠"],
            ["rides", "骑宠"],
            ["avatars", "人物"],
            ["skills", "宠物技能"],
            ["equipment", "人物装备"],
            ["supplies", "物品补给"],
          ].map(([id, name]) => (
            <button
              key={id}
              data-stoneage-tab={id}
              aria-pressed={tab === id}
              onClick={() => setTab(id)}
            >
              {name}
            </button>
          ))}
        </nav>
        <div className="sa-choice-content" ref={content}>
          {(tab === "pets" || tab === "rides") && (
            <>
              <div className="sa-section-line">
                <strong>{tab === "pets" ? "出战宠" : "骑宠"}</strong>
                <span>选择 1 只</span>
              </div>
              <div className="sa-pet-grid">
                {(tab === "pets" ? pool.pets : pool.rides).map((p) => (
                  <PetChoice
                    key={p.id}
                    pet={p}
                    selected={
                      p.id === (tab === "pets" ? draft.pet : draft.ride)
                    }
                    disabled={disabled}
                    onClick={() =>
                      tab === "pets"
                        ? update({ pet: p.id })
                        : update({
                            ride: p.id,
                            avatar: p.riders!.some(
                              (r) => r.avatar === draft.avatar,
                            )
                              ? draft.avatar
                              : p.riders![0].avatar,
                          })
                    }
                  />
                ))}
              </div>
            </>
          )}
          {tab === "avatars" && (
            <>
              <div className="sa-element-editor">
                <div className="sa-section-line">
                  <strong>人物属性</strong>
                  <span data-stoneage-elements-left>
                    剩余{" "}
                    {10 -
                      (draft.elements ?? [10, 0, 0, 0]).reduce(
                        (a, b) => a + b,
                        0,
                      )}{" "}
                    / 10
                  </span>
                </div>
                {["地", "水", "火", "风"].map((name, i) => {
                  const values = draft.elements ?? [10, 0, 0, 0];
                  return (
                    <div className={`sa-element-row element-${i}`} key={name}>
                      <span>{name}</span>
                      <button
                        type="button"
                        aria-label={"减少" + name + "属性"}
                        disabled={disabled || values[i] === 0}
                        onClick={() =>
                          update({
                            elements: adjustElementPoints(values, i, -1),
                          })
                        }
                      >
                        −
                      </button>
                      <span className="sa-element-meter" aria-hidden="true">
                        {Array.from({ length: 10 }, (_, n) => (
                          <i
                            key={n}
                            className={n < values[i] ? "filled" : ""}
                          />
                        ))}
                      </span>
                      <button
                        type="button"
                        aria-label={"增加" + name + "属性"}
                        disabled={
                          disabled ||
                          values[(i + 2) % 4] > 0 ||
                          values[i] === 10
                        }
                        onClick={() =>
                          update({
                            elements: adjustElementPoints(values, i, 1),
                          })
                        }
                      >
                        +
                      </button>
                      <output
                        aria-label={name + "属性"}
                        data-stoneage-element-value={i}
                      >
                        {values[i]}
                      </output>
                    </div>
                  );
                })}
              </div>

              <div className="sa-section-line">
                <strong>人物外观</strong>
                <span>选择 1 位</span>
              </div>
              <div
                className="sa-avatar-grid"
                role="group"
                aria-label="人物外观"
              >
                {ride.riders!.map((rider) => {
                  const avatar = avatarAppearances.find(
                    (a) => a.id === rider.avatar,
                  )!;
                  return (
                    <button
                      key={avatar.id}
                      className={`sa-avatar-choice ${draft.avatar === avatar.id ? "selected" : ""}`}
                      data-stoneage-avatar={avatar.id}
                      aria-label={`人物外观 ${avatar.family + 1}，配色 ${avatar.color + 1}`}
                      aria-pressed={draft.avatar === avatar.id}
                      disabled={disabled}
                      onClick={() => update({ avatar: avatar.id })}
                    >
                      <Sprite
                        graphic={avatar.id}
                        direction={previewDirection}
                        progress={0}
                        size={76}
                      />
                      {draft.avatar === avatar.id && <Check size={15} />}
                    </button>
                  );
                })}
              </div>
            </>
          )}
          {tab === "skills" && (
            <>
              <div className="sa-section-line">
                <strong>{pet.name}的技能</strong>
                <span>{draft.skills.length} / 7</span>
              </div>
              <div className="sa-skill-grid">
                {pool.skills.map((skill) => (
                  <button
                    key={skill.id}
                    className={`sa-skill ${draft.skills.includes(skill.id) ? "selected" : ""}`}
                    disabled={disabled || skill.id <= 2}
                    aria-pressed={draft.skills.includes(skill.id)}
                    onClick={() => toggle("skills", skill.id, 7)}
                  >
                    <span className="sa-skill-icon">
                      {skill.status ? (
                        <Sparkles />
                      ) : skill.id === 2 ? (
                        <Shield />
                      ) : (
                        <Swords />
                      )}
                    </span>
                    <span>
                      <strong>{skill.name}</strong>
                      <small>{skill.description}</small>
                    </span>
                    {draft.skills.includes(skill.id) && <Check size={16} />}
                  </button>
                ))}
              </div>
            </>
          )}
          {tab === "equipment" && (
            <>
              {[
                ["hat", "帽子", pool.hats],
                ["weapon", "武器", pool.weapons],
                ["clothes", "衣服", pool.clothes],
                ["accessories", "饰品", pool.accessories],
              ].map(([field, label, rows]) => (
                <section className="sa-gear-section" key={field as string}>
                  <div className="sa-section-line">
                    <strong>{label as string}</strong>
                    <span>
                      {field === "accessories"
                        ? `${draft.accessories.length} / 2`
                        : "选择 1 件"}
                    </span>
                  </div>
                  <div className="sa-gear-grid">
                    {(rows as Equipment[]).map((g) => (
                      <button
                        key={g.id}
                        className={`sa-gear ${(field === "accessories" ? draft.accessories.includes(g.id) : draft[field as "hat" | "weapon" | "clothes"] === g.id) ? "selected" : ""}`}
                        disabled={disabled}
                        aria-pressed={
                          field === "accessories"
                            ? draft.accessories.includes(g.id)
                            : draft[field as "hat" | "weapon" | "clothes"] ===
                              g.id
                        }
                        onClick={() =>
                          field === "accessories"
                            ? toggle("accessories", g.id, 2)
                            : update({ [field as string]: g.id })
                        }
                      >
                        <img src={assetURL(g.icon)} alt="" />
                        <span>
                          <strong>{g.name}</strong>
                          <GearStats gear={g} />
                          {g.magic && (
                            <small className="sa-magic">
                              <Sparkles size={12} />
                              {g.magic.name} · 气 {g.magic.mp}
                            </small>
                          )}
                          {g.resist && (
                            <small>{statusNames[g.resist]}抗性 +25%</small>
                          )}
                        </span>
                      </button>
                    ))}
                  </div>
                </section>
              ))}
            </>
          )}
          {tab === "supplies" && (
            <>
              {[
                ["hpItem", "回血补给", pool.hpItems, "3 份"],
                ["mpItem", "回气补给", pool.mpItems, "2 份"],
                ["cureItem", "解除异常", pool.cureItems, "1 份"],
                ["tactics", "战术道具", pool.tactics, "最多 2 份"],
              ].map(([field, label, rows, count]) => (
                <section className="sa-gear-section" key={field as string}>
                  <div className="sa-section-line">
                    <strong>{label as string}</strong>
                    <span>{count as string}</span>
                  </div>
                  <div className="sa-supply-grid">
                    {(rows as Item[]).map((item) => (
                      <button
                        key={item.id}
                        className={`sa-supply ${(field === "tactics" ? draft.tactics.includes(item.id) : draft[field as "hpItem" | "mpItem" | "cureItem"] === item.id) ? "selected" : ""}`}
                        aria-pressed={
                          field === "tactics"
                            ? draft.tactics.includes(item.id)
                            : draft[
                                field as "hpItem" | "mpItem" | "cureItem"
                              ] === item.id
                        }
                        disabled={disabled}
                        onClick={() =>
                          field === "tactics"
                            ? toggle("tactics", item.id, 2)
                            : update({ [field as string]: item.id })
                        }
                      >
                        <FlaskConical size={25} />
                        <span>
                          <strong>{item.name}</strong>
                          <small>{item.description}</small>
                        </span>
                      </button>
                    ))}
                  </div>
                </section>
              ))}
            </>
          )}
        </div>
        <div className="sa-selection-footer">
          <span>
            {mine?.locked ? (
              <>
                <LockKeyhole size={16} />
                配置已锁定，等待其他玩家
              </>
            ) : room.spectating ? (
              "正在观看双方选配"
            ) : (
              <>
                <CheckCheck size={16} />
                出战宠、骑宠、装备与基础补给齐全
              </>
            )}
          </span>
          <div>
            <button
              data-stoneage-auto-config
              disabled={disabled}
              onClick={async () => {
                const next = recommendedLoadout(data);
                setDraft(next);
                await send({ type: "loadout", loadout: next });
              }}
            >
              <WandSparkles size={15} />
              推荐配置
            </button>
            <button
              data-stoneage-lock
              className="sa-primary"
              disabled={
                disabled ||
                draft.points.reduce((a, b) => a + b, 0) !== data.pointBudget ||
                (draft.elements ?? [10, 0, 0, 0]).reduce((a, b) => a + b, 0) !==
                  10
              }
              onClick={lock}
            >
              {mine?.locked ? "已锁定" : "锁定配置"}
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
function Elements({ values }: { values: number[] }) {
  return (
    <span className="sa-elements">
      {values.map(
        (value, i) =>
          value > 0 && (
            <span className={`element-${i}`} key={i}>
              {["地", "水", "火", "风"][i]} {value / 10}
            </span>
          ),
      )}
    </span>
  );
}
function PetChoice({
  pet,
  selected,
  disabled,
  onClick,
}: {
  pet: PetDefinition;
  selected: boolean;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <button
      className={`sa-pet-choice ${selected ? "selected" : ""}`}
      data-stoneage-pet={pet.id}
      disabled={disabled}
      aria-pressed={selected}
      onClick={onClick}
    >
      <img src={assetURL(petPreview(pet)!)} alt="" />
      <strong>{pet.name}</strong>
      <span className="sa-pet-loyalty" data-stoneage-loyalty={pet.loyalty}>
        忠诚 {pet.loyalty}
      </span>
      <Elements values={pet.elements} />
      <PetStats pet={pet} />
      {selected && <Check size={16} className="sa-chosen" />}
    </button>
  );
}
function PetStats({ pet }: { pet: PetDefinition }) {
  const stats = pet.stats;
  if (!stats) return null;
  return (
    <span className="sa-pet-stats">
      {[
        ["生命", stats.hp],
        ["攻击", stats.attack],
        ["防御", stats.defense],
        ["敏捷", stats.speed],
      ].map(([name, value]) => (
        <span key={String(name)}>
          <small>{name}</small>
          <b>{value}</b>
        </span>
      ))}
    </span>
  );
}
function GearStats({ gear: g }: { gear: Equipment }) {
  return (
    <small className="sa-gear-stats">
      {g.maxHits && g.maxHits > 1 ? (
        <span>
          {g.minHits === g.maxHits ? g.maxHits : `${g.minHits}～${g.maxHits}`}{" "}
          次攻击
        </span>
      ) : null}
      {!!g.critical && <span>会心 +{g.critical}%</span>}
      {!!g.evade && <span>回避 +{g.evade}%</span>}
      {Object.entries(g.resistances ?? {}).map(([kind, n]) => (
        <span key={kind}>
          {
            (
              {
                poison: "毒",
                stone: "石化",
                sleep: "睡眠",
                confusion: "混乱",
                drunk: "酒醉",
              } as Record<string, string>
            )[kind]
          }
          抗性 {n}%
        </span>
      ))}
      {g.slot === "weapon" && <span>{weaponNames[g.weaponType]}</span>}
      {[
        ["攻", g.attack],
        ["防", g.defense],
        ["敏", g.speed],
        ["生命", g.hp],
        ["气", g.mp],
      ]
        .filter(([, n]) => n !== 0)
        .map(([name, n]) => (
          <span key={name}>
            {name} {Number(n) > 0 ? "+" : ""}
            {n}
          </span>
        ))}
    </small>
  );
}
