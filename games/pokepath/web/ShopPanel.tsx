import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { useFact, useLanguage, useTranslate } from "./I18n";
import { RichText } from "./Panels";
import { formatGameNumber } from "./number-format";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, Pokemon, View } from "./types";

const icon = "src/assets/images/icons/";
const soldArt = "src/assets/images/items/poke.png";
const tierColors: Record<string, string> = { pokemon: "Pokemon", common: "Common", uncommon: "Uncommon", luxury: "Luxury", epic: "Epic", legendary: "Legendary", mega: "Mega", admin: "Admin" };
type TileInfo = { slot: number; price: number; sold: boolean; art: string; name: string; description: string; tint: string; affordable: boolean; action: Action };
type Tip = { slot: number; x: number; y: number };
type Position = { clientX: number; clientY: number };

const local = (value: unknown, language: number): string => Array.isArray(value)
  ? String(value[language] || value[0] || "")
  : typeof value === "string" ? value : "";

function ShopTile({ entry, store, disabled, soldLabel, onInspect, onClear, onBuy }: { entry: TileInfo; store?: AssetStore; disabled: boolean; soldLabel: string; onInspect: (entry: TileInfo, button: HTMLButtonElement, point?: Position) => void; onClear: () => void; onBuy: (action: Action) => void }) {
  const language = useLanguage();
  const link = useResourceURL(entry.art, store);
  const unavailable = disabled || !entry.affordable || entry.sold;
  const price = entry.sold ? soldLabel : `$${formatGameNumber(entry.price, language)}`;
  return <button type="button" className={`pokepath-shop-tile${entry.slot === -1 ? " pokepath-shop-egg" : " pokepath-shop-slot"}`} data-shop-stock={entry.slot < 0 ? undefined : entry.slot} data-shop-item={entry.action.itemID as string | undefined} data-shop-unaffordable={!entry.affordable || undefined} data-resource={entry.art} aria-label={entry.sold ? soldLabel : `${entry.name} ${price}`} aria-disabled={unavailable} disabled={entry.sold} style={link ? { backgroundImage: `url("${link}")` } : undefined}
    onFocus={event => onInspect(entry, event.currentTarget)}
    onBlur={onClear}
    onPointerMove={event => { if (event.pointerType === "mouse") onInspect(entry, event.currentTarget, event); }}
    onPointerLeave={onClear}
    onClick={() => { if (!unavailable) { onClear(); onBuy(entry.action); } }}>
    <strong className="pokepath-shop-price">{price}</strong>
  </button>;
}

export function ShopPanel({ view, assets, store, disabled, onAct, onClose }: { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onClose: () => void }) {
  const fact = useFact();
  const language = useLanguage();
  const translate = useTranslate();
  const shop = view.progression;
  const offers = shop?.shop ?? [];
  const eggCount = shop?.remainingEggs ?? 0;
  const eggPrice = Math.min(50000, shop?.eggPrice ?? 0);
  const sold = fact("text.shop.sold").toUpperCase() || "售罄";
  const area = useRef<HTMLElement>(null);
  const tooltip = useRef<HTMLDivElement>(null);
  const [tip, setTip] = useState<Tip>();
  const entries: TileInfo[] = [{
    slot: -1, price: eggPrice, sold: eggCount === 0, art: icon + "egg.png",
    name: fact("text.shop.eggName") || "蛋", description: fact("text.shop.egg"), tint: "var(--pp-tierPokemon)",
    affordable: eggCount > 0 && view.gold >= eggPrice, action: { type: "egg", quote: eggCount }
  }];
  for (let index = 0; index < 10; index++) {
    const offer = offers.find(entry => entry.stock === index);
    const key = offer?.itemID;
    const item = key && assets.catalog?.items[key];
    entries.push({
      slot: index, price: offer?.price ?? 0, sold: !key, art: offer?.sprite || soldArt,
      name: (item ? local(item.names ?? item.name, language) : "") || offer?.name || key || "",
      description: (item ? local(item.descriptions ?? item.description, language) : "") || offer?.description || "",
      tint: tierColors[offer?.tier ?? ""] ? `var(--pp-tier${tierColors[offer!.tier]})` : "#111",
      affordable: Boolean(key && offer?.affordable), action: { type: "buyItem", itemID: key, stock: index }
    });
  }
  const inspect = (entry: TileInfo, button: HTMLButtonElement, point?: Position) => {
    const bounds = area.current?.getBoundingClientRect();
    if (entry.sold || !bounds || !area.current || bounds.width <= 0) return;
    const rect = button.getBoundingClientRect();
    const scale = bounds.width / area.current.clientWidth;
    const x = ((point?.clientX ?? rect.left + rect.width / 2) - bounds.left) / scale;
    const y = ((point?.clientY ?? rect.top) - bounds.top) / scale;
    setTip({ slot: entry.slot, x, y });
  };
  const entry = tip && entries[tip.slot + 1];
  useLayoutEffect(() => {
    const node = tooltip.current;
    const parent = area.current;
    if (!node || !parent || !tip) return;
    const x = Math.max(4, Math.min(parent.clientWidth - node.offsetWidth - 4, tip.x + 10 - node.offsetWidth / 2));
    const above = tip.y - node.offsetHeight - 13;
    const y = Math.max(4, Math.min(parent.clientHeight - node.offsetHeight - 4, above >= 4 ? above : tip.y + 25));
    node.style.transform = `translate(${x}px, ${y + parent.scrollTop}px)`;
  }, [tip, entry?.name, entry?.description]);
  const mapMode = shop?.settings?.mapEffectsMode;
  const animated = mapMode === undefined ? shop?.settings?.mapEffects !== false : mapMode !== 0;
  return <section className="pokepath-central-panel pokepath-classic-shop" role="dialog" aria-label={fact("text.shop.title") || "商店"} ref={area} data-shop-theme={shop?.settings?.theme} data-shop-font={shop?.settings?.font} data-shop-animated={animated || undefined} onScroll={() => setTip(undefined)}>
    <div className="pokepath-shop-slots">{entries.map(current => <ShopTile key={current.slot} entry={current} store={store} disabled={disabled || view.paused} soldLabel={sold} onInspect={inspect} onClear={() => setTip(undefined)} onBuy={onAct} />)}</div>
    {entry && !entry.sold && <div className="pokepath-shop-tooltip" role="tooltip" data-shop-tooltip ref={tooltip} style={{ "--pp-shop-tooltip-accent": entry.tint } as CSSProperties}><strong>{entry.name.toUpperCase()}</strong><RichText value={entry.description} literal enabled={shop?.settings?.colorTags !== false} classic /></div>}
    <button type="button" data-controller-cancel className="pokepath-central-close" aria-label={translate("关闭商店")} onClick={onClose}>×</button>
  </section>;
}

const dusk = (hex: string) => /^#[0-9a-f]{6}$/i.test(hex) ? `rgb(${[1, 3, 5].map(offset => Math.floor(Number.parseInt(hex.slice(offset, offset + 2), 16) * .75)).join(", ")})` : hex;

export function HatchPanel({ pokemon, assets, store, font, onClose }: { pokemon: Pokemon; assets: Assets; store?: AssetStore; font?: string; onClose: () => void }) {
  const fact = useFact();
  const language = useLanguage();
  const dialog = useRef<HTMLElement>(null);
  const species = assets.catalog?.species[pokemon.species];
  const name = (local(species?.names ?? species?.name, language) || pokemon.species).toUpperCase();
  const color = species?.color ?? "#a33635";
  const background = dusk(color);
  const image = useResourceURL(pokemon.sprite?.base ?? species?.sprite.base, store);
  const title = fact("text.shop.pokemon").toUpperCase();
  useEffect(() => { dialog.current?.focus({ preventScroll: true }); }, [pokemon.id]);
  return <div className="pokepath-details-overlay pokepath-hatch-overlay" onClick={event => { if (event.target === event.currentTarget) onClose(); }}>
    <section className="pokepath-hatch" ref={dialog} tabIndex={-1} role="dialog" aria-modal="true" aria-label={title} data-shop-hatch={pokemon.id} data-shop-font={font} style={{ "--pp-hatch-color": background } as CSSProperties}>
      <header className="pokepath-hatch-header"><span className="pokepath-hatch-title">{title}</span></header>
      <div className="pokepath-hatch-body">
        <span className="pokepath-hatch-prompt">{fact("text.shop.new").toUpperCase()}</span>
        <strong className="pokepath-hatch-name" style={{ color }}>{name}</strong>
        <span className="pokepath-hatch-art" aria-hidden="true" style={image ? { backgroundImage: `url("${image}")` } : undefined} />
        <button type="button" data-hatch-close data-controller-cancel onClick={onClose}><span>OK</span></button>
      </div>
    </section>
  </div>;
}
