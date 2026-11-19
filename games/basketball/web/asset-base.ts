import release from "../assets/release-manifest.json" with { type: "json" };




export function resolveBasketballAssetsBase(platformBase: string): string {
  const base = platformBase.replace(/\/+$/, "");
  if (!base || base === "/" || base.endsWith("/_media")) return "/bga";
  try {
    const url = new URL(base);
    if (url.origin === new URL(release.assetsBaseURL).origin && !url.username && !url.password)
      return "/bga";
  } catch {
  }
  return base;
}
