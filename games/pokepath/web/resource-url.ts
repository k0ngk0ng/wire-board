import { useEffect, useState } from "react";
import type { AssetStore } from "./assets";

export function useResourceURL(path?: string, store?: AssetStore) {
  const [url, setURL] = useState(() => path && store?.images.has(store.path(path)) ? store.url(path) : undefined);
  useEffect(() => {
    let alive = true;
    setURL(path && store?.images.has(store.path(path)) ? store.url(path) : undefined);
    if (path && store) void store.image(path).then(() => { if (alive) setURL(store.url(path)); }).catch(() => { if (alive) setURL(undefined); });
    return () => { alive = false; };
  }, [path, store]);
  return url;
}
