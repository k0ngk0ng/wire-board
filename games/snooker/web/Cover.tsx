import { useContext, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import { CoverArt } from "./CoverArt";
export function Cover() {
  const base = useContext(AssetsContext),
    [failed, setFailed] = useState(false);
  return base && !failed ? (
    <img
      src={`${base}/snooker/v1/cover.webp`}
      alt=""
      style={{
        width: "100%",
        height: "100%",
        objectFit: "contain",
        background: "#dce6d9",
      }}
      onError={() => setFailed(true)}
    />
  ) : (
    <CoverArt />
  );
}
