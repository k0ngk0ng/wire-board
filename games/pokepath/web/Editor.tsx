import { useLocalize } from "./I18n";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Action, Catalog, Definition, View } from "./types";
import "./editor.css";

export type EditorSlot = { enemy: string | null; amount: number };
export type EditorData = { mapId: number; hearts: number; health: number; armor: number; wave: EditorSlot[] };
type DraftGroup = { key: number; enemy: string | null; amount: string };
type Draft = { mapId: number; hearts: string; health: string; armor: string; groups: DraftGroup[] };
type EnemyChoice = { key: string; name: string };
type EditorProgression = NonNullable<View["progression"]> & { custom?: boolean; customData?: EditorData };
export type EditorProps = { view: View; catalog?: Catalog; onAct: (action: Action) => Promise<unknown> | void; disabled: boolean; onClose: () => void };

const groupLimit = 256;
const spawnLimit = 4096;
const codeLimit = 32768;
const secretRoutes: Record<number, string> = { 20: "manaphy", 30: "mew", 31: "victini", 32: "marshadow", 33: "farfetchdGalar" };
const inputKeys = new Set(["mapId", "hearts", "health", "armor", "wave"]);
const slotKeys = new Set(["enemy", "amount"]);

function object(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

function allowedRoute(id: number, view: View, catalog: Catalog): boolean {
  const route = catalog.routes[String(id)];
  if (!route) return false;
  if (!route.isSecret) return view.stars >= (route.unlockStars ?? 0);
  const required = secretRoutes[route.id];
  return Boolean(required && view.progression?.secrets?.[required]);
}

function integer(value: unknown, fallback: number, minimum: number, maximum: number, step: number, name: string): number {
  const number = value === undefined ? fallback : value;
  if (typeof number !== "number" || !Number.isSafeInteger(number) || number < minimum || number > maximum || number % step !== 0) throw Error(`${name}应为 ${minimum}～${maximum}${step > 1 ? `，并按 ${step} 递增` : ""}`);
  return number;
}

export function validateEditorData(value: unknown, view: View, catalog: Catalog): EditorData {
  if (!object(value) || Object.keys(value).some(key => !inputKeys.has(key))) throw Error("分享码包含无效的波次字段");
  const mapId = value.mapId ?? 0;
  if (typeof mapId !== "number" || !Number.isSafeInteger(mapId) || mapId < 0) throw Error("路线编号无效");
  if (!allowedRoute(mapId, view, catalog)) throw Error("该路线尚未解锁，无法用于实践");
  const hearts = integer(value.hearts, 14, 1, 14, 1, "防线生命");
  const health = integer(value.health, 0, 0, 200, 5, "生命强化");
  const armor = integer(value.armor, 0, 0, 200, 5, "护甲强化");
  if (!Array.isArray(value.wave) || value.wave.length === 0 || value.wave.length > groupLimit) throw Error("请设置 1～256 组出兵");
  let expanded = 0;
  let enemies = 0;
  const wave = value.wave.map((entry: unknown): EditorSlot => {
    if (!object(entry) || Object.keys(entry).some(key => !slotKeys.has(key))) throw Error("出兵分组格式无效");
    const amount = integer(entry.amount, 0, 1, spawnLimit, 1, "分组数量");
    const enemy = entry.enemy === undefined || entry.enemy === null || entry.enemy === "" ? null : entry.enemy;
    if (enemy !== null && (typeof enemy !== "string" || !Object.hasOwn(catalog.enemies, enemy))) throw Error("出兵分组包含未知敌人");
    expanded += amount;
    if (expanded > spawnLimit) throw Error("出兵与间隔合计不能超过 4096 个位置");
    if (enemy !== null) enemies += amount;
    return { enemy, amount };
  });
  if (enemies === 0) throw Error("波次至少需要一名敌人");
  return { mapId, hearts, health, armor, wave };
}

function inputOnly(data: EditorData): EditorData {
  return { mapId: data.mapId, hearts: data.hearts, health: data.health, armor: data.armor, wave: data.wave.map(slot => ({ enemy: slot.enemy || null, amount: slot.amount })) };
}

export function encodeEditorCode(data: EditorData): string {
  const bytes = new TextEncoder().encode(JSON.stringify(inputOnly(data)));
  const code = btoa(Array.from(bytes, byte => String.fromCharCode(byte)).join(""));
  if (code.length > codeLimit) throw Error("分享码过长，请减少出兵分组");
  return code;
}

export function decodeEditorCode(source: string, view: View, catalog: Catalog): EditorData {
  const original = source.trim();
  if (!original || original.length > codeLimit) throw Error("分享码为空或过长，请重新复制完整代码");
  const code = original.replace(/[\r\n]/g, "");
  if (!code || code.length % 4 !== 0 || !/^[A-Za-z0-9+/]*={0,2}$/.test(code)) throw Error("分享码格式无效，请重新复制完整代码");
  try {
    const binary = atob(code);
    if (btoa(binary) !== code) throw Error("invalid base64 padding");
    const bytes = Uint8Array.from(binary, character => character.charCodeAt(0));
    if (bytes.length > codeLimit) throw Error("oversized editor input");
    const input = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)) as unknown;
    return validateEditorData(input, view, catalog);
  } catch (cause) {
    if (cause instanceof Error && /路线|生命|强化|分组|位置|敌人|字段|波次/.test(cause.message)) throw cause;
    throw Error("分享码无法读取，请重新复制完整代码");
  }
}

function displayName(definition: Definition, fallback: string): string {
  const name = Array.isArray(definition.name) ? definition.name[8] ?? definition.name[0] : definition.name;
  return (name || fallback).replace(/\[\/?[A-Z_]+\]/g, "");
}

function initialData(view: View, catalog?: Catalog): EditorData {
  const progression = view.progression as EditorProgression | undefined;
  if (progression?.customData && catalog) {
    try { return validateEditorData(inputOnly(progression.customData), view, catalog); } catch { }
  }
  const routes = Object.values(catalog?.routes ?? {}).filter(route => catalog && allowedRoute(route.id, view, catalog)).sort((a, b) => a.id - b.id);
  const mapId = routes.some(route => route.id === view.route) ? view.route : routes[0]?.id ?? 0;
  const enemy = catalog && !catalog.enemies.rattata ? Object.keys(catalog.enemies)[0] : "rattata";
  return { mapId, hearts: 14, health: 0, armor: 0, wave: enemy ? [{ enemy, amount: 3 }] : [] };
}

function draftFrom(data: EditorData, nextKey: () => number): Draft {
  return { mapId: data.mapId, hearts: String(data.hearts), health: String(data.health), armor: String(data.armor), groups: data.wave.map(slot => ({ key: nextKey(), enemy: slot.enemy, amount: String(slot.amount) })) };
}

function candidate(draft: Draft): EditorData {
  const number = (value: string) => value.trim() === "" ? NaN : Number(value);
  return { mapId: draft.mapId, hearts: number(draft.hearts), health: number(draft.health), armor: number(draft.armor), wave: draft.groups.map(group => ({ enemy: group.enemy, amount: number(group.amount) })) };
}

const GroupRow = memo(function GroupRow({ group, index, count, enemies, disabled, onUpdate, onStep, onMove, onDelete }: { group: DraftGroup; index: number; count: number; enemies: EnemyChoice[]; disabled: boolean; onUpdate: (key: number, patch: Partial<Pick<DraftGroup, "enemy" | "amount">>) => void; onStep: (key: number, delta: number) => void; onMove: (key: number, delta: number) => void; onDelete: (key: number) => void }) {
  const label = `第 ${index + 1} 组`;
  return <li className="pokepath-editor-group" data-editor-group>
    <select aria-label={`${label}敌人`} value={group.enemy ?? ""} disabled={disabled} onChange={event => onUpdate(group.key, { enemy: event.target.value || null })}>
      <option value="">间隔</option>{enemies.map(enemy => <option key={enemy.key} value={enemy.key}>{enemy.name}</option>)}
    </select>
    <div className="pokepath-editor-counter"><button type="button" aria-label={`减少${label}数量`} disabled={disabled || Number(group.amount) <= 1} onClick={() => onStep(group.key, -1)}>−</button><input aria-label={`${label}数量`} type="number" inputMode="numeric" min={1} max={spawnLimit} step={1} value={group.amount} disabled={disabled} onChange={event => onUpdate(group.key, { amount: event.target.value })} /><button type="button" aria-label={`增加${label}数量`} disabled={disabled || Number(group.amount) >= spawnLimit} onClick={() => onStep(group.key, 1)}>+</button></div>
    <div className="pokepath-editor-row-actions"><button type="button" aria-label={`上移${label}`} disabled={disabled || index === 0} onClick={() => onMove(group.key, -1)}>↑</button><button type="button" aria-label={`下移${label}`} disabled={disabled || index === count - 1} onClick={() => onMove(group.key, 1)}>↓</button><button type="button" aria-label={`删除${label}`} disabled={disabled} onClick={() => onDelete(group.key)}>删除</button></div>
  </li>;
});

export function Editor({ view, catalog, onAct, disabled, onClose }: EditorProps) {
  const localize = useLocalize();
  const serial = useRef(0);
  const nextKey = () => ++serial.current;
  const [draft, setDraft] = useState<Draft>(() => draftFrom(initialData(view, catalog), nextKey));
  const [error, setError] = useState("");
  const [codeText, setCodeText] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [pending, setPending] = useState(false);
  const textarea = useRef<HTMLTextAreaElement>(null);
  const progression = view.progression as EditorProgression | undefined;
  const customSignature = progression?.customData ? JSON.stringify(progression.customData) : "";
  const boundary = useRef({ epoch: view.epoch, customSignature, catalog: Boolean(catalog) });
  const routes = useMemo(() => Object.values(catalog?.routes ?? {}).filter(route => catalog && allowedRoute(route.id, view, catalog)).sort((a, b) => a.id - b.id), [catalog, view.stars, view.progression?.secrets]);
  const enemies = useMemo(() => Object.entries(catalog?.enemies ?? {}).map(([key, enemy]) => ({ key, name: displayName(enemy, key) })).sort((a, b) => a.name.localeCompare(b.name, "zh-CN")), [catalog]);
  const locked = disabled || pending || !catalog;
  const worldBusy = Boolean(view.waveActive || view.paused || view.saving || progression?.challenge?.active || progression?.draft?.active);

  useEffect(() => {
    const previous = boundary.current;
    if (previous.epoch !== view.epoch || previous.customSignature !== customSignature || !previous.catalog && catalog) {
      setDraft(draftFrom(initialData(view, catalog), nextKey));
      setCodeText(null);
      setError("");
      setCopied(false);
    }
    boundary.current = { epoch: view.epoch, customSignature, catalog: Boolean(catalog) };
  }, [view.epoch, customSignature, catalog]);

  const mutate = useCallback((update: (previous: Draft) => Draft) => {
    setDraft(update);
    setCodeText(null);
    setCopied(false);
    setError("");
  }, []);
  const updateGroup = useCallback((key: number, patch: Partial<Pick<DraftGroup, "enemy" | "amount">>) => mutate(previous => ({ ...previous, groups: previous.groups.map(group => group.key === key ? { ...group, ...patch } : group) })), [mutate]);
  const stepGroup = useCallback((key: number, delta: number) => mutate(previous => ({ ...previous, groups: previous.groups.map(group => group.key === key ? { ...group, amount: String(Math.max(1, Math.min(spawnLimit, (Number(group.amount) || 0) + delta))) } : group) })), [mutate]);
  const moveGroup = useCallback((key: number, delta: number) => mutate(previous => {
    const index = previous.groups.findIndex(group => group.key === key);
    const destination = index + delta;
    if (index < 0 || destination < 0 || destination >= previous.groups.length) return previous;
    const groups = [...previous.groups];
    [groups[index], groups[destination]] = [groups[destination], groups[index]];
    return { ...previous, groups };
  }), [mutate]);
  const deleteGroup = useCallback((key: number) => mutate(previous => ({ ...previous, groups: previous.groups.filter(group => group.key !== key) })), [mutate]);
  const expanded = draft.groups.reduce((sum, group) => sum + (Number.isSafeInteger(Number(group.amount)) && Number(group.amount) > 0 ? Number(group.amount) : 0), 0);
  const enemyCount = draft.groups.reduce((sum, group) => sum + (group.enemy && Number.isSafeInteger(Number(group.amount)) && Number(group.amount) > 0 ? Number(group.amount) : 0), 0);
  let valid: EditorData | undefined;
  let issue = "";
  if (catalog) {
    try { valid = validateEditorData(candidate(draft), view, catalog); } catch (cause) { issue = cause instanceof Error ? cause.message : "波次设置无效"; }
  }
  let generated = "";
  if (valid) {
    try { generated = encodeEditorCode(valid); } catch (cause) { issue = cause instanceof Error ? cause.message : "无法生成分享码"; }
  }
  const shareCode = codeText ?? generated;
  const addGroup = (enemy: string | null) => {
    if (locked || draft.groups.length >= groupLimit || expanded >= spawnLimit) return;
    const group: DraftGroup = { key: nextKey(), enemy, amount: "1" };
    mutate(previous => ({ ...previous, groups: [...previous.groups, group] }));
  };
  const submit = async (action: Action, close = false) => {
    if (locked || worldBusy) return;
    setError("");
    setPending(true);
    try { await onAct(action); if (close) onClose(); } catch (cause) { setError(cause instanceof Error ? cause.message : "操作未完成，请重试"); } finally { setPending(false); }
  };
  const importCode = () => {
    if (locked || !catalog) return;
    try {
      const data = decodeEditorCode(shareCode, view, catalog);
      mutate(() => draftFrom(data, nextKey));
    } catch (cause) { setError(cause instanceof Error ? cause.message : "分享码无法读取"); }
  };
  const copyCode = async () => {
    if (!shareCode.trim()) return;
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(shareCode);
      else {
        textarea.current?.focus();
        textarea.current?.select();
        if (!document.execCommand("copy")) throw Error("无法复制，请选择分享码后复制");
      }
      setCopied(true);
      setError("");
    } catch { setError("无法复制，请选择分享码后复制"); }
  };

  return localize(<div className="pokepath-overlay" onClick={event => { if (event.target === event.currentTarget) onClose(); }} onKeyDown={event => { if (event.key === "Escape") { event.stopPropagation(); onClose(); } }}>
    <section className="pokepath-panel pokepath-editor" role="dialog" aria-modal="true" aria-label="波次编辑" data-native-editor>
      <header><h2>波次编辑</h2><button type="button" data-controller-cancel className="pokepath-panel-close" aria-label="关闭波次编辑" onClick={onClose}>×</button></header>
      <div className="pokepath-panel-body pokepath-editor-body">
        <div className="pokepath-editor-parameters">
          <label className="pokepath-editor-route">路线<select aria-label="路线" value={draft.mapId} disabled={locked} onChange={event => mutate(previous => ({ ...previous, mapId: Number(event.target.value) }))}>{routes.map(route => <option key={route.id} value={route.id}>{route.name}</option>)}</select></label>
          <label className="pokepath-editor-hearts">防线生命<input aria-label="防线生命" type="number" inputMode="numeric" min={1} max={14} step={1} value={draft.hearts} disabled={locked} onChange={event => mutate(previous => ({ ...previous, hearts: event.target.value }))} /></label>
          <label>生命强化<span className="pokepath-editor-percent"><input aria-label="生命强化" type="number" inputMode="numeric" min={0} max={200} step={5} value={draft.health} disabled={locked} onChange={event => mutate(previous => ({ ...previous, health: event.target.value }))} /><span aria-hidden="true">%</span></span></label>
          <label>护甲强化<span className="pokepath-editor-percent"><input aria-label="护甲强化" type="number" inputMode="numeric" min={0} max={200} step={5} value={draft.armor} disabled={locked} onChange={event => mutate(previous => ({ ...previous, armor: event.target.value }))} /><span aria-hidden="true">%</span></span></label>
        </div>
        <section className="pokepath-editor-sequence" aria-label="出兵次序"><div className="pokepath-editor-sequence-heading"><h3>出兵次序</h3><div><button type="button" aria-label="加入敌人" disabled={locked || !enemies.length || draft.groups.length >= groupLimit || expanded >= spawnLimit} onClick={() => addGroup(catalog?.enemies.rattata ? "rattata" : enemies[0]?.key ?? null)}>加入敌人</button><button type="button" aria-label="加入间隔" disabled={locked || draft.groups.length >= groupLimit || expanded >= spawnLimit} onClick={() => addGroup(null)}>加入间隔</button></div></div>
          <ol className="pokepath-editor-groups">{draft.groups.map((group, index) => <GroupRow key={group.key} group={group} index={index} count={draft.groups.length} enemies={enemies} disabled={locked} onUpdate={updateGroup} onStep={stepGroup} onMove={moveGroup} onDelete={deleteGroup} />)}</ol>
        </section>
        <section className="pokepath-editor-preview" aria-label="波次预览"><div><h3>波次预览</h3><span>{draft.groups.length} / {groupLimit} 组 · {expanded} / {spawnLimit} 个位置</span></div><ol>{draft.groups.map(group => <li key={group.key}>{group.enemy ? enemies.find(enemy => enemy.key === group.enemy)?.name ?? group.enemy : "间隔"}<strong>×{group.amount || "—"}</strong></li>)}</ol><p>{enemyCount} 名敌人 · {expanded - enemyCount} 个间隔</p></section>
        <details className="pokepath-editor-sharing"><summary>分享与导入</summary><label>分享码<textarea ref={textarea} aria-label="分享码" rows={4} value={shareCode} readOnly={disabled || pending} spellCheck={false} onChange={event => { setCodeText(event.target.value); setCopied(false); setError(""); }} /></label><div><button type="button" aria-label="复制分享码" disabled={!shareCode.trim()} onClick={() => { void copyCode(); }}>复制分享码</button><button type="button" aria-label="导入分享码" disabled={locked || !shareCode.trim()} onClick={importCode}>导入分享码</button>{copied && <span role="status">已复制</span>}</div></details>
        {!catalog && <p role="status">正在读取路线和敌人资料</p>}
        {(error || issue) && <p className="pokepath-editor-error" role="alert">{error || issue}</p>}
        {worldBusy && <p className="pokepath-editor-wait">{view.saving ? "正在确认保存，请稍候" : view.waveActive ? "当前波次结束后可开始实践" : view.paused ? "继续游戏后可开始实践" : "结束当前挑战后可开始实践"}</p>}
      </div>
      <footer className="pokepath-editor-footer">{progression?.custom && <div><button type="button" aria-label="退出实践" disabled={locked || worldBusy} onClick={() => { void submit({ type: "custom_exit" }, true); }}>退出实践</button><button type="button" aria-label="重试实践" disabled={locked || worldBusy} onClick={() => { void submit({ type: "custom_retry" }, true); }}>重试实践</button></div>}<button type="button" className="primary" aria-label="开始实践" disabled={locked || worldBusy || !valid || Boolean(issue)} onClick={() => { if (valid) void submit({ type: "custom", settings: valid }, true); }}>开始实践</button></footer>
    </section>
  </div>);
}
