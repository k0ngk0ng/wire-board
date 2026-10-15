import { useEffect, useMemo, useRef, useState } from "react";
import type { Room } from "../types";
import type { GameClock, PresentationFrame, RoomPresentation } from "./module";
import { gameModule } from "./registry";
import { schedulePresentationWake } from "./presentation-wake";

export function useRoomPresentation(
  room: Room | undefined,
  serverNow: number,
  receivedAt: number,
  epoch: number,
) {
  const [, tick] = useState(0);
  const clock = useMemo<GameClock>(
    () => ({ serverNow, receivedAt, epoch }),
    [serverNow, receivedAt, epoch],
  );
  const owner = useRef<
    | {
        identity: string;
        epoch: number;
        room?: Room;
        clock?: GameClock;
        controller: RoomPresentation;
      }
    | undefined
  >(undefined);
  const factory = room ? gameModule(room.kind)?.createPresentation : undefined;
  let frame: PresentationFrame | undefined;
  if (room && factory) {
    const identity = `${room.id}:${room.you}:${!!room.spectating}`;
    const changed = !owner.current || owner.current.identity !== identity;
    if (changed) owner.current = { identity, epoch, controller: factory() };
    const o = owner.current!;
    const reset = changed || o.epoch !== epoch;
    if (reset || o.room !== room || o.clock !== clock) {
      frame = o.controller.update({ room, clock }, performance.now(), reset);
      o.room = room;
      o.clock = clock;
      o.epoch = epoch;
    } else frame = o.controller.sample(performance.now());
  } else owner.current = undefined;
  const wakeAt = frame?.wakeAt;
  useEffect(() => {
    if (wakeAt === undefined) return;
    return schedulePresentationWake(wakeAt, () => tick((v) => v + 1));
  }, [wakeAt]);
  return {
    room: (frame?.room ?? room) as Room | undefined,
    clock: frame ? { ...frame.clock, epoch } : clock,
    blocked: frame?.blocked ?? false,
  };
}
