import { useEffect, useRef } from "react";
import type { CSSProperties } from "react";
import { imageFor, spriteAction, imagePixelScale } from "./assets";

export function Sprite({
  graphic,
  direction = 3,
  action = 3,
  time = 0,
  progress,
  dead = false,
  size = 120,
}: {
  graphic: number;
  direction?: number;
  action?: number;
  time?: number;
  progress?: number;
  dead?: boolean;
  size?: number;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current,
      animation = spriteAction(graphic, direction, dead ? 2 : action);
    if (!canvas || !animation?.frames.length) return;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) return;
    ctx.setTransform(imagePixelScale, 0, 0, imagePixelScale, 0, 0);
    const frames = animation.frames;
    const index = dead && progress === undefined
      ? frames.length - 1
      : progress === undefined
        ? Math.floor(time / ((Math.max(1, animation.ticks) * 1000) / 60)) %
          frames.length
        : Math.min(frames.length - 1, Math.floor(progress * frames.length));
    const frame = frames[index],
      image = imageFor(frame.file);
    ctx.clearRect(0, 0, size, size);
    if (!image) return;
    const extent = Math.max(
      ...frames.map((f) => Math.max(f.width, Math.abs(f.y) + 8)),
    );
    const scale = Math.min(1.15, (size - 8) / extent);
    ctx.imageSmoothingEnabled = true;
    ctx.drawImage(
      image,
      size / 2 + frame.x * scale,
      size - 12 + frame.y * scale,
      frame.width * scale,
      frame.height * scale,
    );
    canvas.dataset.stoneageSpriteFrame = frame.file;
  }, [graphic, direction, action, time, progress, dead, size]);
  return (
    <canvas
      ref={ref}
      width={size * imagePixelScale}
      height={size * imagePixelScale}
      data-stoneage-pixel-scale={imagePixelScale}
      style={{ "--sa-logical-size": `${size}px` } as CSSProperties}
      aria-hidden="true"
      className="sa-sprite"
    />
  );
}
