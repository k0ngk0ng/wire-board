import { lazy, Suspense, type ComponentProps } from "react";
const load = () => import("./room-ui");
export const loadLegacyRoom = load;
function part<K extends keyof typeof import("./room-ui")>(name: K) {
  const C = lazy(() =>
    load().then((m) => ({ default: m[name] })),
  ) as React.ComponentType<any>;
  return (props: ComponentProps<(typeof import("./room-ui"))[K]>) => (
    <Suspense fallback={<p role="status">正在加载游戏界面…</p>}>
      <C {...props} />
    </Suspense>
  );
}
export const LegacyCreate = part("LegacyCreate"),
  LegacyWaiting = part("LegacyWaiting"),
  LegacyPlayers = part("LegacyPlayers"),
  LegacyTurn = part("LegacyTurn"),
  LegacyRules = part("LegacyRules"),
  Results = part("Results");
export { LegacyCover } from "./cover";
