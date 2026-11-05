export function entryAutoScrollAllowed(
  module: { viewportFit?: boolean } | undefined,
) {
  return module?.viewportFit !== true;
}
export function viewportFitHeight(input: {
  viewportHeight: number;
  documentTop: number;
  occlusionTop?: number;
  bottomPadding: number;
  previousHeight?: number;
  reason: "layout" | "scroll";
}): number | undefined {
  const { viewportHeight, documentTop, bottomPadding, occlusionTop } = input;
  if (![viewportHeight, documentTop, bottomPadding].every(Number.isFinite))
    return;
  const bottom =
    occlusionTop !== undefined && Number.isFinite(occlusionTop)
      ? Math.min(viewportHeight, occlusionTop)
      : viewportHeight;
  const height = Math.floor(bottom - documentTop - bottomPadding);
  if (height <= 0) return;
  return input.reason === "scroll" && input.previousHeight !== undefined
    ? Math.min(height, input.previousHeight)
    : height;
}
