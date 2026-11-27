import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { ClassicAsset, ClassicManifest } from "./classic-assets";
import { classicSkidStrength } from "./classic-skids.ts";

export const classicSkidMetadataPath = "audio/sfx/skid.json";
export const classicSkidPatches = [41, 42, 45, 46, 47] as const;

export type ClassicSkidPatchMeta = {
  patch: number;
  surface: string;
  path: string;
  sampleRate: number;
  channels: number;
  bits: number;
  loopStart: number;
  loopEnd: number;
  frames: number;
  duration: number;
  volume: number;
  bytes: number;
  sha256: string;
};

export type ClassicSkidMetadata = {
  version: number;
  source: string;
  sourceSha256: string;
  patches: Record<string, ClassicSkidPatchMeta>;
};

export type BrakeAudioCar = {
  airborne?: boolean;
  vx: number;
  vy: number;
  angle: number;
  speed: number;
  brake: boolean;
  brakeAmount?: number;
  drift: boolean;
  offroad: boolean;
  finishAt: number;
  protected?: boolean;
  resetUntil: number;
  motion?: { longAccel?: number; brake?: number; handbrake?: number };
};

export type ClassicBrakeVoice = {
  key: string;
  patch: number;
  volume: number;
  rate: number;
  cause: "brake" | "handbrake" | "slip";
};

const clamp = (value: number, low = 0, high = 1) =>
  Math.max(low, Math.min(high, value));
const finite = (value: number | undefined, fallback = 0) =>
  Number.isFinite(value) ? value! : fallback;

export function classicSkidAssets(manifest: ClassicManifest): ClassicAsset[] {
  if (manifest.version !== 3) throw Error("刹车胎响清单版本不匹配");
  const required = [
    classicSkidMetadataPath,
    ...classicSkidPatches.map((patch) => `audio/sfx/skid-${patch}.flac`),
  ];
  return required.map((path) => {
    const file = manifest.assets.find((asset) => asset.path === path);
    if (
      !file ||
      !Number.isSafeInteger(file.bytes) ||
      file.bytes <= 0 ||
      file.bytes > 2 * 1024 * 1024 ||
      !/^[a-f0-9]{64}$/.test(file.sha256) ||
      file.mime !== (path.endsWith(".flac") ? "audio/flac" : "application/json")
    ) {
      throw Error(`刹车胎响资源清单缺失：${path}`);
    }
    return file;
  });
}

export async function loadClassicSkidSounds(
  pack: { manifest: ClassicManifest; files: ReadonlyMap<string, ArrayBuffer> },
  signal: AbortSignal,
): Promise<{ metadata: ClassicSkidMetadata; buffers: Map<number, ArrayBuffer> }> {
  const specs = classicSkidAssets(pack.manifest);
  for (const file of specs) {
    signal.throwIfAborted();
    const data = pack.files.get(file.path);
    if (!data || data.byteLength !== file.bytes) {
      throw Error(`刹车胎响资源不完整：${file.path}`);
    }
    const digest = await crypto.subtle.digest("SHA-256", data);
    signal.throwIfAborted();
    const hash = [...new Uint8Array(digest)]
      .map((n) => n.toString(16).padStart(2, "0"))
      .join("");
    if (hash !== file.sha256) {
      throw Error(`刹车胎响资源校验失败：${file.path}`);
    }
  }
  const metadata = JSON.parse(
    new TextDecoder().decode(pack.files.get(classicSkidMetadataPath)!),
  ) as ClassicSkidMetadata;
  if (metadata.version !== 1 || !metadata.patches) {
    throw Error("刹车胎响元数据无效");
  }
  const buffers = new Map<number, ArrayBuffer>();
  for (const patch of classicSkidPatches) {
    const entry = metadata.patches[String(patch)];
    const bytes = pack.files.get(`audio/sfx/skid-${patch}.flac`);
    if (!entry || !bytes || entry.bytes !== bytes.byteLength) {
      throw Error(`刹车胎响样本缺失：${patch}`);
    }
    buffers.set(patch, bytes);
  }
  return { metadata, buffers };
}

export function classicSkidPatchForSurface(surface: number): number {
  const code = Number.isFinite(surface) ? Math.trunc(surface) : 1;
  if (code === 4 || code === 12) return 46;
  if (code === 7) return 41;
  if (code === 3 || code === 11) return 45;
  if (code === 2 || code === 5 || code === 9 || code === 10 || code === 13 || code === 15) return 47;
  return 42;
}

export function classicBrakeAudioVoice(
  car: BrakeAudioCar | undefined,
  serverNow: number,
  surface?: number,
): ClassicBrakeVoice | null {
  if (!car) return null;
  if (![car.vx, car.vy, car.angle].every(Number.isFinite)) return null;
  if (car.airborne || car.protected || car.finishAt || car.resetUntil > serverNow) {
    return null;
  }
  const speedMps = Math.hypot(finite(car.vx), finite(car.vy)) / 10;
  if (speedMps < 2.2) return null;
  const forwardMps =
    (car.vx * Math.cos(car.angle) + car.vy * Math.sin(car.angle)) / 10;
  const skid = classicSkidStrength(car, serverNow);
  const skidIntensity = Math.max(skid.front, skid.rear);
  const effectiveBrake = finite(
    car.motion?.brake,
    car.brake ? (car.brakeAmount ?? 1) : 0,
  );
  const forwardBrake =
    effectiveBrake > 0.15 && forwardMps >= 2.8
      ? clamp(effectiveBrake) * clamp((forwardMps - 2.2) / 14, 0.25, 1) * 0.72
      : 0;
  const intensity = clamp(Math.max(skidIntensity, forwardBrake));
  if (intensity <= 0.05) return null;
  const cause: "brake" | "handbrake" | "slip" =
    skid.cause === "handbrake"
      ? "handbrake"
      : skid.cause === "brake" || forwardBrake >= skidIntensity
        ? "brake"
        : "slip";
  const effectiveSurface =
    surface !== undefined ? surface : car.offroad ? 2 : 1;
  const patch = classicSkidPatchForSurface(effectiveSurface);
  const volume = clamp(0.06 + intensity * 0.22, 0, 0.28);
  const rate = clamp(
    0.76 + clamp(speedMps / 42, 0, 1) * 0.42 + intensity * 0.12,
    0.72,
    1.38,
  );
  return {
    key: `racing-skid-${patch}`,
    patch,
    volume,
    rate,
    cause,
  };
}

type BrakeAudioPort = Pick<GameAudioPort, "loop" | "stopLoop">;

export class ClassicBrakePlayback {
  private readonly audio: BrakeAudioPort;
  private activeKey = "";

  constructor(audio: BrakeAudioPort) {
    this.audio = audio;
  }

  readonly stop = () => {
    if (this.activeKey) {
      this.audio.stopLoop(this.activeKey);
      this.activeKey = "";
    }
  };

  update(
    sources: ReadonlyMap<number, string> | undefined,
    voice: ClassicBrakeVoice | null,
    running: boolean,
  ) {
    if (!running || !sources || !voice) {
      this.stop();
      return;
    }
    const url = sources.get(voice.patch);
    if (!url) {
      this.stop();
      return;
    }
    if (this.activeKey && this.activeKey !== voice.key) {
      this.audio.stopLoop(this.activeKey);
    }
    this.activeKey = voice.key;
    void this.audio
      .loop(voice.key, [url], {
        volume: voice.volume,
        rate: voice.rate,
        continuous: true,
        seamOverlapMS: 5,
      })
      .catch(() => {});
  }
}
