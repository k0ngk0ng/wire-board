import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import { createPortal } from "react-dom";
import { useFact, useLanguage, useLocalize } from "./I18n";
import { RichText } from "./Panels";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, Definition, InventoryItem, Pokemon, View } from "./types";

type InventoryProps = { view: View; assets: Assets; store?: AssetStore; disabled: boolean; selected?: number; onAct: (action: Action) => void; onSelect?: (id: number) => void; onClose: () => void };
type Drag = { pointerID: number; button: HTMLButtonElement; itemID: string; x: number; y: number; active: boolean };
const categories = ["all", "power", "speed", "range", "critical", "utility"];
const sizes = [42, 47, 53, 60, 38];

function tierColor(tier?: string, assets?: Assets) {
  const key = `tier${tier ? tier[0].toUpperCase() + tier.slice(1) : ""}`;
  return assets?.catalog?.themes?.original?.[key] ? `var(--pp-${key})` : "#111";
}

function tierGradient(tier: string | undefined, assets: Assets, theme: string) {
  const key = `tier${tier ? tier[0].toUpperCase() + tier.slice(1) : ""}`;
  const color = assets.catalog?.themes?.[theme]?.[key] ?? "#111";
  const hex = color.slice(1).length === 3 ? [...color.slice(1)].map(value => value + value).join("") : color.slice(1);
  const dark = [0, 2, 4].map(offset => Math.floor(parseInt(hex.slice(offset, offset + 2), 16) * 0.75));
  return `linear-gradient(135deg, ${tierColor(tier, assets)}, rgb(${dark.join(", ")}))`;
}

function InventoryTeam({ pokemon, store, label, selected, dim, onSelect }: { pokemon?: Pokemon; store?: AssetStore; label: string; selected: boolean; dim: boolean; onSelect: () => void }) {
  const image = useResourceURL(pokemon?.sprite?.base, store);
  return <button type="button" data-inventory-pokemon={pokemon?.id} aria-label={label} aria-pressed={selected} disabled={!pokemon} onClick={onSelect} style={{ backgroundImage: image ? `url("${image}")` : undefined, filter: dim ? "brightness(0.55)" : undefined, opacity: dim ? 0.7 : undefined }} />;
}

function InventorySlot({ item, definition, owner, store, assets, theme, size, selected, index, label, onSelect, onDown, onMove, onUp, onCancel }: { item?: InventoryItem; definition?: Definition; owner?: Pokemon; store?: AssetStore; assets: Assets; theme: string; size: number; selected: boolean; index: number; label: string; onSelect: () => void; onDown: (event: ReactPointerEvent<HTMLButtonElement>) => void; onMove: (event: ReactPointerEvent<HTMLButtonElement>) => void; onUp: (event: ReactPointerEvent<HTMLButtonElement>) => void; onCancel: () => void }) {
  const image = useResourceURL(typeof definition?.sprite === "string" ? definition.sprite : undefined, store);
  const portrait = useResourceURL(owner?.sprite?.base, store);
  return <button type="button" data-inventory-slot={index} data-inventory-item={item?.id} className={selected ? "selected" : ""} aria-label={label} disabled={!item} onClick={onSelect} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerCancel={onCancel} onLostPointerCapture={onCancel} style={{ width: size, height: size, backgroundImage: item ? `${image ? `url("${image}"), ` : ""}${tierGradient(definition?.tier, assets, theme)}` : undefined }}>{owner && <span className="pokepath-inventory-owner" style={{ backgroundImage: portrait ? `url("${portrait}")` : undefined }} />}</button>;
}

export function InventoryPanel({ view, assets, store, disabled, onAct, onSelect, onClose }: InventoryProps) {
  const localize = useLocalize();
  const fact = useFact();
  const language = useLanguage();
  const root = useRef<HTMLElement>(null);
  const dragScene = useRef<HTMLElement | undefined>(undefined);
  const pending = useRef<Drag | undefined>(undefined);
  const dragCards = useRef<HTMLElement[]>([]);
  const suppressClick = useRef(false);
  const [search, setSearch] = useState("");
  const [searchOrder, setSearchOrder] = useState<string[]>();
  const [category, setCategory] = useState("all");
  const [sort, setSort] = useState(0);
  const [size, setSize] = useState(0);
  const [pokemonID, setPokemonID] = useState<number>();
  const [itemID, setItemID] = useState(view.items[0]?.id);
  const [drag, setDrag] = useState<{ itemID: string; x: number; y: number; scale: number }>();
  const theme = view.progression?.settings.theme ?? "original";
  const definitionFor = (id: string) => assets.catalog?.items[id];
  const nameFor = (definition?: Definition) => {
    const names = definition?.names;
    return Array.isArray(names) ? String(names[language] ?? names[0] ?? "") : Array.isArray(definition?.name) ? definition.name[language] ?? definition.name[0] : definition?.name ?? "";
  };
  const matchesName = (id: string, query: string) => {
    const definition = definitionFor(id);
    return query === "mega" ? Boolean(definition?.megaStone) : nameFor(definition).toLowerCase().startsWith(query);
  };
  const selected = view.items.find(item => item.id === itemID);
  const definition = itemID ? definitionFor(itemID) : undefined;
  const chosen = view.team.find(pokemon => pokemon.id === pokemonID);
  const query = search.toLowerCase().trim();
  const searchItems = searchOrder ? searchOrder.flatMap(id => { const item = view.items.find(item => item.id === id); return item ? [item] : []; }) : view.items;
  const items = searchItems.filter(item => {
    const definition = definitionFor(item.id);
    return matchesName(item.id, query) && (category === "all" || Array.isArray(definition?.tags) && definition.tags.includes(category)) && (!chosen || chosen.equipableItems?.includes(item.id));
  });
  const tier = (item: InventoryItem) => {
    const value = String(definitionFor(item.id)?.tier ?? "").toLowerCase();
    const index = ["common", "luxury", "rare", "epic", "legendary", "mega", "admin"].indexOf(value);
    return index < 0 ? Number.MAX_SAFE_INTEGER : index;
  };
  const acquisition = new Map(view.items.map((item, index) => [item.id, index]));
  if (sort !== 0) items.sort((first, second) => {
    if (sort <= 2) return (acquisition.get(first.id)! - acquisition.get(second.id)!) * (sort === 1 ? -1 : 1);
    if (sort <= 4) return nameFor(definitionFor(first.id)).normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().localeCompare(nameFor(definitionFor(second.id)).normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase(), undefined, { numeric: true, sensitivity: "base" }) * (sort === 3 ? 1 : -1);
    return (tier(first) - tier(second)) * (sort === 5 ? -1 : 1);
  });
  const sortLabel = sort === 0 ? fact("text.box.default") || "預設" : `${sort % 2 ? "↑" : "↓"} ${fact(sort <= 2 ? "text.inventory.acquisition" : sort <= 4 ? "text.box.alphabetical" : "text.inventory.tier")}`;
  const sizeIcon = useResourceURL("src/assets/images/icons/size.png", store);
  const ghost = useResourceURL(drag && typeof definitionFor(drag.itemID)?.sprite === "string" ? definitionFor(drag.itemID)!.sprite as string : undefined, store);
  const clearCards = () => {
    if (dragScene.current) delete dragScene.current.dataset.inventoryDrag;
    dragScene.current = undefined;
    for (const card of dragCards.current) delete card.dataset.inventoryDrop;
    dragCards.current = [];
  };
  const cancel = () => {
    const current = pending.current;
    pending.current = undefined;
    if (current?.active) suppressClick.current = true;
    if (current?.button.hasPointerCapture(current.pointerID)) current.button.releasePointerCapture(current.pointerID);
    clearCards();
    setDrag(undefined);
  };
  useEffect(() => () => {
    const current = pending.current;
    if (current?.button.hasPointerCapture(current.pointerID)) current.button.releasePointerCapture(current.pointerID);
    pending.current = undefined;
    clearCards();
  }, []);
  const down = (event: ReactPointerEvent<HTMLButtonElement>, id?: string) => {
    suppressClick.current = false;
    if (!id || disabled || !event.isPrimary || event.button !== 0 || (event.target as HTMLElement).closest(".pokepath-inventory-owner")) return;
    pending.current = { pointerID: event.pointerId, button: event.currentTarget, itemID: id, x: event.clientX, y: event.clientY, active: false };
    event.currentTarget.setPointerCapture(event.pointerId);
  };
  const move = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const current = pending.current;
    if (!current || event.pointerId !== current.pointerID || !current.active && Math.hypot(event.clientX - current.x, event.clientY - current.y) <= 6) return;
    current.active = true;
    const scene = root.current?.closest<HTMLElement>(".pokepath-classic-scene");
    const rect = scene?.getBoundingClientRect();
    if (!scene || !rect) return;
    if (scene.dataset.inventoryDrag !== current.itemID) {
      clearCards();
      scene.dataset.inventoryDrag = current.itemID;
      dragScene.current = scene;
      dragCards.current = [...scene.querySelectorAll<HTMLElement>("[data-party-slot]")];
    }
    const hit = document.elementFromPoint(event.clientX, event.clientY)?.closest("[data-party-slot]");
    for (const card of dragCards.current) {
      const pokemon = view.team.find(member => member.id === Number(card.dataset.pokemonId));
      card.dataset.inventoryDrop = pokemon?.equipableItems?.includes(current.itemID) ? card === hit ? "target" : "allowed" : "blocked";
    }
    const scale = rect.width / scene.offsetWidth || 1;
    setDrag({ itemID: current.itemID, x: (event.clientX - rect.left) / scale, y: (event.clientY - rect.top) / scale, scale });
  };
  const up = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const current = pending.current;
    if (!current || current.pointerID !== event.pointerId) return;
    if (current.active && !disabled) {
      setItemID(current.itemID);
      const target = document.elementFromPoint(event.clientX, event.clientY);
      const item = target?.closest<HTMLElement>("[data-inventory-item]")?.dataset.inventoryItem;
      const id = Number(target?.closest<HTMLElement>("[data-party-slot][data-pokemon-id]")?.dataset.pokemonId);
      const pokemon = view.team.find(pokemon => pokemon.id === id);
      if (item && item !== current.itemID) onAct({ type: "swapItems", itemID: current.itemID, name: item });
      else if (pokemon?.equipableItems?.includes(current.itemID)) onAct({ type: "equip", itemID: current.itemID, pokemonID: pokemon.id });
    }
    cancel();
  };
  const surface = root.current?.closest(".pokepath-classic-scene");
  return localize(<section ref={root} className="pokepath-central-panel pokepath-classic-inventory" data-pokepath-font={view.progression?.settings.font} role="dialog" aria-label="背包"><div className="pokepath-box-counts">{`ITEMS: ${view.items.length}/${view.progression?.profileStats?.itemTotal ?? 178}`}</div><div className="pokepath-inventory-team">{Array.from({ length: 10 }, (_, index) => {
    const member = view.team[index];
    const pokemon = member && { ...member, sprite: member.sprite ?? assets.catalog?.species[member.species].sprite };
    return <InventoryTeam key={pokemon?.id ?? `empty-${index}`} pokemon={pokemon} store={store} label={pokemon ? `筛选${pokemon.alias || assets.catalog?.species[pokemon.species]?.names?.[language] || pokemon.species}的可携带道具` : "空队伍位置"} selected={pokemon?.id === pokemonID} dim={Boolean(chosen && chosen.id !== pokemon?.id)} onSelect={() => { if (pokemon) { setPokemonID(previous => previous === pokemon.id ? undefined : pokemon.id); onSelect?.(pokemon.id); } }} />;
  })}</div><button type="button" className="pokepath-inventory-size" aria-label="切换道具图标大小" onClick={() => setSize((size + 1) % sizes.length)} style={{ backgroundImage: sizeIcon ? `url("${sizeIcon}")` : undefined }} /><div className="pokepath-inventory-grid">{Array.from({ length: Math.max(180, items.length) }, (_, index) => {
    const item = items[index];
    const owner = item?.equippedBy ? [...view.team, ...view.box].find(pokemon => pokemon.id === item.equippedBy) : undefined;
    return <InventorySlot key={item?.id ?? `empty-${index}`} item={item} definition={item ? definitionFor(item.id) : undefined} owner={owner && { ...owner, sprite: owner.sprite ?? assets.catalog?.species[owner.species].sprite }} store={store} assets={assets} theme={theme} size={sizes[size]} selected={Boolean(item && item.id === itemID)} index={index} label={item ? `选择${nameFor(definitionFor(item.id))}` : "空道具位置"} onSelect={() => { if (item && !suppressClick.current) setItemID(item.id); }} onDown={event => down(event, item?.id)} onMove={move} onUp={up} onCancel={cancel} />;
  })}</div><section className="pokepath-inventory-description" style={{ borderBottomColor: definition ? tierColor(definition.tier, assets) : undefined }}><strong data-i18n-literal style={{ backgroundColor: definition ? tierColor(definition.tier, assets) : undefined }}>{(definition ? nameFor(definition) : fact("text.inventory.noItem") || "無道具").toUpperCase()}</strong><p>{definition ? <RichText classic value={definition.descriptions as string[] ?? definition.description} enabled={view.progression?.settings.colorTags} /> : fact("text.inventory.noItemDescription") || "當前未選擇任何道具。"}</p>{selected && <button type="button" className="pokepath-inventory-equipped-action" disabled={disabled || !chosen || !chosen.equipableItems?.includes(selected.id)} onClick={() => chosen && onAct({ type: chosen.item === selected.id ? "unequip" : "equip", pokemonID: chosen.id, itemID: selected.id })}>{chosen?.item === selected.id ? "卸下" : "裝備"}</button>}</section><input className="pokepath-inventory-search" aria-label="搜索道具" maxLength={20} value={search} onInput={event => { const value = event.currentTarget.value; const query = value.toLowerCase().trim(); setSearch(value); setSearchOrder(view.items.filter(item => matchesName(item.id, query)).map(item => item.id)); }} /><div className="pokepath-inventory-filter"><button type="button" aria-label="上一种道具排序" onClick={() => setSort((sort + 6) % 7)}>{"<"}</button><div className="pokepath-inventory-sort-value" data-i18n-literal>{sortLabel.toUpperCase()}</div><button type="button" aria-label="下一种道具排序" onClick={() => setSort((sort + 1) % 7)}>{">"}</button></div><div className="pokepath-box-terrains">{categories.map(key => <button type="button" key={key} aria-pressed={category === key} onClick={() => setCategory(key)} data-i18n-literal>{fact(`text.inventory.${key}Tab`).toUpperCase()}</button>)}</div><button type="button" data-controller-cancel className="pokepath-central-close" aria-label="关闭背包" onClick={onClose}>×</button>{drag && surface && createPortal(<div className="pokepath-inventory-drag" style={{ left: drag.x - 22 / drag.scale, top: drag.y - 22 / drag.scale, width: 44 / drag.scale, height: 44 / drag.scale, filter: `drop-shadow(${4 / drag.scale}px ${4 / drag.scale}px ${1 / drag.scale}px #222)`, backgroundImage: ghost ? `url("${ghost}")` : undefined }} />, surface)}</section>);
}
