import { codeMediaBase } from "../../../web/src/media-assets";

type RuntimeBuildMeta = ImportMeta & { env?: { BASE_URL?: string } };
const runtimeAssetBase = codeMediaBase((import.meta as RuntimeBuildMeta).env?.BASE_URL || "/");

export function runtimeManifestURL(
  runtimeBuildID: string,
  pageURL: string,
): string {
  const base = new URL(runtimeAssetBase.replace(/\/?$/, "/"), pageURL);
  return new URL(`contra/runtime-${runtimeBuildID}/manifest.json`, base).href;
}
