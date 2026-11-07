import type { Progress } from "./types";

// Resource capacities use the same bytes as their verified manifests. Progress
// weights reserve initialization time; they never determine download sizes.
export function combineLoadingProgress(
  runtime: Progress,
  assets: Progress,
  label: string,
): Progress {
  const known = [runtime, assets].filter(
    (value) => value.total !== undefined && value.loaded !== undefined,
  );
  return {
    percent: runtime.percent * 0.35 + assets.percent * 0.55,
    label,
    ...(known.length
      ? {
          loaded: known.reduce((sum, value) => sum + value.loaded!, 0),
          total: known.reduce((sum, value) => sum + value.total!, 0),
        }
      : {}),
  };
}

export function loadingStage(
  previous: Progress,
  percent: number,
  label: string,
): Progress {
  return { ...previous, percent, label };
}
