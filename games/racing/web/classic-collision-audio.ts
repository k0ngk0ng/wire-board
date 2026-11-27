import type { SpatialListener, SpatialOptions } from "../../../web/src/audio-spatial";
import type { ClassicAsset, ClassicManifest } from "./classic-assets";
import type { RaceEvent } from "./types";

export const classicCollisionMetadataPath = "audio/sfx/collision.json";
export const classicCollisionPatches = [24, 25, 29, 30, 31, 32, 33, 34, 35, 36, 37] as const;
export const nfs4CollisionDistanceScale = 5625;
export const nfs4CollisionCutoffMeters = Math.sqrt(nfs4CollisionDistanceScale / 2);

export type ClassicCollisionPatchMeta = {
  patch: number;
  path: string;
  sampleRate: number;
  channels: number;
  bits: number;
  frames: number;
  duration: number;
  bytes: number;
  sha256: string;
};

export type ClassicCollisionMetadata = {
  version: number;
  source: string;
  sourceSha256: string;
  cutoffDistanceSqScaled: number;
  patches: Record<string, ClassicCollisionPatchMeta>;
};

export type ClassicCollisionListener = {
  seat: number;
  x: number;
  y: number;
  angle: number;
  height?: number;
};

export type ResolvedCollisionSound = {
  patch: number;
  volume: number;
  distanceMeters: number;
  spatial: Omit<SpatialOptions, "playbackID">;
};

export function classicCollisionAssets(manifest: ClassicManifest): ClassicAsset[] {
  if (manifest.version !== 3) throw Error("撞击音效清单版本不匹配");
  const required = [
    classicCollisionMetadataPath,
    ...classicCollisionPatches.map((patch) => `audio/sfx/collision-${patch}.flac`),
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
      throw Error(`撞击音效资源清单缺失：${path}`);
    }
    return file;
  });
}

export async function loadClassicCollisionSounds(
  pack: { manifest: ClassicManifest; files: ReadonlyMap<string, ArrayBuffer> },
  signal: AbortSignal,
): Promise<{ metadata: ClassicCollisionMetadata; buffers: Map<number, ArrayBuffer> }> {
  const specs = classicCollisionAssets(pack.manifest);
  for (const file of specs) {
    signal.throwIfAborted();
    const data = pack.files.get(file.path);
    if (!data || data.byteLength !== file.bytes) throw Error(`撞击音效资源不完整：${file.path}`);
    const digest = await crypto.subtle.digest("SHA-256", data);
    signal.throwIfAborted();
    const hash = [...new Uint8Array(digest)]
      .map((n) => n.toString(16).padStart(2, "0"))
      .join("");
    if (hash !== file.sha256) throw Error(`撞击音效资源校验失败：${file.path}`);
  }
  const metadata = JSON.parse(
    new TextDecoder().decode(pack.files.get(classicCollisionMetadataPath)!),
  ) as ClassicCollisionMetadata;
  if (metadata.version !== 1 || !metadata.patches) throw Error("撞击音效元数据无效");
  const buffers = new Map<number, ArrayBuffer>();
  for (const patch of classicCollisionPatches) {
    const entry = metadata.patches[String(patch)];
    const bytes = pack.files.get(`audio/sfx/collision-${patch}.flac`);
    if (!entry || !bytes || entry.bytes !== bytes.byteLength) {
      throw Error(`撞击音效样本缺失：${patch}`);
    }
    buffers.set(patch, bytes);
  }
  return { metadata, buffers };
}

export function nfs4CollisionAttenuation(distanceMeters: number): number {
  if (!Number.isFinite(distanceMeters) || distanceMeters < 0) return 0;
  const scaledDistSq = 2 * distanceMeters * distanceMeters;
  if (scaledDistSq >= nfs4CollisionDistanceScale) return 0;
  return (nfs4CollisionDistanceScale - scaledDistSq) / nfs4CollisionDistanceScale;
}

export function selectNfs4CollisionPatch(
  kind: "car" | "wall" | "prop",
  intensity: number,
  lastPatch: number,
  seed = 0,
): number {
  const scaled = Math.max(0, Math.min(127, Math.round((Number.isFinite(intensity) ? intensity : 0) * 127)));
  if (kind === "prop") {
    const pool = scaled > 50 ? [31, 35] : [24, 25];
    const pick = pool[Math.abs(seed) % pool.length];
    return pick === lastPatch ? pool[(Math.abs(seed) + 1) % pool.length] : pick;
  }
  if (kind === "car") {
    if (scaled > 80) return 33;
    const pool = scaled > 45 ? [36, 32, 37] : [34, 30, 29];
    const pick = pool[Math.abs(seed) % pool.length];
    return pick === lastPatch ? pool[(Math.abs(seed) + 1) % pool.length] : pick;
  }
  let chosen = 30;
  if (scaled > 65) chosen = 33;
  else if (scaled > 45) chosen = 32;
  else if (scaled > 30) chosen = 37;
  else if (scaled > 15) chosen = 34;
  else chosen = 30;
  if (chosen === lastPatch && chosen !== 33) {
    if (chosen === 30) return 29;
    if (chosen === 34) return 37;
    if (chosen === 37) return 32;
    if (chosen === 32) return 34;
  }
  return chosen;
}

export function collisionSpatialListener(listener: ClassicCollisionListener): SpatialListener {
  return {
    x: listener.x,
    y: listener.y,
    z: listener.height ?? 0,
    yaw: listener.angle + Math.PI,
  };
}

export function resolveCollisionSpatialSound(
  event: Pick<RaceEvent, "id" | "type" | "kind" | "seat" | "x" | "y" | "intensity" | "at">,
  listener: ClassicCollisionListener | undefined,
  lastPatch: number,
): ResolvedCollisionSound | null {
  if (event.type !== "collision" && event.type !== "prop") return null;
  if (!listener) return null;
  const own = event.seat === listener.seat;
  const distanceMeters = own
    ? 0
    : Math.hypot(event.x - listener.x, event.y - listener.y) / 10;
  const attenuation = own ? 1 : nfs4CollisionAttenuation(distanceMeters);
  if (attenuation <= 0) return null;
  const kind: "car" | "wall" | "prop" =
    event.type === "prop" || event.kind === "prop"
      ? "prop"
      : event.kind === "car"
        ? "car"
        : "wall";
  const intensity = Math.max(0.15, Math.min(1, Number.isFinite(event.intensity) ? event.intensity : 0.4));
  const patch = selectNfs4CollisionPatch(kind, intensity, lastPatch, event.id);
  const baseGain = kind === "prop" ? 0.55 + intensity * 0.35 : 0.48 + intensity * 0.52;
  const volume = Math.max(0, Math.min(1, baseGain * attenuation));
  if (volume <= 0.01) return null;
  const spatialListener = collisionSpatialListener(listener);
  return {
    patch,
    volume,
    distanceMeters,
    spatial: {
      source: own
        ? { x: spatialListener.x, y: spatialListener.y, z: spatialListener.z }
        : { x: event.x, y: event.y, z: spatialListener.z },
      listener: spatialListener,
      volume,
      maxDistance: 100000,
    },
  };
}
