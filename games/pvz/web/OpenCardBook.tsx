import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Modal } from "../../../web/src/ui/board-host";
import { useGameOverlayRoot } from "../../../web/src/ui/game-overlay-root";
import { CardArt } from "./CardArt";
import { mapFor, terrainReason } from "./scene";
import type { CardBookProps } from "./CardBook";
import type { Side } from "./types";

export function OpenCardBook({
  view,
  art,
  actor,
  canEdit,
  onClose,
  onSave,
  onUse,
}: CardBookProps & { onUse: NonNullable<CardBookProps["onUse"]> }) {
  const player = view.players[actor],
    ownSide = player?.side ?? "plants";
  const overlayRoot = useGameOverlayRoot();
  const [side, setSide] = useState<Side>(ownSide);
  const [mode, setMode] = useState<"use" | "quickbar">("use");
  const [draft, setDraft] = useState([...(player?.deck ?? [])]);
  const [imitateKind, setImitateKind] = useState(
    player?.imitaterKind ?? "peashooter",
  );
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [lifetime] = useState(() => new AbortController());
  useEffect(() => () => lifetime.abort(), [lifetime]);
  const map = mapFor(view),
    ownCatalog = view.catalog[ownSide];
  const interactive =
    !!player &&
    canEdit &&
    !!view.canAct &&
    ["prepare", "battle"].includes(view.phase);
  const usablePhase =
    interactive && (ownSide === "plants" || view.phase === "battle");
  const edit = mode === "quickbar";
  const toggle = (kind: string) => {
    if (!interactive || side !== ownSide || busy) return;
    setError("");
    if (draft.includes(kind)) setDraft(draft.filter((item) => item !== kind));
    else if (draft.length < 10) setDraft([...draft, kind]);
    else setError("快捷栏最多 10 张，可随时从卡册选用其他卡牌");
  };
  const use = async (kind: string) => {
    setBusy(kind);
    setError("");
    try {
      await onUse(kind, imitateKind, lifetime.signal);
      if (!lifetime.signal.aborted) onClose();
    } catch (reason) {
      if (!lifetime.signal.aborted)
        setError(
          reason instanceof Error ? reason.message : "卡牌准备失败，请重试",
        );
    } finally {
      if (!lifetime.signal.aborted) setBusy("");
    }
  };
  const save = async () => {
    if (!draft.length) {
      setError("快捷栏至少保留一张卡");
      return;
    }
    setBusy("save");
    setError("");
    try {
      await onSave(draft, imitateKind);
      if (!lifetime.signal.aborted) onClose();
    } catch (reason) {
      if (!lifetime.signal.aborted)
        setError(reason instanceof Error ? reason.message : "快捷栏保存失败");
    } finally {
      if (!lifetime.signal.aborted) setBusy("");
    }
  };
  return createPortal(
    <Modal
      title="植物大战僵尸卡册"
      onClose={onClose}
      className="pvz-card-book"
      initialFocus="[aria-label=查看植物目录]"
    >
      <div className="pvz-book-toolbar" data-controller-group="catalog-tabs">
        <div className="pvz-book-tabs">
          <button
            type="button"
            aria-label="查看植物目录"
            aria-pressed={side === "plants"}
            onClick={() => {
              setSide("plants");
              setError("");
            }}
          >
            植物 <small>{view.catalog.plants.length}</small>
          </button>
          <button
            type="button"
            aria-label="查看僵尸目录"
            aria-pressed={side === "zombies"}
            onClick={() => {
              setSide("zombies");
              setError("");
            }}
          >
            僵尸 <small>{view.catalog.zombies.length}</small>
          </button>
        </div>
        <span>
          {map.id === "day"
            ? "白天"
            : map.id === "night"
              ? "夜晚"
              : map.id === "pool"
                ? "泳池"
                : map.id === "fog"
                  ? "浓雾"
                  : map.id === "roof"
                    ? "屋顶"
                    : "夜间屋顶"}
        </span>
      </div>
      {player && (
        <div
          className="pvz-book-tabs pvz-book-modes"
          data-controller-group="catalog-modes"
        >
          <button
            type="button"
            aria-pressed={!edit}
            onClick={() => {
              setMode("use");
              setError("");
            }}
          >
            选用卡牌
          </button>
          <button
            type="button"
            aria-pressed={edit}
            onClick={() => {
              setMode("quickbar");
              setError("");
            }}
          >
            编辑快捷栏
          </button>
        </div>
      )}
      {player && edit && (
        <div className="pvz-book-deck" data-controller-group="deck">
          <div className="pvz-book-deck-count">
            快捷栏<strong>{draft.length} / 10</strong>
          </div>
          <div className="pvz-book-slots">
            {draft.map((kind) => (
              <button
                type="button"
                key={kind}
                disabled={!interactive || !!busy}
                aria-label={`移出${ownCatalog.find((spec) => spec.kind === kind)?.name ?? kind}`}
                onClick={() => setDraft(draft.filter((item) => item !== kind))}
              >
                {ownCatalog.find((spec) => spec.kind === kind)?.name ?? kind}
                <span aria-hidden="true">×</span>
              </button>
            ))}
          </div>
        </div>
      )}
      {player &&
        ownSide === "plants" &&
        side === ownSide &&
        (!edit || draft.includes("imitater")) && (
          <label className="pvz-imitater-target">
            模仿目标
            <select
              aria-label="模仿目标"
              disabled={!interactive || !!busy}
              value={imitateKind}
              onChange={(event) => setImitateKind(event.target.value)}
            >
              {view.catalog.plants
                .filter((spec) => spec.kind !== "imitater" && !spec.upgradeFrom)
                .map((spec) => (
                  <option key={spec.kind} value={spec.kind}>
                    {spec.name} · {spec.cost} 阳光
                  </option>
                ))}
            </select>
          </label>
        )}
      <div className="pvz-book-grid" data-controller-group="catalog-cards">
        {view.catalog[side].map((spec) => {
          const effective =
            spec.kind === "imitater"
              ? (view.catalog.plants.find(
                  (card) => card.kind === imitateKind,
                ) ?? spec)
              : spec;
          const reason = terrainReason(effective, map),
            locked = (spec.unlockAt ?? 0) > view.tick;
          const seconds = Math.max(
            0,
            Math.ceil(((player?.cooldowns[spec.kind] ?? 0) - view.tick) * 0.05),
          );
          const included =
            edit && side === ownSide && draft.includes(spec.kind);
          const disabled =
            side !== ownSide ||
            !!busy ||
            (edit ? !interactive : !usablePhase || !!reason || locked);
          return (
            <button
              type="button"
              key={spec.kind}
              data-pvz-book-card={spec.kind}
              className={included ? "pvz-book-included" : ""}
              aria-pressed={edit ? included : undefined}
              aria-disabled={disabled}
              aria-label={`${spec.name}，${effective.cost}${side === "plants" ? "阳光" : "脑力"}${locked ? `，开战后 ${spec.unlockAfterSeconds} 秒解锁` : seconds ? `，冷却 ${seconds} 秒` : ""}${reason ? `，${reason}` : ""}`}
              onClick={() => {
                if (!disabled) {
                  if (edit) toggle(spec.kind);
                  else void use(spec.kind);
                }
              }}
            >
              <CardArt
                art={art}
                kind={spec.kind === "imitater" ? imitateKind : spec.kind}
                lazy
              />
              <strong>{spec.name}</strong>
              <span>
                {effective.cost} {side === "plants" ? "阳光" : "脑力"} ·{" "}
                {(effective.cooldown ?? 0) / 1000} 秒
              </span>
              {spec.upgradeFrom && (
                <small>
                  {spec.kind === "cobcannon"
                    ? "需要两株相邻的本人玉米投手"
                    : `升级自 ${view.catalog.plants.find((card) => card.kind === spec.upgradeFrom)?.name ?? spec.upgradeFrom}`}
                </small>
              )}
              {spec.unlockAfterSeconds && (
                <small>
                  {locked
                    ? `开战后 ${spec.unlockAfterSeconds} 秒解锁`
                    : "已解锁"}
                </small>
              )}
              {!edit && seconds > 0 && !locked && (
                <small>冷却 {seconds} 秒</small>
              )}
              <p>{spec.description}</p>
              {reason && <em>{reason}</em>}
            </button>
          );
        })}
      </div>
      <div className="pvz-book-footer" data-controller-group="catalog-save">
        <span role="status">
          {error ||
            (busy && busy !== "save"
              ? "正在准备卡牌素材…"
              : !edit && ownSide === "zombies" && view.phase === "prepare"
                ? "开战后可出兵"
                : "")}
        </span>
        {player && edit && (
          <button
            type="button"
            aria-label="保存快捷栏"
            disabled={!interactive || !!busy || !draft.length}
            onClick={() => void save()}
          >
            {busy === "save" ? "保存中" : "保存快捷栏"}
          </button>
        )}
      </div>
    </Modal>,
    overlayRoot,
  );
}
