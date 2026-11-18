import { useEffect, useRef, type RefObject } from "react";
import type { PalInput } from "./input";
import type { PalView } from "./view";
import { PalScreenInput } from "./screen-input";

export function PalControls({
  canvas,
  input,
  data,
  enabled,
}: {
  canvas: RefObject<HTMLCanvasElement | null>;
  input: PalInput;
  data: PalView;
  enabled: () => boolean;
}) {
  const latest = useRef({ data, enabled });
  latest.current = { data, enabled };
  useEffect(() => {
    if (!canvas.current) return;
    const surface = new PalScreenInput(
      canvas.current,
      input,
      () => latest.current.data,
      () => latest.current.enabled(),
    );
    return () => surface.dispose();
  }, [canvas, input]);
  return null;
}
