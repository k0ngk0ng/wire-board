import release from "../assets/visual-release.json";
import type { VisualPublication } from "./scene-renderer";

const assetID = "997f43b90cbcb6432137d46dfa29ad5c5cfcfbf8a2d151aff933a3a9cfae2c1c";

export function visualPublication(): VisualPublication {
  const publication = (release as { version: number; catalogs: Record<string, VisualPublication & { cdnVerified: boolean }> }).catalogs[assetID];
  if (!publication?.cdnVerified || publication.assetID !== assetID || publication.files["index.json"]?.sha256 !== assetID) throw Error("原版仙剑素材尚未发布");
  return publication;
}
