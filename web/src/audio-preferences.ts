export function readAudioPreferences(storage?: Pick<Storage, "getItem">) {
  try {
    const source = storage ?? localStorage;
    const raw = source.getItem("wb_volume");
    const value = raw?.trim() ? Number(raw) : NaN;
    return {
      enabled: source.getItem("wb_sound") !== "off",
      volume: Number.isFinite(value) && value >= 0 && value <= 1 ? value : 0.5,
    };
  } catch {
    return { enabled: true, volume: 0.5 };
  }
}
