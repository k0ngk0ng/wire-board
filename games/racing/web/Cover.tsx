import { useContext, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import { coverSVG } from "../assets/art";
import { assetURL } from "./types";
export function Cover() {
  const base = useContext(AssetsContext),
    [failed, setFailed] = useState(false);
  return (
    <img
      alt=""
      src={
        base && !failed
          ? assetURL(base, "cover.webp")
          : `data:image/svg+xml;charset=utf-8,${encodeURIComponent(coverSVG)}`
      }
      onError={() => setFailed(true)}
      style={{ width: "100%", height: "100%", objectFit: "cover" }}
    />
  );
}
