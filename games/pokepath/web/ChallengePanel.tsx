import { useState } from "react";
import { useFact, useLanguage, useTranslate } from "./I18n";
import { ResourceImage, RichText, type Panel } from "./Panels";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, Challenge, Definition, View } from "./types";

export const emptyChallenge = (): Challenge => ({ levelCap: 0, slotLimit: 0, toughEnemies: 0, draft: false, noItems: false, permadeath: false, mirror: false });

export function challengeLocksPanel(view: View, panel: Panel): boolean {
  const progression = view.progression;
  const rules = progression?.challenge;
  if (panel === "challenge") return !progression?.canChallenge && !rules?.active;
  if (!rules?.active) return false;
  return panel === "routes" || panel === "box" && rules.draft || panel === "inventory" && rules.noItems;
}

const rules = [
  { key: "levelCap", text: "lvlCap", values: [0, 75, 50], annotation: "warning" },
  { key: "slotLimit", text: "slotLimit", values: [0, 8, 6] },
  { key: "toughEnemies", text: "toughEnemies", values: [0, 25, 50] },
  { key: "draft", text: "draft", values: [false, true], annotation: "bossDebuff" },
  { key: "noItems", text: "noItems", values: [false, true], annotation: "warning" },
  { key: "permadeath", text: "permadeath", values: [false, true] },
  { key: "mirror", text: "mirror", values: [false, true] }
] as const;
const icons = "src/assets/images/icons/";
type Rule = (typeof rules)[number];
type ChallengeProps = { view: View; assets: Assets; store?: AssetStore; disabled: boolean; plan?: Challenge; onPlan?: (plan: Challenge) => void; onAct: (action: Action) => void; onClose: () => void };

function Reward({ slot, title, details, asset, threshold, lock, claimed, readOnly, noColors, store, onClaim }: {
  slot: number; title: string; details: string; asset?: string; threshold: number; lock: boolean; claimed: boolean; readOnly: boolean; noColors: boolean; store?: AssetStore; onClaim: () => void;
}) {
  const fact = useFact();
  const url = useResourceURL(asset, store);
  const [inspect, setInspect] = useState(false);
  const tooltip = `pokepath-challenge-reward-tip-${slot}`;
  const closed = !lock && claimed;
  return <div className="pokepath-challenge-reward-position" onMouseEnter={() => setInspect(true)} onMouseLeave={() => setInspect(false)}>
    <button type="button" data-challenge-reward={slot} className="pokepath-challenge-reward" data-unavailable={lock || closed || undefined} data-reward-claimed={closed || undefined} aria-disabled={readOnly || lock || closed} aria-label={title} aria-describedby={inspect ? tooltip : undefined} style={{ backgroundImage: url ? `url("${url}")` : undefined }} onFocus={() => setInspect(true)} onBlur={() => setInspect(false)} onClick={() => { if (!readOnly && !lock && !closed) { setInspect(false); onClaim(); } }}>
      <span className="pokepath-challenge-claim">{lock ? <>{threshold}{store && <ResourceImage path={`${icons}ribbon1.png`} store={store} />}</> : fact(closed ? "text.challenge.claimed" : "text.challenge.claim")}</span>
    </button>
    {inspect && !closed && <div className="pokepath-challenge-reward-tip" id={tooltip} role="tooltip"><strong>{title}</strong><p><RichText value={details} literal enabled={!noColors} classic /></p></div>}
  </div>;
}

function localized(value: Definition["name"] | undefined, language: number, fallback = "") {
  return typeof value === "string" ? value : value?.[language] || value?.[0] || fallback;
}

export function ChallengePanel({ view, assets, store, disabled, plan, onPlan, onAct, onClose }: ChallengeProps) {
  const fact = useFact();
  const language = useLanguage();
  const translate = useTranslate();
  const [fallback, setFallback] = useState(emptyChallenge);
  const [selected, setSelected] = useState(0);
  const current = view.progression;
  const active = Boolean(current?.challenge?.active);
  const draft = plan ?? fallback;
  const shown = active ? current!.challenge : draft;
  const setPlan = onPlan ?? setFallback;
  const route = assets.catalog?.routes[String(view.route)] ?? assets.route;
  const routeName = route?.names?.[language] || route?.names?.[0] || route?.name || "";
  const routeRibbons = current?.ribbons?.[String(view.route)] ?? {};
  const claims = current?.claims?.[String(view.route)] ?? [];
  const ribbonCount = current?.currentRibbons ?? 0;
  const chosen = rules[selected];
  const annotation = "annotation" in chosen ? chosen.annotation : undefined;
  const ribbon = useResourceURL(`${icons}ribbon1.png`, store);
  const goldRibbon = useResourceURL(`${icons}ribbon3.png`, store);
  const canSubmit = !disabled && !view.paused && (active || Boolean(current?.canChallenge) && Object.values(draft).some(Boolean));
  const describe = (rule: Rule, part: string) => fact(`text.challenge.${rule.text}.${part}`);
  const ruleValue = (rule: Rule) => {
    const value = shown[rule.key];
    if (!value) return fact("text.challenge.off");
    if (rule.key === "levelCap") return `${describe(rule, "level")} ${value}`;
    if (rule.key === "slotLimit") return `${value} ${describe(rule, "slots")}`;
    if (rule.key === "toughEnemies") return `+${value}%`;
    return fact("text.challenge.on");
  };
  const advance = (rule: Rule, index: number) => {
    setSelected(index);
    if (disabled || active) return;
    const options: readonly (number | boolean)[] = rule.values;
    setPlan({ ...draft, [rule.key]: options[(options.indexOf(draft[rule.key]) + 1) % options.length] });
  };
  const rewards = (route?.challengeRewards ?? []).map((key, slot) => ({ key, slot, threshold: current?.rewardThresholds?.[slot] ?? [2, 3, 5, 1][slot] })).filter(reward => reward.threshold !== undefined).sort((first, second) => first.threshold - second.threshold);
  return <section className="pokepath-central-panel pokepath-classic-challenge" data-pokepath-theme={current?.settings.theme} role="dialog" aria-label={fact("text.challenge.title")}>
    <h2 className="pokepath-challenge-route">{routeName.toUpperCase()} ({ribbonCount}/10)</h2>
    <div className="pokepath-challenge-rules">{rules.map((rule, index) =>
      <button key={rule.key} type="button" className="pokepath-challenge-rule" data-challenge-rule={rule.key} aria-disabled={disabled || active} onMouseEnter={() => setSelected(index)} onFocus={() => setSelected(index)} onClick={() => advance(rule, index)}>
        <span className="pokepath-challenge-label">{describe(rule, "title")}</span><span className="pokepath-challenge-result" data-enabled={Boolean(shown[rule.key])}>{ruleValue(rule)}</span>
        <span className="pokepath-challenge-ribbons" aria-hidden="true">{Array.from({ length: rule.values.length - 1 }, (_, tier) =>
          <span className={(routeRibbons[rule.key] ?? 0) > tier ? "earned" : ""} key={tier} style={(tier === 0 ? ribbon : goldRibbon) ? { backgroundImage: `url("${tier === 0 ? ribbon : goldRibbon}")` } : undefined} />
        )}</span>
      </button>
    )}</div>
    <section className="pokepath-challenge-info">
      <h2>{describe(chosen, "title")}</h2>
      <p>{describe(chosen, "description")}{annotation && <><br /><span className={annotation === "warning" ? "pokepath-challenge-warning" : "pokepath-challenge-debuff"}>{fact(`text.challenge.${annotation}`)}</span></>}</p>
    </section>
    <button type="button" data-challenge-submit disabled={!canSubmit} title={!active && !current?.canChallenge ? current?.challengeReason : undefined} onClick={() => { if (canSubmit) { onAct(active ? { type: "surrender" } : { type: "challenge", settings: draft }); onClose(); } }}>
      <span>{fact(active ? "text.challenge.surrender" : "text.challenge.start")}</span>
    </button>
    <div className="pokepath-challenge-rewards">{rewards.map(({ key, slot, threshold }) => {
      const species = slot === 1 ? assets.catalog?.species[key] : undefined;
      const item = slot === 1 ? undefined : assets.catalog?.items[key];
      const owned = species ? [...view.team, ...view.box].some(pokemon => assets.catalog?.species[pokemon.species]?.id === species.id) : Boolean(item?.equipable && (view.items.some(entry => entry.id === key) || current?.unlocks?.[key]));
      const claimed = owned || Boolean(claims[slot] && !species && !item?.equipable);
      return <Reward key={slot} slot={slot} title={species ? fact("text.shop.eggName") : localized(item?.names as string[] | undefined ?? item?.name, language, key)} details={species ? fact("text.challenge.egg") : localized(item?.descriptions as string[] | undefined ?? item?.description, language)} asset={slot === 1 ? `${icons}egg.png` : typeof item?.sprite === "string" ? item.sprite : undefined} threshold={threshold} lock={ribbonCount < threshold} claimed={claimed} readOnly={disabled} noColors={current?.settings.colorTags === false} store={store} onClaim={() => onAct({ type: "claimReward", slot })} />;
    })}</div>
    <button type="button" className="pokepath-central-close" data-controller-cancel aria-label={translate("关闭挑战")} onClick={onClose}>×</button>
  </section>;
}
