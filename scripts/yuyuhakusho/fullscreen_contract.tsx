import { createRef } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { GameFullscreen as Before } from "fullscreen-before";
import { GameFullscreen as Current } from "../../web/src/ui/room-fullscreen";

// Public UI contract only: no game rules or match state.
const root = createRoot(document.getElementById("root")!);
let handle = createRef<{ enter: () => void }>();
let generation = 0;
Object.defineProperty(HTMLElement.prototype, "requestFullscreen", {
  configurable: true,
  value: () => Promise.reject(new Error("Contract exercises page fallback")),
});
(window as any).fullscreenContract = {
  mount(before: boolean, actions = false, disabled = false) {
    handle = createRef();
    const Surface: any = before ? Before : Current;
    flushSync(() => root.render(
      <Surface key={++generation} ref={handle} scope="public-contract" enabled
        onReadyChange={() => {}}
        status={<span data-contract-clock>10 秒</span>}
        toolbarActions={actions ? <><span role="status">托管中</span><button data-viewport-resume disabled={disabled}>取消托管</button></> : undefined}>
        <canvas data-contract-canvas width={240} height={180} style={{ width: 240, height: 180 }} />
      </Surface>,
    ));
  },
  enter() { handle.current?.enter(); },
};
