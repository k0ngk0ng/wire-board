export type MapViewport = {
  handleEvent: (event: Event) => boolean;
  logicalPoint: (clientX: number, clientY: number) => { x: number; y: number };
  pointToClient: (x: number, y: number) => { x: number; y: number };
  fit: () => void;
  fill: () => void;
  reset: () => void;
  update: () => void;
  zoomBy: (factor: number, point?: { x: number; y: number }) => void;
  cancel: () => void;
  destroy: () => void;
  readonly state: { scale: number; width: number; height: number; scrollLeft: number; scrollTop: number; fitted: boolean };
};
export function attachMapViewport(options: { canvas: HTMLCanvasElement; wrapper: HTMLElement; onChange?: (state: MapViewport["state"]) => void; minimumScale?: number; maximumScale?: number; dragThreshold?: number; initialMode?: "native" | "fit" | "fill" }): MapViewport;
