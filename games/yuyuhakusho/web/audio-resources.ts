import type { GameRoom } from "../../../web/src/game-api/module";
import playback from "../assets/audio-playback.json" with { type: "json" };
import release from "../assets/audio-release.json" with { type: "json" };

const arenaMusic = [0x82, 0x86, 0x83, 0x95, 0x85, 0x8a, 0x88, 0x8d];
export function yuyuAudioResources(room: Pick<GameRoom, "game" | "options">, localBase?: string) {
  const state = room.game?.data as { options?: { arena?: number } } | undefined;
  const options = room.options as { arena?: number } | undefined;
  const arena = state?.options?.arena ?? options?.arena ?? 0;
  const id = arenaMusic[arena / 2];
  if (id === undefined) throw Error("对局场地尚未就绪");
  const music = playback.music as Record<string, { file: string; bytes: number }>;
  const clips = [...Object.values(playback.effects), music[id]];
  return [...new Map(clips.map(clip => [clip.file, {
    url: localBase === undefined ? release.assetsBaseURL + "/" + clip.file : localBase.replace(/\/$/, "") + "/yuyuhakusho/audio/" + clip.file,
    bytes: clip.bytes,
  }])).values()];
}
