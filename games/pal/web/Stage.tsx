import { useLayoutEffect, useRef, type ReactNode, type Ref } from "react";
import "./stage.css";

export function PalStage({
  canvasRef,
  controls,
  children,
  label = "仙剑奇侠传",
}: {
  canvasRef?: Ref<HTMLCanvasElement>;
  controls?: ReactNode;
  children?: ReactNode;
  label?: string;
}) {
  const stage = useRef<HTMLDivElement>(null);
  const hasControls = !!controls;
  useLayoutEffect(() => {
    const node = stage.current;
    if (!node) return;
    const fit = () => {
      const css = getComputedStyle(node);
      const width =
        node.clientWidth -
        parseFloat(css.paddingLeft) -
        parseFloat(css.paddingRight);
      const height =
        node.clientHeight -
        parseFloat(css.paddingTop) -
        parseFloat(css.paddingBottom);
      node.style.setProperty(
        "--pal-canvas-width",
        `${Math.max(0, Math.min(width, height * 1.6))}px`,
      );
    };
    fit();
    const observer = new ResizeObserver(fit);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  return (
    <div className="pal-stage" ref={stage} data-pal-stage>
      <div className="pal-stage-screen">
        <canvas
          className="pal-stage-canvas"
          ref={canvasRef}
          width={320}
          height={200}
          tabIndex={0}
          aria-label={label}
        />
        {hasControls && <div className="pal-stage-controls">{controls}</div>}
        {children && <div className="pal-stage-overlay">{children}</div>}
      </div>
    </div>
  );
}
