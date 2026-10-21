import release from "../assets/release-manifest.json" with { type: "json" };

/** Keep platform-wide configuration untouched. Empty uses the pinned OSS release;
 * explicit mirrors/harness roots survive, while the production CDN selects
 * this game's pinned immutable object namespace. */
export function resolveBasketballAssetsBase(platformBase: string): string {
  const base = platformBase.replace(/\/+$/, "");
  if (!base) return release.assetsBaseURL;
  try {
    const url = new URL(base);
    if (url.host === "cdn.ichenj.com" && ["http:", "https:"].includes(url.protocol) && !url.username && !url.password)
      return release.assetsBaseURL;
  } catch {
    // Relative local roots are intentional in previews and test harnesses.
  }
  return base;
}
