import { codeMediaBase } from "../../../web/src/media-assets";
export function releaseAssetBase(fallback: string) {
  const configured = import.meta.env?.BASE_URL;
  return typeof configured === "string" ? codeMediaBase(configured).replace(/\/$/, "") : fallback;
}
export function localAudioFixture() {
  return !import.meta.env || import.meta.env.VITE_YUYU_LOCAL_ASSETS === "1";
}
