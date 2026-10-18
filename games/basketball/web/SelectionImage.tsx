import { useEffect, useState } from "react";

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
  const remote = assets ? `${assets.replace(/\/$/, "")}${local}` : local;
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
