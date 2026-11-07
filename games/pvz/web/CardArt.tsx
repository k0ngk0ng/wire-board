import { useEffect, useRef } from "react";
import { previewFrame, type Art } from "./art";

export function CardArt({ art, kind, lazy = false }: { art: Art; kind: string; lazy?: boolean }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const controller = new AbortController();
    const draw = async () => {
      const context = canvas.current?.getContext("2d");
      if (!context) return;
      try {
        const selected = await previewFrame(art, kind, controller.signal);
        if (!selected || controller.signal.aborted) return;
        const { image, frame } = selected;
        const scratch = document.createElement("canvas"); scratch.width = frame.w; scratch.height = frame.h;
        const scan = scratch.getContext("2d", { willReadFrequently: true });
        let crop = { x: 0, y: 0, w: frame.w, h: frame.h };
        if (scan) {
          scan.drawImage(image, frame.x, frame.y, frame.w, frame.h, 0, 0, frame.w, frame.h);
          try {
            const pixels = scan.getImageData(0, 0, frame.w, frame.h).data;
            let x1 = frame.w, y1 = frame.h, x2 = -1, y2 = -1;
            for (let y = 0; y < frame.h; y++) for (let x = 0; x < frame.w; x++) if (pixels[(y * frame.w + x) * 4 + 3] > 4) { x1 = Math.min(x1, x); y1 = Math.min(y1, y); x2 = Math.max(x2, x); y2 = Math.max(y2, y); }
            if (x2 >= x1) crop = { x: x1, y: y1, w: x2 - x1 + 1, h: y2 - y1 + 1 };
          } catch { /* The original frame remains usable when CORS prevents scanning. */ }
        }
        const scale = Math.min(80 / crop.w, 82 / crop.h), w = crop.w * scale, h = crop.h * scale;
        context.clearRect(0, 0, 100, 90);
        context.drawImage(image, frame.x + crop.x, frame.y + crop.y, crop.w, crop.h, (100 - w) / 2, 86 - h, w, h);
      } catch { if (!controller.signal.aborted && canvas.current) canvas.current.dataset.loadError = "true"; }
    };
    let observer: IntersectionObserver | undefined;
    if (lazy && canvas.current) {
      observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { observer?.disconnect(); void draw(); } }, { rootMargin: "100px" });
      observer.observe(canvas.current);
    } else void draw();
    return () => { controller.abort(); observer?.disconnect(); };
  }, [art, kind, lazy]);
  return <canvas ref={canvas} width={100} height={90} aria-hidden="true" />;
}
