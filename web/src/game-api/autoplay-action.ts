type InputReleaseCapability = {
  realtime?: boolean;
  isInputRelease?: (action: Record<string, unknown>) => boolean;
};

// A release clears held human controls; it must never reclaim an automated
// seat. The server independently validates the action and its ownership.
export function blocksAutoplayAction(
  automated: boolean | undefined,
  module: InputReleaseCapability | undefined,
  action: Record<string, unknown>,
): boolean {
  return !!automated && !(module?.realtime && module.isInputRelease?.(action));
}
