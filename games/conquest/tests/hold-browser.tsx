// Pre-implementation hook bridge acceptance. The fixture changes props through
// real controls/timers; input is native browser input, never dispatched handlers.
import { StrictMode, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { useHoldAction } from "../web/hold-action";

type Row = { kind: string; identity: string; payload: string; button: string };
const evidence = {
  calls: [] as Row[],
  presses: [] as { trusted: boolean; type: string; at: number }[],
  errors: [] as string[],
};
Object.assign(window, { holdEvidence: evidence });
window.addEventListener("unhandledrejection", (e) =>
  evidence.errors.push(String(e.reason)),
);
function Action({
  name,
  enabled,
  identity,
  payload,
  mode,
  record,
}: {
  name: string;
  enabled: boolean;
  identity: string;
  payload: string;
  mode: string;
  record: (r: Row) => void;
}) {
  const action = useHoldAction({
    enabled,
    identity,
    onTap: () => record({ kind: "tap", identity, payload, button: name }),
    onHold: () => {
      record({ kind: "hold", identity, payload, button: name });
      if (mode === "reject")
        return Promise.reject(new Error("fixture refusal"));
    },
  });
  return (
    <button
      id={"action-" + name}
      {...action.handlers}
      disabled={!enabled}
      data-holding={action.holding}
      aria-keyshortcuts="Shift+Enter"
    >
      {name}
      <span>hold={String(action.holding)}</span>
    </button>
  );
}
function Fixture() {
  const [mode, setMode] = useState("normal"),
    [enabled, setEnabled] = useState(true),
    [identity, setIdentity] = useState("old"),
    [payload, setPayload] = useState("old"),
    [mounted, setMounted] = useState(true),
    [count, setCount] = useState(0);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);
  const record = (row: Row) => {
    evidence.calls.push(row);
    setCount(evidence.calls.length);
    if (mode === "fast" && row.kind === "hold") {
      setEnabled(false);
      timers.current.push(setTimeout(() => setEnabled(true), 70));
    }
  };
  const prepare = (value: string) => {
    for (const t of timers.current) clearTimeout(t);
    timers.current = [];
    setMode(value);
    setEnabled(true);
    setIdentity("old");
    setPayload("old");
    setMounted(true);
    evidence.calls = [];
    evidence.presses = [];
    evidence.errors = [];
    setCount(0);
  };
  return (
    <main>
      <label>
        Mode
        <select
          id="mode"
          value={mode}
          onChange={(e) => prepare(e.target.value)}
        >
          {[
            "normal",
            "identity",
            "disable",
            "fast",
            "latest",
            "unmount",
            "reject",
          ].map((x) => (
            <option key={x}>{x}</option>
          ))}
        </select>
      </label>
      <button id="reset" onClick={() => prepare(mode)}>
        Reset
      </button>
      <div
        id="actions"
        onPointerDownCapture={(e) => {
          evidence.presses.push({
            trusted: e.isTrusted,
            type: e.pointerType,
            at: performance.now(),
          });
          if (mode === "identity")
            timers.current.push(setTimeout(() => setIdentity("new"), 160));
          if (mode === "latest")
            timers.current.push(setTimeout(() => setPayload("new"), 160));
          if (mode === "disable") {
            timers.current.push(setTimeout(() => setEnabled(false), 160));
            timers.current.push(setTimeout(() => setEnabled(true), 300));
          }
          if (mode === "unmount")
            timers.current.push(setTimeout(() => setMounted(false), 160));
        }}
      >
        {mounted && (
          <>
            <Action
              name="A"
              {...{ enabled, identity, payload, mode, record }}
            />
            <Action
              name="B"
              {...{ enabled, identity, payload, mode, record }}
            />
          </>
        )}
      </div>
      <output id="count">{count}</output>
      <output id="identity">
        {identity}:{payload}
      </output>
    </main>
  );
}
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Fixture />
  </StrictMode>,
);
