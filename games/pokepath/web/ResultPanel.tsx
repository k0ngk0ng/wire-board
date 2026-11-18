import { useEffect, useRef } from "react";
import { useFact, useLanguage, useTranslate } from "./I18n";
import { useResourceURL } from "./resource-url";
import { resultBoss, routeRetryMessage, routeVictoryMessage } from "./result-facts";
import type { AssetStore } from "./assets";
import type { Action, Assets, Challenge, View } from "./types";

const ruleKeys = ["levelCap", "slotLimit", "toughEnemies", "draft", "noItems", "permadeath", "mirror"] as const;
const icons = "src/assets/images/icons/";

export function hasRouteResult(phase?: string) {
  return phase === "route_complete" || phase === "defeat" || phase === "practice_complete";
}

function ResultSprite({ className, path, store }: { className: string; path?: string; store?: AssetStore }) {
  const url = useResourceURL(path, store);
  return <span className={className} aria-hidden="true" style={url ? { backgroundImage: `url("${url}")` } : undefined} />;
}

function ChallengeResult({ rules, victory }: { rules: Challenge; victory: boolean }) {
  const fact = useFact();
  return <div className="pokepath-result-rules">{(victory ? ruleKeys : ruleKeys.slice(0, 6)).map(key => {
    const value = rules[key];
    const title = fact(`text.challenge.${key === "levelCap" ? "lvlCap" : key}.title`);
    const display = !value ? fact("text.challenge.off") : key === "levelCap" ? `LEVEL ${value}` : key === "slotLimit" ? `${value} SLOTS` : key === "toughEnemies" ? `+${value}%` : fact("text.challenge.on");
    return <div key={key} className="pokepath-result-rule" data-enabled={Boolean(value)}>{`${title} — ${display}`.toUpperCase()}</div>;
  })}</div>;
}

export function ResultPanel({ view, assets, store, disabled, onAct }: { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void }) {
  const fact = useFact();
  const language = useLanguage();
  const translate = useTranslate();
  const panel = useRef<HTMLElement>(null);
  const open = hasRouteResult(view.phase);
  useEffect(() => { if (open) panel.current?.focus({ preventScroll: true }); }, [open, view.phase]);
  if (!open) return null;
  const victory = view.phase === "route_complete";
  const practice = view.phase === "practice_complete";
  const ruleSet = view.progression?.challenge;
  const showRules = ruleSet?.active && (victory || ruleSet.permadeath);
  const theme = view.progression?.settings.theme;
  const font = view.progression?.settings.font;
  const boss = assets.catalog?.enemies[resultBoss[view.route]];
  const names = boss?.names as string[] | undefined;
  const bossName = names?.[language] || names?.[0] || (typeof boss?.name === "string" ? boss.name : "");
  const spooky = theme === "halloween" || view.route === 46;
  const ghost = spooky ? "ghost-halloween.png" : practice && view.health > 0 ? "ghost-happy.png" : "ghost.png";
  const art = victory ? boss?.sprite.base : icons + ghost;
  const retry = !practice && view.wave >= 25;
  const retryWave = view.wave === 100 ? 100 : Math.min(75, Math.floor(view.wave / 25) * 25);
  const lives = view.wave === 100 ? 1 : 7 + [150, 300, 450].filter(stars => view.stars >= stars).length;
  const title = fact(victory ? "text.final.title" : "text.defeat.title").toUpperCase();
  const prompt = victory ? fact("text.final.prompt") : `${fact("text.defeat.prompt")} ${view.wave} ${fact("text.map.waves")}`;
  const message = victory ? routeVictoryMessage(language, bossName) : showRules ? fact("text.defeat.promptChallengeLost") : practice ? fact(view.health > 0 ? "text.editable.win" : "text.editable.lose") : retry ? routeRetryMessage(language, retryWave, lives) : fact("text.defeat.cantRetry");
  return <div className="pokepath-details-overlay pokepath-result-overlay">
    <section className={`pokepath-result ${victory ? "victory" : "defeat"}${showRules ? " has-rules" : ""}`} ref={panel} tabIndex={-1} role="dialog" aria-modal="true" aria-label={title} data-route-result={victory ? "victory" : practice ? "practice" : "defeat"} data-result-font={font}>
      <header className="pokepath-result-header"><span className="pokepath-result-title">{title}</span>{victory && <button type="button" data-result-continue data-controller-cancel aria-label={translate("关闭")} disabled={disabled} onClick={() => onAct({ type: "continueRoute" })}>X</button>}</header>
      <div className="pokepath-result-body">
        <p className="pokepath-result-prompt">{prompt.toUpperCase()}</p>
        <ResultSprite className={`pokepath-result-portrait${!victory && spooky ? " spooky" : ""}`} path={art} store={store} />
        {victory && <div className="pokepath-result-team">{view.team.map(pokemon => <ResultSprite key={pokemon.id} className="pokepath-result-member" path={pokemon.sprite?.base ?? assets.catalog?.species[pokemon.species]?.sprite.base} store={store} />)}</div>}
        {!victory && <>
          <button type="button" data-result-restart disabled={disabled} onClick={() => onAct(practice ? { type: "custom_retry" } : { type: "retry", mode: "restart" })}><span>{fact("text.defeat.restart").toUpperCase()}</span></button>
          <button type="button" data-result-retry disabled={disabled || !retry} onClick={() => onAct({ type: "retry", mode: "retry" })}><span>{fact("text.defeat.retry").toUpperCase()}</span></button>
        </>}
        <p className="pokepath-result-text">{message.toUpperCase()}</p>
        {ruleSet && showRules && <ChallengeResult rules={ruleSet} victory={victory} />}
      </div>
    </section>
  </div>;
}
