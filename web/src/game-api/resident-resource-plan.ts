import type { ResidentResource } from "./resident-assets";

export function residentResourcePlan(kind: string, media: Readonly<Record<string, number>>, paths: readonly string[], additional?: readonly ResidentResource[]): ResidentResource[] {
  const files = new Map<string, ResidentResource>();
  const add = (file: ResidentResource) => {
    if (!file.url || !Number.isSafeInteger(file.bytes) || file.bytes <= 0) throw Error("资源目录缺少有效大小");
    const previous = files.get(file.url);
    if (previous && previous.bytes !== file.bytes) throw Error("资源目录存在冲突");
    files.set(file.url, file);
  };
  const selected = !paths.length && additional === undefined ? Object.keys(media).filter(path => path.startsWith("/" + kind + "/")) : paths;
  for (const url of selected) add({ url, bytes: media[url] });
  for (const file of additional ?? []) add(file);
  return [...files.values()];
}
