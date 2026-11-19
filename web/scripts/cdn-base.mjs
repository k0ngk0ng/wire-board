export function assetBase(value, origin = process.env.CDN_ORIGIN) {
  if (!value || value === "/") return "/";
  const url = new URL(value);
  if (url.protocol !== "https:" || url.pathname !== "/bga/" || url.username || url.password || url.search || url.hash || (origin && url.origin !== origin.replace(/\/$/, "")))
    throw new Error(
      "Expected the configured stable CDN bga root",
    );
  return value;
}
export function assetURL(base, file) {
  return base + file.replace(/^\//, "");
}
