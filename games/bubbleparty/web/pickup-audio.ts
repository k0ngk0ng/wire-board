import type { GameEvent } from "./types";

/** Pickup sounds are private feedback; a newly revealed item is still silent. */
export function shouldPlayPickup(
  event: Pick<GameEvent, "type" | "seat">,
  you: number,
  spectating: boolean,
): boolean {
  return (
    event.type === "item" &&
    !spectating &&
    Number.isInteger(you) &&
    you >= 0 &&
    event.seat === you
  );
}
