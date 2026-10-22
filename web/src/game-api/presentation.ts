import { RealtimeClock } from "./realtime-clock";
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
  const realtimeClock = useRef(new RealtimeClock());
  const realtime = !!room && !!gameModule(room.kind)?.realtime;
  const identity = `${room?.id}:${room?.matchId ?? room?.setupVersion}:${epoch}:${room?.clockOffsetMs ?? 0}`;
  const clock = useMemo<GameClock>(() => {
    const raw: GameClock = {
      serverNow:
        room?.loading?.frozenAt ?? serverNow - (room?.clockOffsetMs ?? 0),
      receivedAt,
      epoch,
      paused: !!room?.loading,
    };
    return realtime
      ? {
          ...realtimeClock.current.update(
            identity,
            raw.serverNow,
            receivedAt,
            !!raw.paused,
          ),
          epoch,
        }
      : raw;
  }, [
    realtime,
    identity,
    serverNow,
    receivedAt,
    epoch,
    room?.clockOffsetMs,
    room?.loading?.frozenAt,
  ]);
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
  const factory =
    room && !room.loading
      ? gameModule(room.kind)?.createPresentation
      : undefined;
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
