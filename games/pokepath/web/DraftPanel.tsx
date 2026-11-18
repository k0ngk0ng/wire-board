import { useEffect, useRef } from "react";
import { useFact, useLanguage } from "./I18n";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, Pokemon, View } from "./types";

function DraftOffer({ index, pokemon, assets, store, readOnly, pick, inspect }: { index: number; pokemon?: Pokemon; assets: Assets; store?: AssetStore; readOnly: boolean; pick: () => void; inspect: () => void }) {
  const fact = useFact();
  const language = useLanguage();
  const species = pokemon && assets.catalog?.species[pokemon.species];
  const displayName = pokemon?.alias || species?.names?.[language] || species?.names?.[0] || species?.name || "";
  const sprite = useResourceURL(pokemon?.sprite?.base ?? species?.sprite.base, store);
  const title = displayName.toUpperCase();
  const info = fact("text.ui.info").toUpperCase();
  return <article data-draft-slot={index} data-pokemon-id={pokemon?.id}>
    <button type="button" className="pokepath-draft-choice" aria-label={title || undefined} disabled={readOnly || !pokemon} onClick={pick} style={sprite ? { backgroundImage: `url("${sprite}")` } : undefined}><span>{title}</span></button>
    <button type="button" className="pokepath-draft-info" aria-label={title ? `${info}: ${title}` : info} disabled={!pokemon} onClick={inspect}><span>{info}</span></button>
  </article>;
}

export function DraftPanel({ view, assets, store, disabled, onAct, onDetails }: { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onDetails: (id: number) => void }) {
  const fact = useFact();
  const dialog = useRef<HTMLElement>(null);
  const draft = view.progression?.draft;
  const active = Boolean(draft?.active);
  useEffect(() => { if (active) dialog.current?.focus({ preventScroll: true }); }, [active]);
  if (!draft?.active) return null;
  const candidates = [...view.team, ...view.box];
  const title = fact("text.challenge.draft.title").toUpperCase();
  const readOnly = disabled || Boolean(view.paused);
  return <div className="pokepath-details-overlay pokepath-draft-overlay">
    <section ref={dialog} className="pokepath-draft" tabIndex={-1} role="dialog" aria-modal="true" aria-label={title} data-draft-token={draft.token} data-draft-font={view.progression?.settings?.font}>
      <header className="pokepath-draft-header"><span>{title}</span></header>
      <div className="pokepath-draft-body">
        <h2 className="pokepath-draft-round">{`${fact("text.challenge.draft.round").toUpperCase()} ${draft.round + 1}/${draft.rounds}`}</h2>
        <div className="pokepath-draft-offers">{Array.from({ length: 3 }, (_, index) => {
          const pokemon = candidates.find(candidate => candidate.id === draft.offers[index]);
          return <DraftOffer key={pokemon?.id ?? `empty-${index}`} index={index} pokemon={pokemon} assets={assets} store={store} readOnly={readOnly}
            pick={() => { if (pokemon && !readOnly) onAct({ type: "draftPick", pick: index, targetID: draft.token }); }}
            inspect={() => { if (pokemon) onDetails(pokemon.id); }} />;
        })}</div>
        <div className="pokepath-draft-actions">
          <button type="button" data-draft-reroll disabled={readOnly || !draft.canReroll} onClick={() => onAct({ type: "draftReroll", targetID: draft.token })}><span>{(fact("text.ui.reroll") || "Reroll").toUpperCase()}</span></button>
          <button type="button" data-draft-restart disabled={readOnly} onClick={() => onAct({ type: "draftRestart", targetID: draft.token })}><span>{(fact("text.ui.restart") || "Restart").toUpperCase()}</span></button>
        </div>
      </div>
    </section>
  </div>;
}
