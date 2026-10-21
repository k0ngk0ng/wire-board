import { useEffect, useState } from "react";
import { resolveBasketballAssetsBase } from "./asset-base";

export function SelectionImage({
  assets,
  file,
  alt,
  className,
}: {
  assets: string;
  file: string;
  alt: string;
  className?: string;
}) {
  const local = `/basketball/v1/selection/${file}`;
  const base = resolveBasketballAssetsBase(assets);
  const remote = base ? `${base}${local}` : local;
  const [source, setSource] = useState(remote);
  useEffect(() => setSource(remote), [remote]);
  return (
    <img
      className={className}
      src={source}
      alt={alt}
      draggable={false}
      onError={() => {
        if (source !== local) setSource(local);
      }}
    />
  );
}
