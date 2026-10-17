import type { Room } from "./types";
import {
  flightCenter,
  visibleFlightAnchor,
} from "../../../web/src/flight-anchor";
import "./splendor-flight-recipient.css";

// A named, real on-screen endpoint replaces only an offscreen avatar. Never
// clamp that avatar onto a different player's visible position.
export function SplendorFlightRecipients({
  room,
  players,
}: {
  room: Room;
  players: number[];
}) {
  return (
    <>
      {[...new Set(players)].map((player) => (
        <span
          key={player}
          className="splendor-flight-recipient"
          data-flight-recipient={player}
          data-recipient-name={room.seats[player]?.name || "玩家"}
          style={{ top: 12 + player * 38 }}
        >
          <i>{player + 1}</i>
          <span>{room.seats[player]?.name || "玩家"}</span>
        </span>
      ))}
    </>
  );
}

export function measureSplendorRecipient(
  layer: HTMLElement | null,
  player: number,
) {
  const avatar = document.querySelector(
    `[data-player-seat="${player}"] .avatar`,
  );
  const actual = avatar ? visibleFlightAnchor(avatar) : null;
  const badge = layer?.querySelector<HTMLElement>(
    `[data-flight-recipient="${player}"]`,
  );
  const view = window.visualViewport;
  const clip = {
    left: view?.offsetLeft ?? 0,
    top: view?.offsetTop ?? 0,
    right: (view?.offsetLeft ?? 0) + (view?.width ?? innerWidth),
    bottom: (view?.offsetTop ?? 0) + (view?.height ?? innerHeight),
  };
  return {
    point:
      actual ||
      (badge ? flightCenter(badge.getBoundingClientRect(), [clip]) : null),
    badge,
    offscreen: !actual,
    color: avatar ? getComputedStyle(avatar).backgroundColor : "#496147",
  };
}

export function showSplendorRecipient(
  recipient: ReturnType<typeof measureSplendorRecipient>,
) {
  if (!recipient.badge) return;
  recipient.badge.style.visibility = recipient.offscreen ? "visible" : "hidden";
  recipient.badge.style.setProperty("--recipient-color", recipient.color);
}
