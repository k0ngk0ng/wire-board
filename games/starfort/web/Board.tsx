import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Battlefield } from "./Canvas";
import { assetURL, freshEvents, pickSound } from "./presentation";
import type { EventCursor } from "./presentation";
import type { View } from "./types";

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
function Sprite({ name, base }: { name: string; base: string }) {
  return (
    <img
      className="sf-asset"
      src={assetURL(base, `${name}.webp`)}
      alt=""
      aria-hidden="true"
      onError={(e) => {
        const local = assetURL("", `${name}.webp`);
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
    void audio.play([assetURL(assets, `audio/${clip}.wav`)]).then((ok) => {
      if (!ok && assets && !cancelled && !document.hidden)
        void audio.play([assetURL("", `audio/${clip}.wav`)]);
    });
    return () => {
      cancelled = true;
    };
  }, [data, room.id, sound, audio, assets]);
  const player = room.you >= 0 ? data.players[room.you] : undefined;
  const selectedTower = data.towers.find((t) => t.pad === selected);
  const upgrade = data.controls.upgrades.find(
    (u) => u.towerId === selectedTower?.id,
  );
  const sale = data.controls.sells.find((s) => s.towerId === selectedTower?.id);
  const locked = busy || pending;
  const active = data.controls.canAct && !room.spectating;
  const send = async (type: string, extra: Record<string, unknown> = {}) => {
    if (locked) return;
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
            ? `${remaining} 秒后出波`
            : data.phase === "battle"
              ? data.waveInfo.boss
                ? "首领来袭"
                : (ruleNames[data.waveInfo.rule] ?? "敌潮来袭")
              : data.phase === "paused"
                ? "等待全员同意恢复"
                : `共守住 ${data.completedWaves} 波`}
        </span>
      </div>
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
            <span>入口 → 星核</span>
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
                        disabled={locked || !allowed?.enabled}
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
                  ? `${data.players.filter((p) => p.ready).length} / ${data.players.length} 人已准备${data.players.some((p) => p.pauseVote) ? ` · ${data.players.filter((p) => p.pauseVote).length} 人申请暂停` : ""}`
                  : data.phase === "battle"
                    ? "防御塔自动作战，现在仍可建造与升级。"
                    : "本局守望结束。"}
            </p>
          </div>
          {!finished && (
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
                    disabled={locked || !data.controls.canReady}
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
