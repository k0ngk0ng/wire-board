import { hasPublishedImage, publishedImageURL } from "../ui/published-image";

export function catalogCover(kind: string) {
  const path = `/game-covers/${kind}.png`;
  if (!hasPublishedImage(path)) return undefined;
  return function Cover() {
    return (
      <img
        src={publishedImageURL(path)}
        alt=""
        width={480}
        height={285}
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
