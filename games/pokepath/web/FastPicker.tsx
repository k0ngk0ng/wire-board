import { useLocalize } from "./I18n";
import { Portrait, ResourceImage, text } from "./Panels";
import type { AssetStore } from "./assets";
import type { Action, Assets, Pokemon, View } from "./types";

export function FastPicker({ mode, pokemon, view, assets, store, disabled, onAct, onClose }: { mode: "pokemon" | "item"; pokemon?: Pokemon; view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onClose: () => void }) {
  const localize = useLocalize();
  const items = view.items.filter(item => pokemon?.equipableItems?.includes(item.id));
  return localize(<div className={`pokepath-fast-picker ${mode}`} data-fast-picker role="dialog" aria-label={mode === "pokemon" ? "快速选择伙伴" : "快速选择道具"} style={mode === "item" && pokemon ? { backgroundColor: `${assets.catalog?.species[pokemon.species]?.color ?? "#777777"}33` } : undefined}>
    {mode === "pokemon" ? view.box.length ? view.box.map(candidate => <button type="button" key={candidate.id} data-pokemon-id={candidate.id} aria-label={`加入${assets.catalog?.species[candidate.species]?.name ?? candidate.species}`} disabled={disabled} onClick={() => { onAct({ type: "team", pokemonID: candidate.id, mode: "team" }); onClose(); }}><Portrait pokemon={candidate} assets={assets} store={store} /></button>) : <span className="pokepath-fast-empty">空位</span> : <>{items.map(item => { const definition = assets.catalog?.items[item.id]; return <button type="button" key={item.id} data-item-id={item.id} aria-label={`装备${text(definition?.name) || item.id}`} title={`${text(definition?.name)}\n${text(definition?.description)}`} disabled={disabled || !pokemon} onClick={() => { if (pokemon) onAct({ type: "equip", pokemonID: pokemon.id, itemID: item.id }); onClose(); }}>{store && typeof definition?.sprite === "string" && <ResourceImage path={definition.sprite} store={store} />}{(item.equippedBy ?? 0) > 0 && <span className="equipped">E</span>}</button>; })}<button type="button" aria-label="卸下道具" disabled={disabled || !pokemon?.item} onClick={() => { if (pokemon) onAct({ type: "unequip", pokemonID: pokemon.id }); onClose(); }}>×</button></>}
  </div>);
}
