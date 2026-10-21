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
  return (
    <img
      className={className}
      src={remote}
      alt={alt}
      draggable={false}

    />
  );
}
