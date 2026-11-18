import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { useFact, useLanguage, useLocalize } from "./I18n";
import { RichText } from "./Panels";
import { dexDescription, dexName, dexPokemonStats, dexSorts, dexTerrain, dexTowerKeys, dexTowerRanks, tutorialArtAtBottom, unregisteredEnemies, type DexEntry, type DexMode } from "./dex-data";
import { formatGameNumber } from "./number-format";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, DexTowerStats, View } from "./types";

type DexProps = { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onClose: () => void };
const modes: { key: DexMode; title: string; fallback: string }[] = [{ key: "tutorials", title: "tutorials", fallback: "圖鑑" }, { key: "pokemon", title: "towers", fallback: "塔" }, { key: "enemies", title: "enemies", fallback: "敵人" }];

function DexSprite({ entry, store }: { entry: DexEntry; store?: AssetStore }) {
  const sprite = entry.pokemon?.sprite?.base || (typeof entry.definition.sprite === "string" ? entry.definition.sprite : entry.definition.sprite?.base);
  const target = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(false);
  const cached = Boolean(sprite && store?.images.has(store.path(sprite)));
  const url = useResourceURL(visible || cached ? sprite : undefined, store);
  useEffect(() => {
    if (!sprite || !store || !target.current || visible || cached) return;
    if (typeof IntersectionObserver === "undefined") { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => { if (entries.some(value => value.isIntersecting)) { setVisible(true); observer.disconnect(); } }, { rootMargin: "100px" });
    observer.observe(target.current);
    return () => observer.disconnect();
  }, [sprite, store, visible, cached]);
  return <span ref={target} className="pokepath-dex-sprite" aria-hidden="true" style={{ backgroundImage: url ? `url("${url}")` : undefined }} />;
}

function DexText({ value, colors, description = false }: { value: string; colors?: boolean; description?: boolean }) {
  let delay = 0;
  const format = (line: string) => line.split(/(\[(?:SHAKE|REVERSE)\][^]*?\[\/(?:SHAKE|REVERSE)\])/gi).map((part, index) => {
    if (/^\[REVERSE\]/i.test(part)) return <span key={index} className="pokepath-dex-reverse">{part.slice(9, -10)}</span>;
    if (/^\[SHAKE\]/i.test(part)) return <span key={index} className="pokepath-dex-shake">{part.slice(7, -8).split(/(\s+)/).map((word, index) => /^\s+$/.test(word) ? word : <span key={index} className="pokepath-dex-shake-word">{[...word].map((letter, index) => <span key={index} className="pokepath-dex-shake-letter" style={{ "--pp-shake-delay": `${delay++ * 0.04}s` } as CSSProperties}>{letter}</span>)}</span>)}</span>;
    return <RichText key={index} literal classic value={part} enabled={colors} tagHints={description} />;
  });
  return value.trim().split(/\r?\n/).map((value, index) => <span key={index} className={value.trim() ? "pokepath-dex-text-line" : "pokepath-dex-text-line empty"}>{index > 0 ? "\n" : ""}{format(value.trim())}</span>);
}

function DexDescription({ title, value, colors }: { title: string; value: string; colors?: boolean }) {
  if (!title && !value) return null;
  return <section className="pokepath-dex-description" data-i18n-literal><div>{title.toUpperCase()}</div><div><DexText value={value} colors={colors} description /></div></section>;
}

function DexIllustration({ path, store, bottom, secret, disabled, onSecret }: { path: string; store: AssetStore; bottom: boolean; secret: boolean; disabled: boolean; onSecret: () => void }) {
  const url = useResourceURL(path, store);
  const attributes = { className: `pokepath-dex-illustration ${bottom ? "bottom" : "top"}`, style: { backgroundImage: url ? `url("${url}")` : undefined } };
  return secret ? <button type="button" {...attributes} aria-label="查看秘密插图" disabled={disabled} onClick={onSecret} /> : <div {...attributes} />;
}

function DexData({ entry, mode, props, talk, question, ranks, onViewedSecret }: { entry: DexEntry; mode: DexMode; props: DexProps; talk: boolean; question: string; ranks?: Partial<Record<keyof DexTowerStats, number>>; onViewedSecret: () => void }) {
  const { view, assets, store, onAct, disabled } = props;
  const fact = useFact();
  const language = useLanguage();
  const colorTags = view.progression?.settings.colorTags;
  const catalog = assets.catalog;
  const rows: { label: string; value: string | number; extra?: boolean; rank?: number }[] = [];
  const row = (key: string, value: string | number, extra = false, rank?: number) => rows.push({ label: (fact(`text.${key}`) || key.split(".").at(-1) || "").toUpperCase(), value, extra, rank });
  const number = (value: number) => formatGameNumber(value, language);
  const description = (title: string, value: string) => <DexDescription title={title} value={value} colors={colorTags} />;
  const stance = (definition: unknown) => fact(definition ? "text.ui.yes" : "text.ui.no").toUpperCase();
  let details: ReactNode;
  if (mode === "tutorials") {
    const image = typeof entry.definition.sprite === "string" ? entry.definition.sprite : undefined;
    const secret = entry.key === "secrets" && !view.progression?.secrets?.tyrunt;
    const bottom = tutorialArtAtBottom.has(entry.key);
    const illustration = image && store && <DexIllustration path={image} store={store} bottom={bottom} secret={secret} disabled={disabled} onSecret={() => { onAct({ type: "tutorialSecret", name: "secrets" }); onViewedSecret(); }} />;
    return <div className="pokepath-dex-expanded pokepath-dex-tutorial">{!bottom && illustration}<p data-i18n-literal><DexText value={dexDescription(entry.definition, language)} colors={colorTags} /></p>{bottom && illustration}</div>;
  }
  if (mode === "pokemon") {
    const hint = !entry.pokemon && fact(`text.hints.${entry.key}.text`);
    if (talk && !entry.pokemon) details = description("UNOWN", question.toLowerCase());
    else if (hint) details = description(fact(`text.hints.${entry.key}.title`), hint);
    else {
      const stats = dexPokemonStats(entry);
      const shape = entry.pokemon?.combatRangeType ?? entry.species?.rangeType;
      row("pokemon.power", stats.power);
      row("pokemon.speed", `${(stats.speed / 1000).toFixed(2)}s`);
      row("pokemon.critical", `${stats.critical.toFixed(1)}%`);
      row("pokemon.range", stats.range);
      row("pokemon.rangeType", shape ? (fact(`text.pokemon.${shape}`) || shape).toUpperCase() : "-");
      row("pokemon.terrain", dexTerrain(entry.species?.terrains ?? [], language) || "-");
      if (entry.pokemon && view.items?.some(item => item.id === "noxDex")) {
        for (const key of dexTowerKeys) {
          const value = entry.pokemon.dex?.[key] ?? 0;
          const minutes = Math.floor(value / 60);
          const formatted = key === "active" ? `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}` : number(Math.floor(value));
          row(`pokemon.stadistics.${key}`, formatted, true, ranks?.[key]);
        }
      }
      const ability = catalog?.abilities[entry.species?.abilityId ?? ""];
      details = talk ? description("UNOWN", question.toLowerCase()) : description(dexName(ability, language), dexDescription(ability, language));
    }
  } else {
    const enemy = entry.definition;
    row("ui.health", number(Number(enemy.health ?? 0)));
    row("ui.armor", number(Number(enemy.armor ?? 0)));
    row("ui.speed", Number(enemy.speed ?? 0) / 60);
    row("ui.power", Number(enemy.power ?? 0));
    row("enemy.sort.regeneration", `${number(Number(enemy.regeneration ?? 0))}/s`);
    for (const [label, key] of [["stun", "canStun"], ["slow", "canSlow"], ["burn", "canBurn"], ["poison", "canPoison"], ["invisible", "invisible"]]) row(`ui.${label}`, stance(enemy[key]));
    if (view.items?.some(item => item.id === "thundersDrive")) {
      const stats = view.progression?.enemyStats?.[entry.key];
      row("enemy.stadistics.damage", number(Math.floor(stats?.damage ?? 0)), true);
      row("enemy.stadistics.defeated", number(stats?.defeated ?? 0), true);
      row("enemy.stadistics.escaped", number(stats?.escaped ?? 0), true);
    }
    const passive = catalog?.passives[String(enemy.passiveId ?? "")];
    details = description(dexName(passive, language), dexDescription(passive, language));
  }
  return <div className="pokepath-dex-expanded">{rows.length > 0 && <dl>{rows.map((row, index) => <div key={index} className={row.extra ? "extra" : undefined} data-rank={row.rank}><dt data-i18n-literal>{row.label}</dt><dd data-i18n-literal>{row.value}</dd></div>)}</dl>}{details}</div>;
}

export function DexPanel(props: DexProps) {
  const { view, assets, store, disabled, onAct, onClose } = props;
  const localize = useLocalize();
  const fact = useFact();
  const language = useLanguage();
  const [mode, setMode] = useState<DexMode>("tutorials");
  const [sort, setSort] = useState({ pokemon: 0, enemies: 0 });
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string>();
  const [answer, setAnswer] = useState("");
  const list = useRef<HTMLDivElement>(null);
  const selection = useRef<HTMLElement>(null);
  const catalog = assets.catalog;
  const settings = view.progression?.settings;
  const owns = new Map([...view.box, ...view.team].map(pokemon => [catalog?.species[pokemon.species]?.id, pokemon]));
  const entries: DexEntry[] = [];
  if (mode === "pokemon") {
    for (const key of catalog?.collection ?? []) {
      const species = catalog?.species[String(key)];
      if (!species) continue;
      const pokemon = owns.get(species.id);
      const fontSecret = species.key === "unown" && settings?.font === "Unown";
      if (species.key === "unown" && !pokemon && !fontSecret) continue;
      const form = pokemon ? catalog?.species[pokemon.species] ?? species : species;
      const name = dexName(form, language);
      const ability = dexName(catalog?.abilities[form.abilityId], language);
      entries.push({ key: species.key, definition: form, species: form, pokemon, name, discovered: Boolean(pokemon || fontSecret), search: [name.replace(/^m-/i, ""), ability, pokemon?.alias ?? ""] });
    }
  } else if (mode === "enemies") {
    for (const [key, definition] of Object.entries(catalog?.enemies ?? {})) {
      if (unregisteredEnemies.has(key)) continue;
      const name = dexName(definition, language);
      const defeated = view.progression?.enemyStats?.[key]?.defeated ?? 0;
      entries.push({ key, definition, name, discovered: defeated > 0, defeated, search: [name, key] });
    }
  } else {
    for (const definition of Object.values(catalog?.tutorials ?? {})) entries.push({ key: definition.key, definition, name: dexName(definition, language), discovered: true, search: [] });
  }
  const order = mode === "tutorials" ? [] : dexSorts(mode);
  const current = mode === "tutorials" ? undefined : order[sort[mode]];
  const query = search.trim().toLowerCase();
  const shown = mode === "tutorials" || !query ? entries : entries.filter(entry => entry.search.some(value => value.toLowerCase().includes(query)));
  if (current) shown.sort(current.compare);
  const talking = mode === "pokemon" && selected === "unown" && !view.progression?.secrets?.unown;
  const ranks = mode === "pokemon" && selected && view.items?.some(item => item.id === "noxDex") ? dexTowerRanks(entries.flatMap(entry => entry.pokemon ? [entry.pokemon] : [])) : undefined;
  const question = fact(`text.interfaceOptions.unownDialogue.${view.progression?.unownStage ?? 0}`) || view.progression?.unownQuestion || "";
  useLayoutEffect(() => {
    const pane = list.current;
    const entry = selection.current;
    if (!pane || !entry) return;
    const frame = requestAnimationFrame(() => { pane.scrollTop = Math.max(0, Math.min(entry.offsetTop - (pane.clientHeight - entry.offsetHeight) / 2, pane.scrollHeight - pane.clientHeight)); });
    return () => cancelAnimationFrame(frame);
  }, [selected]);
  const activate = (mode: DexMode) => { setMode(mode); setSelected(undefined); setAnswer(""); };
  const changeSort = (direction: number) => {
    if (mode !== "tutorials") { setSelected(undefined); setSort(value => ({ ...value, [mode]: (value[mode] + direction + order.length) % order.length })); }
  };
  const label = current ? `${current.direction === 1 ? "↑ " : current.direction === -1 ? "↓ " : ""}${fact(current.label)}` : fact("text.box.default") || "預設";
  return localize(<section className="pokepath-central-panel pokepath-classic-dex" role="dialog" aria-label="图鉴" data-dex-mode={mode} data-pokepath-font={settings?.font}><nav>{modes.map(tab => <button type="button" key={tab.key} aria-pressed={mode === tab.key} data-i18n-literal onClick={() => { if (mode !== tab.key) activate(tab.key); }}><span>{(fact(`text.files.${tab.title}`) || tab.fallback).toUpperCase()}</span></button>)}</nav><div ref={list} className={`pokepath-dex-list ${mode}`}>
    {shown.map(entry => {
      const expanded = entry.key === selected;
      const status = mode === "pokemon" ? entry.pokemon ? `Lv. ${entry.pokemon.level}` : "???" : entry.discovered ? fact("text.ui.registered").toUpperCase() : "???";
      return <article ref={expanded ? selection : undefined} key={entry.key} data-dex-species={mode === "pokemon" ? entry.key : undefined} data-dex-enemy={mode === "enemies" ? entry.key : undefined} className={`${entry.discovered ? "" : "locked"}${expanded ? " selected" : ""}`}><button type="button" aria-expanded={expanded} onClick={() => { setSelected(expanded ? undefined : entry.key); if (entry.key === "unown") { setSearch(""); setAnswer(""); } }}>{mode !== "tutorials" && <DexSprite entry={entry} store={store} />}<strong data-i18n-literal>{(entry.name || "???").toUpperCase()}</strong>{mode !== "tutorials" && <small data-i18n-literal>{status}</small>}</button>{expanded && <DexData entry={entry} mode={mode} props={props} talk={talking} question={question} ranks={entry.pokemon && ranks?.get(entry.pokemon.id)} onViewedSecret={() => setSelected(undefined)} />}</article>;
    })}
    {mode === "tutorials" && entries.length === 0 && <button type="button" disabled={disabled} onClick={() => onAct({ type: "tutorialSecret", name: "secrets" })}>秘密</button>}
  </div>{mode !== "tutorials" && <><form className="pokepath-dex-search" onSubmit={event => { event.preventDefault(); if (talking && !disabled && answer.trim()) { onAct({ type: "unown", name: answer, targetID: view.progression?.unownStage }); setAnswer(""); } }}><input type="text" aria-label={talking ? "未知图腾答案" : "搜索图鉴"} maxLength={20} value={talking ? answer : search} disabled={talking && disabled} onInput={event => { if (talking) setAnswer(event.currentTarget.value); else { setSearch(event.currentTarget.value); setSelected(undefined); } }} />{talking && <button className="pokepath-dex-submit" type="submit" aria-label="回答未知图腾" disabled={disabled}>↵</button>}</form><div className="pokepath-dex-sort"><button type="button" aria-label="上一种图鉴排序" onClick={() => changeSort(-1)}>&lt;</button><div className="pokepath-dex-sort-value" data-i18n-literal>{label.toUpperCase()}</div><button type="button" aria-label="下一种图鉴排序" onClick={() => changeSort(1)}>&gt;</button></div></>}<button type="button" data-controller-cancel className="pokepath-central-close" aria-label="关闭图鉴" onClick={onClose}>×</button></section>);
}
