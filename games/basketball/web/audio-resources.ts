import type { GameRoom } from "../../../web/src/game-api/module";
import catalog from "../assets/audio-catalog.json" with { type: "json" };
import playback from "../assets/audio-playback.json" with { type: "json" };
import release from "../assets/release-manifest.json" with { type: "json" };

const courtMusic = [59, 66, 61, 64, 60, 62, 63, 65];
type Options = { court?: string; campaign?: unknown };
export function basketballAudioResources(room: Pick<GameRoom, "game" | "options">) {
  const state = room.game?.data as { options?: Options } | undefined;
  const options = state?.options ?? room.options as Options | undefined;
  const court = options?.court === "new-york" ? 1 : Number(options?.court?.replace("court-", "") ?? "1");
  if (!Number.isSafeInteger(court) || court < 0 || court >= courtMusic.length) throw Error("对局场地尚未就绪");
  const effects = new Map(catalog.effects.map(clip => [clip.id, clip.wav]));
  for (const [id, clip] of Object.entries(playback.effects)) effects.set(Number(id), clip.wav);
  const music = playback.music as Record<string, { file: string }>;
  const tracks = options?.campaign ? Object.keys(music).map(Number) : [courtMusic[court], 75, 76];
  const files = new Set([...effects.values(), ...tracks.map(id => {
    const clip = music[id];
    if (!clip?.file) throw Error("音乐发布记录不完整");
    return clip.file;
  })]);
  const metadata = release.files as Record<string, { bytes: number }>;
  return [...files].map(file => {
    const url = "/basketball/v1/audio/" + file;
    const record = metadata[url];
    if (!record) throw Error("音频发布记录不完整");
    return { url, bytes: record.bytes };
  });
}
