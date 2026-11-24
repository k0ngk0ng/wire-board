import publishedImages from "./published-images.json";

const images: Record<string, string> = publishedImages;

export function publishedImageURL(path: string) {
  if (!Object.hasOwn(images, path)) throw new Error(`未登记的图片：${path}`);
  const configured = import.meta.env.VITE_CDN_ORIGIN;
  if (!configured) {
    if (import.meta.env.PROD) throw new Error("缺少 CDN_ORIGIN 配置");
    return path;
  }
  const origin = new URL(configured);
  if (
    origin.protocol !== "https:" ||
    origin.pathname !== "/" ||
    origin.username ||
    origin.password ||
    origin.search ||
    origin.hash
  )
    throw new Error("CDN_ORIGIN 必须是 HTTPS 来源地址");
  return `${origin.origin}/${images[path]}`;
}
