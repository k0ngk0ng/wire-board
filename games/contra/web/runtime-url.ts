type RuntimeBuildMeta = ImportMeta & { env?: { BASE_URL?: string } };

// Vite binds this root to the application commit. Standalone esbuild fixtures
// have no Vite environment and serve the same layout from their local root.
const runtimeAssetBase = (import.meta as RuntimeBuildMeta).env?.BASE_URL || "/";

export function runtimeManifestURL(
  runtimeBuildID: string,
  pageURL: string,
): string {
  const base = new URL(runtimeAssetBase.replace(/\/?$/, "/"), pageURL);
  return new URL(`contra/runtime-${runtimeBuildID}/manifest.json`, base).href;
}
