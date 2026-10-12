import { useEffect, useRef, useState } from "react";
import type { View, Motion } from "./types";
import { motionPath } from "./motion";

export function useMotion(room: string, data: View, receivedAt: number) {
  const [positions, setPositions] = useState(() =>
    data.players.map((p) => p.position),
  );
  const [stage, setStage] = useState("idle");
  const [actor, setActor] = useState(-1);
  const [dice, setDice] = useState(data.dice);
  const latest = useRef(data);
  latest.current = data;
  const seen = useRef({ room, serial: data.motionSerial || 0, receivedAt });
  const queue = useRef<Motion[]>([]),
    running = useRef(false),
    generation = useRef(0);
  const reduced = useRef(
    matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  const reset = () => {
    generation.current++;
    queue.current = [];
    running.current = false;
    setPositions(latest.current.players.map((p) => p.position));
    setDice(latest.current.dice);
    setActor(-1);
    setStage("idle");
  };
  useEffect(() => {
    const query = matchMedia("(prefers-reduced-motion: reduce)");
    const preference = () => {
      reduced.current = query.matches;
      if (query.matches) reset();
    };
    const visibility = () => {
      if (document.hidden) reset();
    };
    query.addEventListener("change", preference);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      generation.current++;
      queue.current = [];
      query.removeEventListener("change", preference);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, []);
  useEffect(() => {
    const previous = seen.current,
      serial = data.motionSerial || 0;
    seen.current = { room, serial, receivedAt };
    const fresh = (data.motion || []).filter((e) => e.id > previous.serial);
    if (
      previous.room !== room ||
      serial < previous.serial ||
      receivedAt - previous.receivedAt > 20000 ||
      reduced.current ||
      document.hidden ||
      (serial > previous.serial && fresh[0]?.id !== previous.serial + 1)
    ) {
      reset();
      return;
    }
    if (!fresh.length) {
      if (!running.current) {
        setPositions(data.players.map((p) => p.position));
        setDice(data.dice);
      }
      return;
    }
    queue.current.push(...fresh);
    if (queue.current.length > 16) {
      reset();
      return;
    }
    if (running.current) return;
    running.current = true;
    const token = generation.current;
    const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
    void (async () => {
      while (queue.current.length && token === generation.current) {
        const event = queue.current.shift()!;
        setActor(event.player);
        setStage(event.kind);
        if (event.kind === "roll") {
          setDice(event.dice);
          await wait(550);
        } else {
          for (const at of motionPath(event)) {
            if (token !== generation.current) return;
            setPositions((old) =>
              old.map((p, i) => (i === event.player ? at : p)),
            );
            await wait(
              event.kind === "jump"
                ? 350
                : Math.min(100, 1200 / Math.max(1, Math.abs(event.steps))),
            );
          }
        }
      }
      if (token === generation.current) {
        running.current = false;
        setStage("idle");
        setActor(-1);
        setPositions(latest.current.players.map((p) => p.position));
        setDice(latest.current.dice);
      }
    })();
  }, [room, data.motionSerial, receivedAt]);
  return { positions, stage, actor, dice, active: stage !== "idle" };
}
