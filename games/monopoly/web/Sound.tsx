import { useEffect, useRef } from "react";
import type { GameRoom } from "../../../web/src/game-api/module";
import type { View } from "./types";

// Only public state differences select cues. No private decks or game rules
// cross this module boundary. Reconnect/version gaps never replay old actions.
function cues(before: View, after: View, room: GameRoom): string[] {
  if (room.game?.finished) return ["winning"];
  const clips: string[] = [];
  if (after.rollSerial !== before.rollSerial) clips.push("rolling");
  if (after.players.some((p, i) => p.bankrupt && !before.players[i]?.bankrupt))
    clips.push("dying");
  else if (after.players.some((p, i) => p.jailed && !before.players[i]?.jailed))
    clips.push("jail");
  else if (after.lastCard?.sequence !== before.lastCard?.sequence)
    clips.push("card");
  else if (
    after.properties.some(
      (p, i) =>
        p.buildings !== before.properties[i]?.buildings ||
        p.owner !== before.properties[i]?.owner,
    )
  )
    clips.push("buying");
  else if (
    after.offers.some(
      (o) => !before.offers.some((b) => b.id === o.id) && o.to === room.you,
    )
  )
    clips.push("notifications");
  else if (
    room.you >= 0 &&
    after.players[room.you].cash !== before.players[room.you].cash
  )
    clips.push(
      after.players[room.you].cash > before.players[room.you].cash
        ? "moneyplus"
        : "moneyminus",
    );
  else if (after.turn !== before.turn && after.turn === room.you)
    clips.push("notifications");
  return clips.slice(0, 2);
}

export function Sound({
  room,
  data,
  enabled,
  assets,
}: {
  room: GameRoom;
  data: View;
  enabled: boolean;
  assets: string;
}) {
  const element = useRef<HTMLAudioElement>(null);
  const previous = useRef<{
    id: string;
    version: number;
    data: View;
    status: string;
  } | null>(null);
  const pending = useRef<string[]>([]);
  const allowed = useRef(false);
  allowed.current = enabled && !!assets && room.status !== "closed";

  const stop = () => {
    pending.current = [];
    element.current?.pause();
  };
  const next = () => {
    const audio = element.current,
      clip = pending.current.shift();
    if (!audio || !clip || !allowed.current || document.hidden) return;
    audio.src = `${assets.replace(/\/$/, "")}/monopoly/v1/audio/${clip}.mp3`;
    audio.muted = false;
    audio.volume = 0.65;
    // CDN failure or autoplay refusal must never block a game action.
    void audio.play().catch(() => {
      pending.current = [];
    });
  };
  useEffect(() => {
    const audio = element.current!;
    let unlocked = false;
    const unlock = () => {
      if (unlocked || !allowed.current || !audio.paused) return;
      unlocked = true;
      // Unlock this same media element within a real user gesture on mobile.
      const silent =
        "data:audio/wav;base64,UklGRiYAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQIAAACAgA==";
      audio.src = silent;
      audio.muted = true;
      void audio
        .play()
        .then(() => {
          if (audio.src === silent) {
            audio.pause();
            audio.removeAttribute("src");
          }
        })
        .catch(() => {
          unlocked = false;
        });
    };
    const hide = () => {
      if (document.hidden) stop();
    };
    document.addEventListener("pointerdown", unlock);
    document.addEventListener("keydown", unlock);
    document.addEventListener("visibilitychange", hide);
    return () => {
      document.removeEventListener("pointerdown", unlock);
      document.removeEventListener("keydown", unlock);
      document.removeEventListener("visibilitychange", hide);
      pending.current = [];
      audio.pause();
    };
  }, []);
  useEffect(() => {
    const prior = previous.current;
    previous.current = {
      id: room.id,
      version: room.version,
      data,
      status: room.status,
    };
    if (!allowed.current || document.hidden) {
      stop();
      return;
    }
    if (
      !prior ||
      prior.id !== room.id ||
      prior.version + 1 !== room.version ||
      prior.status !== "playing"
    )
      return;
    const selected = cues(prior.data, data, room);
    if (!selected.length) return;
    pending.current = selected;
    element.current?.pause();
    next();
  }, [room.id, room.version, room.status, enabled, assets]);
  return (
    <audio
      ref={element}
      data-mono-sound
      preload="none"
      onEnded={next}
      onError={() => {
        pending.current = [];
      }}
    />
  );
}
