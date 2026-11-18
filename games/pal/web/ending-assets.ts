import release from "../assets/ending-release.json";
import type { EndingPublication } from "./ending-renderer";

const assetID = "35ea7fdcb0d19772be93a4dd246e475f8318f993a95270e7f0c6ed3942d15245";

export function endingPublication(): EndingPublication {
  const publication = (release as { version: number; catalogs: Record<string, EndingPublication & { cdnVerified: boolean }> }).catalogs[assetID];
  if (!publication?.cdnVerified || publication.assetID !== assetID || publication.files["index.json"]?.sha256 !== assetID) throw Error("原版结局资源尚未发布");
  return publication;
}
