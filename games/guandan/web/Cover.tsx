import { useContext, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import { CoverArt } from "./CoverArt";
export function Cover() {
  const base = useContext(AssetsContext);
  const [failed, setFailed] = useState(false);
  return base && !failed ? (
    <img
      src={`${base}/poker/v2/covers/guandan.webp`}
      alt=""
      style={{
        width: "100%",
        height: "100%",
        objectFit: "contain",
        background: "#dce5d5",
      }}
      onError={() => setFailed(true)}
    />
  ) : (
    <CoverArt />
  );
}
