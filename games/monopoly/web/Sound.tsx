import { useEffect, useRef } from "react";
import type { GameRoom, GameAudioPort } from "../../../web/src/game-api/module";
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
  audio,
}: {
  room: GameRoom;
  data: View;
  enabled: boolean;
  assets: string;
  audio: GameAudioPort;
}) {
  const previous = useRef<{
    id: string;
    version: number;
    data: View;
    status: string;
  } | null>(null);
  useEffect(() => () => audio.stop(), [audio]);
  useEffect(() => {
    const prior = previous.current;
    previous.current = {
      id: room.id,
      version: room.version,
      data,
      status: room.status,
    };
    if (!enabled || !assets || room.status === "closed" || document.hidden) {
      audio.stop();
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
    if (selected.length)
      void audio.play(
        selected.map(
          (clip) =>
            `${assets.replace(/\/$/, "")}/monopoly/v1/audio/${clip}.mp3`,
        ),
      );
  }, [room.id, room.version, room.status, enabled, assets, audio]);
  return null;
}
