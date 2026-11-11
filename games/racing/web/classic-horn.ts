import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { ClassicAsset, ClassicManifest } from "./classic-assets";
import type { Car } from "./types";

export const CLASSIC_HORN_PATH = "audio/horn/horn.flac";
const HORN_METADATA = "audio/horn/horn.json";
export type ClassicHornMetadata = {
  version: number;
  path: string;
  origin: string;
  sampleRate: number;
  channels: number;
  bits: number;
  frames: number;
  loopStartFrame: number;
  loopEndFrameExclusive: number;
  sha256: string;
};
type HornPack = { manifest: ClassicManifest; files: ReadonlyMap<string, ArrayBuffer> };

/** These two small files are mandatory and use the scene's verified loader. */
export function classicHornAssets(manifest: ClassicManifest): ClassicAsset[] {
  if (manifest.version !== 3) throw Error("喇叭资源版本不匹配");
  return [CLASSIC_HORN_PATH, HORN_METADATA].map((path) => {
    const file = manifest.assets.find((asset) => asset.path === path);
    if (!file || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 1024 * 1024 ||
      !/^[a-f0-9]{64}$/.test(file.sha256) || file.mime !== (path.endsWith(".flac") ? "audio/flac" : "application/json"))
      throw Error("自制喇叭资源清单不完整");
    return file;
  });
}

/** Recheck the prepared map before creating a playable object URL. */
export async function loadClassicHorn(pack: HornPack, signal: AbortSignal) {
  const specs = classicHornAssets(pack.manifest);
  for (const file of specs) {
    signal.throwIfAborted();
    const data = pack.files.get(file.path);
    if (!data) throw Error("喇叭资源缺失");
    if (data.byteLength !== file.bytes) throw Error("喇叭资源大小不匹配");
    const digest = await crypto.subtle.digest("SHA-256", data);
    signal.throwIfAborted();
    const hash = [...new Uint8Array(digest)].map(n => n.toString(16).padStart(2, "0")).join("");
    if (hash !== file.sha256) throw Error("喇叭资源校验失败");
  }
  const metadata = JSON.parse(new TextDecoder().decode(pack.files.get(HORN_METADATA)!)) as ClassicHornMetadata;
  if (metadata.version !== 1 || metadata.origin !== "original-synthesis" || metadata.path !== CLASSIC_HORN_PATH ||
    metadata.sha256 !== specs[0].sha256) throw Error("喇叭来源记录不匹配");
  if (metadata.sampleRate !== 22050 || metadata.channels !== 1 || metadata.bits !== 16 ||
    !Number.isSafeInteger(metadata.frames) || metadata.frames <= 0 ||
    metadata.loopStartFrame !== 0 || metadata.loopEndFrameExclusive !== metadata.frames)
    throw Error("喇叭循环格式无效");
  return { bytes: pack.files.get(CLASSIC_HORN_PATH)!, metadata };
}

type HornCar = Pick<Car, "seat" | "x" | "y" | "horn" | "protected" | "finishAt">;
type HornAudio = Pick<GameAudioPort, "loop" | "stopLoop">;

/** Owns platform loop keys only; it never creates a player or audio context. */
export class ClassicHornPlayback {
  private readonly active = new Set<string>();
  private source?: string;
  private readonly audio: HornAudio;
  constructor(audio: HornAudio) { this.audio = audio; }

  readonly stop = () => {
    for (const key of this.active) this.audio.stopLoop(key);
    this.active.clear();
  };

  update(source: string | undefined, cars: readonly HornCar[], localSeat: number | null | undefined,
    localHorn: boolean, running: boolean): void {
    if (source !== this.source) { this.stop(); this.source = source; }
    const desired = new Set<string>();
    if (source && running) {
      const local = cars.find(car => car.seat === localSeat);
      for (const car of cars) {
        // Board supplies an authorized, lifecycle-awake local intention. It
        // overrides the previous snapshot's protection and horn flags together.
        const own = car.seat === localSeat;
        const held = own ? localHorn : !!car.horn && !car.protected;
        if (!held || car.finishAt > 0) continue;
        const key = `racing-horn-${car.seat}`;
        const distance = local ? Math.hypot(car.x - local.x, car.y - local.y) / 10 : 0;
        const volume = own ? .32 : .18 / (1 + (Number.isFinite(distance) ? distance : 0) / 40);
        desired.add(key);this.active.add(key);
        void this.audio.loop(key, [source], { volume, rate: 1 }).catch(() => { /* Audio failure cannot interrupt driving. */ });
      }
    }
    for (const key of this.active) if (!desired.has(key)) {
      this.audio.stopLoop(key);this.active.delete(key);
    }
  }
}
