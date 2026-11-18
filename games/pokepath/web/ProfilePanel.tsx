import { useFact, useLanguage, useLocalize } from "./I18n";
import { formatGameNumber } from "./number-format";
import { useEffect, useState } from "react";
import { ResourceImage, text } from "./Panels";
import type { AssetStore } from "./assets";
import type { Action, Assets, View } from "./types";

function ProfileAchievement({ index, path, store, label, description, unlocked, disabled, onSelect }: { index: number; path?: string; store?: AssetStore; label: string; description: string; unlocked: boolean; disabled: boolean; onSelect: () => void }) {
  const localize = useLocalize();
  const [url, setURL] = useState(() => path && store?.images.has(store.path(path)) ? store.url(path) : undefined);
  useEffect(() => {
    let alive = true;
    if (path && store) void store.image(path).then(() => { if (alive) setURL(store.url(path)); }).catch(() => { if (alive) setURL(undefined); });
    return () => { alive = false; };
  }, [path, store]);
  return localize(<button type="button" data-achievement={index} className={unlocked ? "unlocked" : ""} title={`${label}\n${description}`} aria-label={label} disabled={disabled} onClick={onSelect} style={{ backgroundImage: url ? `url("${url}")` : undefined }} />);
}

export function ProfilePanel({ view, assets, store, disabled, onAct, onClose }: { view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onClose: () => void }) {
  const localize = useLocalize();
  const language = useLanguage();
  const fact = useFact();
  const [name, setName] = useState<string>();
  const [picker, setPicker] = useState(false);
  const [record, setRecord] = useState<19 | 20>();
  const portrait = view.progression?.portrait ?? 0;
  const data = view.progression?.profileStats;
  const completionTotal = data?.pokemonTotal === undefined ? undefined : Math.max(0, data.pokemonTotal - 1);
  const numeric = (value?: number) => value === undefined ? "—" : formatGameNumber(value, language);
  const maximum = (value?: { value: number; routeTag: string }, rate = false) => value ? `${value.routeTag ? `(${value.routeTag}) ` : ""}$${numeric(value.value)}${rate ? "/s" : ""}` : "—";
  const stats: [string, string][] = [["游玩時間", data ? `${Math.floor(data.timePlayed / 60)}:${Math.floor(data.timePlayed % 60).toString().padStart(2, "0")}` : "—"], ["獲得星數", `${numeric(Math.min(data?.maxStars ?? 4000, data?.stars ?? view.stars))}/${numeric(data?.maxStars ?? 4000)}`], ["擁有的寶可夢", data ? `${numeric(data.pokemonOwned)}/${numeric(completionTotal)}` : "—"], ["閃光寶可夢", data ? `${numeric(data.shinyAmount)}/${numeric(completionTotal)}` : "—"], ["最高等級", data ? `${numeric(data.highestPokemonLevel)}/100` : "—"], ["總等級", data ? `${numeric(Math.min(data.pokemonOwned * 100, data.totalPokemonLevel))}/${numeric(completionTotal === undefined ? undefined : completionTotal * 100)}` : "—"], ["總金幣", data ? `$${numeric(data.totalGold)}` : "—"], ["已解鎖的道具", data ? `${numeric(data.itemAmount)}/${numeric(data.itemTotal)}` : "—"], ["完成波數", numeric(data?.wavesCompleted)], ["最高傷害", numeric(data?.highestHit)], ["擊敗敵人", numeric(data?.defeatedEnemies)], ["擊敗的種類", data ? `${formatGameNumber(Math.min(345, data.defeatedSpecies), 0)}/345` : "—"], ["眩暈次數", numeric(data?.appliedStuns)], ["減速次數", numeric(data?.appliedSlows)], ["灼燒次數", numeric(data?.appliedBurns)], ["中毒次數", numeric(data?.appliedPoisons)], ["詛咒次數", numeric(data?.appliedCurses)], ["重置次數", numeric(data?.resets)], ["被盜的賞金", data ? `$${numeric(data.stolenGold)}` : "—"], ["最大金幣／波次", maximum(data?.maxGoldPerWave)], ["最大金幣／時間", maximum(data?.maxGoldPerTime, true)]];
  const achievements = [...Array(assets.catalog?.achievements.length ?? 0).keys()];
  if (achievements.length === 39) achievements.splice(21, 0, achievements.splice(38, 1)[0]);
  const saveName = () => { if (name !== undefined && name !== view.progression?.profileName) onAct({ type: "profile", name }); };
  const changePortrait = (value: number) => { onAct({ type: "portrait", value: (value + 80) % 80 }); setPicker(false); };
  return localize(<section className="pokepath-central-panel pokepath-classic-profile" data-pokepath-font={view.progression?.settings?.font} role="dialog" aria-label="训练家"><div className="pokepath-profile-player"><button type="button" className="pokepath-profile-portrait" aria-label="选择训练家头像" disabled={disabled} onClick={() => setPicker(!picker)}>{store && <ResourceImage path={`src/assets/images/portraits/${portrait}.png`} store={store} />}</button><button type="button" className="previous" aria-label="上一个训练家头像" disabled={disabled} onClick={() => changePortrait(portrait - 1)}>{"<"}</button><button type="button" className="next" aria-label="下一个训练家头像" disabled={disabled} onClick={() => changePortrait(portrait + 1)}>{">"}</button><form onSubmit={event => { event.preventDefault(); saveName(); }}><input aria-label="训练家名称" value={name ?? ""} placeholder={view.progression?.profileName ?? "Player"} maxLength={11} disabled={disabled} onChange={event => setName(event.target.value)} onBlur={saveName} /></form>{picker && <div className="pokepath-profile-picker">{Array.from({ length: 80 }, (_, value) => <button type="button" key={value} aria-label={`训练家头像 ${value + 1}`} aria-pressed={portrait === value} disabled={disabled} onClick={() => changePortrait(value)}>{store && <ResourceImage path={`src/assets/images/portraits/${value}.png`} store={store} />}</button>)}</div>}</div><div className="pokepath-profile-badges">{achievements.map(index => { const achievement = assets.catalog!.achievements[index]; const image = typeof achievement.image === "string" ? achievement.image : typeof achievement.sprite === "string" ? achievement.sprite : undefined; return <ProfileAchievement key={index} index={index} path={image} store={store} label={text(achievement.name)} description={text(achievement.description)} unlocked={Boolean(view.progression?.achievements[String(index)])} disabled={disabled} onSelect={() => [7, 19].includes(index) && onAct({ type: "profileSecret", value: index })} />; })}</div><dl className="pokepath-profile-statistics">{stats.map(([label, value], index) => <div key={index} data-profile-stat={index}><dt data-i18n-literal>{(fact(`text.profile.stats.${index}`) || label).toUpperCase()}</dt><dd>{index >= 19 ? <button type="button" disabled={disabled} onClick={() => setRecord(index as 19 | 20)}>{value}</button> : value}</dd></div>)}</dl><button type="button" data-controller-cancel className="pokepath-central-close" aria-label="关闭训练家" onClick={onClose}>×</button>{record !== undefined && <div className="pokepath-profile-record-confirm" role="alertdialog" aria-label="清除最高纪录"><strong>{stats[record][0]}</strong><p>清除此項最高紀錄？</p><div><button type="button" disabled={disabled} onClick={() => { onAct({ type: "profileRecordReset", value: record }); setRecord(undefined); }}>清除紀錄</button><button type="button" onClick={() => setRecord(undefined)}>取消</button></div></div>}</section>);
}
