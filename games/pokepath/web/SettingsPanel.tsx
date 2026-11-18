import { Fragment, type ReactNode } from "react";
import { useFact, useLanguage, useTranslate } from "./I18n";
import { creditDisclaimer, creditGroups, officialCredits, productionCredits, type CreditRow } from "./settings-credits";
import type { Action, Settings, View } from "./types";
import type { GameControllerBindings } from "../../../web/src/game-api/module";

const languages = ["English", "Español", "Français", "Português", "Italiano", "Deutsch", "日本語", "한국어", "繁體中文", "Polski"];
const themes = ["original", "classic", "monochrome", "halloween"];
const fonts = ["PressStart2P", "Rubik", "Lexend", "DynaPuff", "Unown"];
const effects = ["projectiles", "areas", "beams", "waves", "auras", "bombardment", "special"] as const;
const audioChannels = [["master", 12], ["music", 12], ["ui", 8], ["effects", 8]] as const;
const muteOrder = [0, 3, 4, 1, 2];
type SetValue = (name: string, value: string | number | boolean) => void;

function Card({ name, title, children }: { name: string; title: string; children?: ReactNode }) {
  return <section className="pokepath-settings-card" data-settings-section={name}><h2>{title}</h2><div className="pokepath-settings-content">{children}</div>{name === "data" && <small className="pokepath-settings-version">v 1.7.3</small>}</section>;
}

function Cycle<T extends string | number>({ name, label, options, value, display, disabled, onSet }: { name: string; label: string; options: readonly T[]; value: T; display: (option: T, index: number) => string; disabled: boolean; onSet: SetValue }) {
  const index = Math.max(0, options.indexOf(value));
  const move = (step: number) => onSet(name, options[(index + step + options.length) % options.length]);
  return <div className="pokepath-settings-row" data-setting-name={name}>
    <span className="pokepath-setting-label">{label}</span>
    <button type="button" className="pokepath-setting-arrow" data-setting-previous={name} aria-label={`${label} ‹`} disabled={disabled} onClick={() => move(-1)}><span>&lt;</span></button>
    <output className="pokepath-setting-value" aria-label={label}>{display(options[index], index)}</output>
    <button type="button" className="pokepath-setting-arrow" data-setting-next={name} aria-label={`${label} ›`} disabled={disabled} onClick={() => move(1)}><span>&gt;</span></button>
  </div>;
}

function Options({ name, label, options, value, disabled, onSet }: { name: string; label: string; options: readonly string[]; value: boolean | number; disabled: boolean; onSet: SetValue }) {
  return <div className="pokepath-settings-row pokepath-setting-options" data-setting-name={name}>
    <span className="pokepath-setting-label">{label}</span>
    <div className="pokepath-setting-buttons" role="group" aria-label={label}>
      {options.map((option, index) => <button type="button" key={index} aria-pressed={Number(value) === index} disabled={disabled} onClick={() => onSet(name, typeof value === "boolean" ? Boolean(index) : index)}><span>{option}</span></button>)}
    </div>
  </div>;
}

function Slider({ id, name, label, value, percentage, disabled, onSet }: { id: string; name: string; label: string; value: number; percentage: boolean; disabled: boolean; onSet: SetValue }) {
  return <div className="pokepath-settings-row pokepath-setting-range" data-setting-name={name}>
    <label className="pokepath-setting-label" htmlFor={id}>{label}</label>
    <input id={id} className="pokepath-setting-slider" type="range" min={0} max={percentage ? 100 : 20} step={percentage ? 5 : 1} value={value} disabled={disabled} onChange={event => onSet(name, event.currentTarget.valueAsNumber)} />
    <output className="pokepath-setting-value" aria-hidden="true">{value}{percentage ? "%" : ""}</output>
  </div>;
}

function graphics(settings?: Settings): readonly [fact: string, name: string, value: number | boolean][] {
  return [
    ["showDamage", "showDamage", settings?.showDamage ?? true],
    ["autoStop", "autoStop", settings?.autoStop ?? false],
    ["autoStopBoss", "autoStopBoss", settings?.autoStopBoss ?? false],
    ["autoReset", "autoReset", settings?.autoReset ?? 0],
    ["displayHealth", "displayHealth", settings?.displayHealth ?? (settings?.showHealth ? 2 : 0)],
    ["mapEffects", "mapEffectsMode", settings?.mapEffectsMode ?? (settings?.mapEffects === false ? 0 : 1)],
    ["indicatorField", "indicatorField", settings?.indicatorField ?? false],
    ["indicatorShape", "indicatorShape", settings?.indicatorShape ?? false],
    ["fastTarget", "fastTarget", settings?.fastTarget ?? false],
    ["durationCC", "durationCC", settings?.durationCC ?? false],
    ["particles", "particles", settings?.particles ?? true],
    ["disableTagColors", "disableTagColors", settings?.disableTagColors ?? (settings?.colorTags === false)]
  ];
}

const keyNames: Readonly<Record<string, string>> = {
  ShiftLeft: "Shift", ShiftRight: "Shift", ControlLeft: "Ctrl", ControlRight: "Ctrl", AltLeft: "Alt", AltRight: "Alt",
  MetaLeft: "⌘", MetaRight: "⌘", ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→", Backspace: "⌫",
  PageUp: "PgUp", PageDown: "PgDn", BracketLeft: "[", BracketRight: "]", Semicolon: ";", Quote: "'", Backslash: "\\",
  Comma: ",", Period: ".", Slash: "/", Backquote: "`", Minus: "−", Equal: "="
};

const shortcutCodes = [
  ["profile", "KeyQ"], ["routes", "KeyW"], ["box", "KeyE"], ["shop", "KeyS"], ["damage", "KeyZ"], ["ranges", "KeyX"], ["cancel", "Escape"],
  ["confirm", "Enter"], ["speed", "ShiftLeft"]
] as const;

function Shortcuts({ bindings }: { bindings?: GameControllerBindings }) {
  const fact = useFact();
  const standard = (index: number) => fact(`text.interfaceOptions.shortcuts.key.${index}`);
  const formatKey = (code: string) => code === "Space" ? standard(8) : code === "Escape" ? standard(7) : code === "ShiftLeft" || code === "ShiftRight" ? standard(9) : /^(Key[A-Z]|Digit[0-9])$/.test(code) ? code.replace(/^(Key|Digit)/, "") : Object.hasOwn(keyNames, code) ? keyNames[code] : code || "—";
  const group = (index: number, prefix: string, defaults: string[]) => {
    if (!bindings) return standard(index);
    const codes = defaults.map((value, offset) => bindings.keyboard[`${prefix}${offset}`] ?? value);
    return codes.every((code, offset) => code === defaults[offset]) ? standard(index) : codes.map(formatKey).join(" / ");
  };
  const label = (index: number) => {
    if (index === 0) return group(index, "select", Array.from({ length: 10 }, (_, slot) => `Digit${(slot + 1) % 10}`));
    if (index === 11) return group(index, "load", ["F1", "F2", "F3", "F4", "F5"]);
    const mapping = shortcutCodes[index - 1];
    if (!mapping || !bindings) return standard(index);
    const [action, defaultCode] = mapping;
    const code = bindings.keyboard[action] ?? defaultCode;
    if (action === "confirm" && code === "Enter") return bindings.keyboardAlt?.confirm?.includes("Space") ? standard(index) : formatKey(code);
    return code === defaultCode ? standard(index) : formatKey(code);
  };
  return <div className="pokepath-settings-shortcuts">{Array.from({ length: 13 }, (_, index) =>
    <div className="pokepath-settings-row" key={index}><kbd className="pokepath-setting-label">{label(index)}</kbd><span className="pokepath-setting-detail">{fact(`text.interfaceOptions.shortcuts.do.${index}`)}</span></div>
  )}</div>;
}

const inLanguage = (values: readonly string[], language: number) => values[language] || values[0];

function CreditsRow({ row, language, compact }: { row: CreditRow; language: number; compact: boolean }) {
  return <div className={compact ? "pokepath-settings-row" : "pokepath-settings-credit"}>
    <span className="pokepath-setting-label">{inLanguage(row.labels, language)}</span>
    <span className="pokepath-setting-detail">{row.names.map(([name, href], index) => <Fragment key={`${index}-${name}`}>{index > 0 && <br />}{href ? <a href={href} target="_blank" rel="noopener noreferrer">{name}</a> : name}</Fragment>)}</span>
  </div>;
}

export function SettingsPanel({ view, disabled, controllerBindings, onAct, onClose }: { view: View; disabled: boolean; controllerBindings?: GameControllerBindings; onAct: (action: Action) => void; onClose: () => void }) {
  const fact = useFact();
  const language = useLanguage();
  const translate = useTranslate();
  const settings = view.progression?.settings;
  const title = (section: string) => fact(`text.menu.${section}.title`);
  const label = (name: string) => fact(`text.menu.settings.${name}`);
  const onSet: SetValue = (name, value) => {
    if (!disabled) onAct({ type: "settings", name, ...(typeof value === "string" ? { mode: value } : typeof value === "number" ? { value } : { enabled: value }) });
  };
  const controls = { disabled, onSet };
  return <section className="pokepath-central-panel pokepath-classic-settings" role="dialog" aria-label={title("settings")} data-settings-font={settings?.font ?? "PressStart2P"}>
    <div className="pokepath-settings-list">
      <Card name="official" title={title("official")}>
        {officialCredits.map(row => <CreditsRow key={row.labels[0]} row={row} language={language} compact />)}
      </Card>
      <Card name="settings" title={title("settings")}>
        <Cycle name="language" label={label("language")} options={languages.map((_, index) => index)} value={settings?.language ?? language} display={index => languages[index]} {...controls} />
        <Cycle name="theme" label={label("theme")} options={themes} value={settings?.theme ?? "original"} display={(_, index) => label(`themes.${index}`)} {...controls} />
        <Cycle name="font" label={label("font")} options={fonts} value={settings?.font ?? "PressStart2P"} display={(_, index) => label(`fonts.${index}`)} {...controls} />
        {graphics(settings).map(([key, name, value]) => <Options key={name} name={name} label={label(`${key}.label`)} options={Array.from({ length: typeof value === "boolean" ? 2 : 3 }, (_, index) => label(`${key}.${index}`))} value={value} {...controls} />)}
        {effects.map(key => <Slider key={key} id={`pokepath-effect-${key}`} name={`effectOpacity.${key}`} label={fact(`text.interfaceOptions.effectOpacityLabels.${key}`)} value={settings?.effectOpacity?.[key] ?? (key === "auras" ? 50 : 100)} percentage {...controls} />)}
      </Card>
      <Card name="audio" title={title("audio")}>
        {audioChannels.map(([key, fallback], index) => <Slider key={key} id={`pokepath-volume-${key}`} name={`audio.${key}`} label={label(`audio.${index}`)} value={settings?.audio[key] ?? fallback} percentage={false} {...controls} />)}
        {muteOrder.map(index => <Options key={index} name={`mute.${index}`} label={index < 3 ? label(`mute.${index}`) : fact(`text.interfaceOptions.muteLabels.${index}`)} options={["OFF", "ON"]} value={Boolean(settings?.mute?.[index])} {...controls} />)}
      </Card>
      <Card name="shortcuts" title={title("shortcuts")}><Shortcuts bindings={controllerBindings} /></Card>
      <Card name="data" title={title("data")} />
      <Card name="credits" title={title("credits")}>
        {productionCredits.map((row, index) => <CreditsRow key={row.labels[0]} row={row} language={language} compact={index === 1} />)}
        {creditGroups.map(group => <section className="pokepath-settings-credit-group" key={group.titles[0]}><h3>{inLanguage(group.titles, language)}</h3>{group.rows.map(row => <CreditsRow key={row.labels[0]} row={row} language={language} compact={false} />)}</section>)}
        <div className="pokepath-settings-disclaimer">{(creditDisclaimer[language] ?? creditDisclaimer[0]).map(paragraph => <p key={paragraph}>{paragraph}</p>)}</div>
      </Card>
    </div>
    <button type="button" data-controller-cancel className="pokepath-central-close" aria-label={translate("关闭设置")} onClick={onClose}>×</button>
  </section>;
}
