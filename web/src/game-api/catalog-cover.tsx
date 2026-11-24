import coverManifest from "../../../assets/game-covers.json";
import { publishedImageURL } from "../ui/published-image";

const covers: Record<string, { path: string; width: number; height: number }> =
  coverManifest.games;

export function catalogCover(kind: string) {
  const cover = covers[kind];
  if (!cover) return undefined;
  return function Cover() {
    return (
      <img
        src={publishedImageURL(cover.path)}
        alt=""
        width={cover.width}
        height={cover.height}
        style={{
          width: "100%",
          height: "100%",
          objectFit: "cover",
          display: "block",
        }}
      />
    );
  };
}
