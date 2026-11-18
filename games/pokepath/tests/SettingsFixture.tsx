import { createRoot } from "react-dom/client";
import { useState } from "react";
import { SettingsPanel } from "../web/SettingsPanel";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, Settings, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View }; parameters: { language: string; theme: string; font: string } };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
const progression = data.input.view.progression!;
const actions: Action[] = [];
Reflect.set(window, "settingReceipts", actions);

function Fixture() {
  const [closed, setClosed] = useState(false);
  const [disabled, setDisabled] = useState(false);
  const [settings, setSettings] = useState({ ...progression.settings, font: data.parameters.font, language: Number(data.parameters.language), theme: data.parameters.theme });
  Reflect.set(window, "setSettingsReadOnly", setDisabled);
  const apply = (action: Action) => {
    actions.push(action);
    const [field, key] = String(action.name).split(".");
    const value = action.enabled ?? action.value ?? action.mode;
    setSettings(current => {
      if (!key) return { ...current, [field]: value };
      if (field === "mute") return { ...current, mute: current.mute?.map((muted, index) => index === Number(key) ? Boolean(value) : muted) };
      return { ...current, [field]: { ...current[field as keyof Settings] as object, [key]: value } };
    });
  };
  return <section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={settings.language} dictionary={data.dictionary} catalog={data.catalog}>{!closed && <SettingsPanel view={{ ...data.input.view, progression: { ...progression, settings } }} disabled={disabled} onAct={apply} onClose={() => setClosed(true)} />}</I18nProvider></div></section>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
