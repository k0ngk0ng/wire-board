import { useState } from "react";
import { createPortal } from "react-dom";
import { Modal } from "../../../web/src/ui/board-host";
import { useGameOverlayRoot } from "../../../web/src/ui/game-overlay-root";
import { CardArt } from "./CardArt";
import type { Art } from "./art";
import { mapFor, terrainReason } from "./scene";
import type { Side, View } from "./types";
import { hasOpenRoster } from "./cards";
import { OpenCardBook } from "./OpenCardBook";

export type CardBookProps = { view: View; art: Art; actor: number; canEdit: boolean; onClose: () => void; onSave: (kinds: string[], imitateKind: string) => Promise<void>; onUse?: (kind: string, imitateKind: string, signal: AbortSignal) => Promise<void> };

export function CardBook(props: CardBookProps) {
  return hasOpenRoster(props.view) && props.onUse ? <OpenCardBook {...props} onUse={props.onUse} /> : <LegacyCardBook {...props} />;
}

function LegacyCardBook({ view, art, actor, canEdit, onClose, onSave }: CardBookProps) {
  const player = view.players[actor], ownSide = player?.side ?? "plants";
  const overlayRoot = useGameOverlayRoot();
  const [side, setSide] = useState<Side>(ownSide);
  const [draft, setDraft] = useState<string[]>([...(player?.deck ?? [])]);
  const [imitateKind, setImitateKind] = useState(player?.imitaterKind ?? "peashooter");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const map = mapFor(view), specs = view.catalog[side], ownCatalog = view.catalog[ownSide];
  const editable = canEdit && view.phase === "prepare";
  const toggle = (kind: string) => {
    if (!editable || side !== ownSide) return;
    setError("");
    if (draft.includes(kind)) setDraft(draft.filter(item => item !== kind));
    else if (draft.length < 10) setDraft([...draft, kind]);
    else setError("每人最多携带 10 张卡，请先移出一张");
  };
  const save = async () => {
    if (!draft.length) { setError("至少携带一张卡"); return; }
    if (draft.includes("imitater") && !imitateKind) { setError("请选择模仿目标"); return; }
    setSaving(true); setError("");
    try { await onSave(draft, imitateKind); onClose(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "卡组保存失败"); }
    finally { setSaving(false); }
  };
  return createPortal(<Modal title="植物大战僵尸卡册" onClose={onClose} className="pvz-card-book" initialFocus="[aria-label=查看植物目录]">
    <div className="pvz-book-toolbar" data-controller-group="catalog-tabs">
      <div className="pvz-book-tabs"><button type="button" aria-label="查看植物目录" aria-pressed={side === "plants"} onClick={() => setSide("plants")}>植物 <small>{view.catalog.plants.length}</small></button><button type="button" aria-label="查看僵尸目录" aria-pressed={side === "zombies"} onClick={() => setSide("zombies")}>僵尸 <small>{view.catalog.zombies.length}</small></button></div>
      <span>{map.id === "day" ? "白天" : map.id === "night" ? "夜晚" : map.id === "pool" ? "泳池" : map.id === "fog" ? "浓雾" : map.id === "roof" ? "屋顶" : "夜间屋顶"}</span>
    </div>
    {player && <div className="pvz-book-deck" data-controller-group="deck">
      <div className="pvz-book-deck-count">{ownSide === "plants" ? "植物卡组" : "僵尸卡组"}<strong>{draft.length} / 10</strong></div>
      <div className="pvz-book-slots">{draft.map(kind => <button type="button" key={kind} disabled={!editable || saving} aria-label={`移出${ownCatalog.find(spec => spec.kind === kind)?.name ?? kind}`} onClick={() => setDraft(draft.filter(item => item !== kind))}>{ownCatalog.find(spec => spec.kind === kind)?.name ?? kind}<span aria-hidden="true">×</span></button>)}</div>
      {draft.includes("imitater") && <label className="pvz-imitater-target">模仿目标<select aria-label="模仿目标" disabled={!editable || saving} value={imitateKind} onChange={event => setImitateKind(event.target.value)}>{view.catalog.plants.filter(spec => spec.kind !== "imitater" && !spec.upgradeFrom).map(spec => <option key={spec.kind} value={spec.kind}>{spec.name} · {spec.cost} 阳光</option>)}</select></label>}
    </div>}
    <div className="pvz-book-grid" data-controller-group="catalog-cards">
      {specs.map(spec => {
        const effective = spec.kind === "imitater" ? view.catalog.plants.find(card => card.kind === imitateKind) ?? spec : spec;
        const reason = terrainReason(spec, map), included = side === ownSide && draft.includes(spec.kind);
        const disabled = !editable || side !== ownSide || saving;
        return <button type="button" key={spec.kind} data-pvz-book-card={spec.kind} className={included ? "pvz-book-included" : ""} aria-pressed={included} aria-disabled={disabled} onClick={() => { if (!disabled) toggle(spec.kind); }}>
          <CardArt art={art} kind={spec.kind} lazy />
          <strong>{spec.name}</strong><span>{effective.cost} {side === "plants" ? "阳光" : "脑力"} · {(effective.cooldown ?? 0) / 1000} 秒</span>
          {spec.upgradeFrom && <small>{spec.kind === "cobcannon" ? "需要两株相邻的本人玉米投手" : `升级自 ${view.catalog.plants.find(card => card.kind === spec.upgradeFrom)?.name ?? spec.upgradeFrom}`}</small>}
          {spec.unlockAfterSeconds && <small>开战后 {spec.unlockAfterSeconds} 秒解锁</small>}
          <p>{spec.description}</p>{reason && <em>{reason}</em>}
        </button>;
      })}
    </div>
    <div className="pvz-book-footer" data-controller-group="catalog-save"><span role="status">{error || (!editable ? "本回合已开战，下一次布阵时可调整卡组" : "")}</span>{player && <button type="button" aria-label="保存卡组" disabled={!editable || saving || !draft.length} onClick={() => void save()}>{saving ? "保存中" : "保存卡组"}</button>}</div>
  </Modal>, overlayRoot);
}
