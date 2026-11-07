/** Vite's immutable application base owns Wasm and decoded scene resources.
 * Explicit local fixtures keep their existing root and never fetch old app code. */
export function releaseAssetBase(fallback: string) {
  const configured = import.meta.env?.BASE_URL;
  return typeof configured === "string" ? configured.replace(/\/$/, "") : fallback;
}
export function localAudioFixture() {
  return !import.meta.env || import.meta.env.VITE_YUYU_LOCAL_ASSETS === "1";
}
