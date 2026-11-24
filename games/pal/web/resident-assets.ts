import audioRelease from "../assets/audio-release.json";
import serverRelease from "../assets/server-release.json";
import { visualPublication } from "./visual-assets.ts";
import { endingPublication } from "./ending-assets.ts";
import type { PalView } from "./view";
import type { ResidentResource } from "../../../web/src/game-api/resident-assets";

type Publication = { cdnVerified?: boolean; files?: Record<string, ResidentResource> };
export function palResidentResources(view: Pick<PalView, "audio" | "local">): ResidentResource[] {
  const files = new Map<string, ResidentResource>();
  const add = (publication: Publication | undefined) => {
    if (!publication?.files || publication.cdnVerified === false) throw Error("此存档的资源版本尚未发布");
    for (const file of Object.values(publication.files)) files.set(file.url, { url: file.url, bytes: file.bytes });
  };
  add(visualPublication());
  add(endingPublication());
  if (view.audio) add((audioRelease.catalogs as Record<string, Publication>)[view.audio.assetID]);
  if (view.local) {
    const key = `cores/${view.local.core}.wasm`;
    const file = Object.values(serverRelease.catalogs).flatMap(catalog => Object.entries(catalog.files).filter(([path]) => path === key).map(([, file]) => file))[0];
    if (!file) throw Error("此存档的本地核心尚未发布");
    files.set(file.url, { url: file.url, bytes: file.bytes });
  }
  return [...files.values()];
}
