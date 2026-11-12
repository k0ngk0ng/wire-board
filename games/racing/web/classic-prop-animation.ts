import { classicPropBroken, type ClassicPropCatalog, type ClassicPropWorld } from "./classic-road.ts";
import type { RaceEvent } from "./types";
import type { AnimationMixer, AnimationClip, AnimationAction, AnimationActionLoopStyles } from "three";

export type ClassicVisualPropCatalog = Omit<ClassicPropCatalog, "props"> & {
  track: string;
  animations: { low: { duration: number }; high: { duration: number } };
  props: Array<ClassicPropCatalog["props"][number] & {
    id: string;
    node: string;
    motionNode: string;
    clips: { low: string; high: string };
  }>;
};
export type ClassicPropPose = {
  index: number;
  phase: "standing" | "hidden" | "animating" | "complete";
  time: number;
  duration: number;
  high: boolean;
  eventId?: number;
  at?: number;
  clip?: string;
};

/** Bind the SHA-verified catalog to this exact route and converted GLB. */
export function validateClassicPropCatalog(value: unknown, track: string, geometry: string, glbHash: unknown): ClassicVisualPropCatalog {
  const catalog = value as ClassicVisualPropCatalog | null;
  if (!catalog || catalog.version !== 1 || catalog.unit !== "metres" ||
      catalog.track !== track || catalog.geometry !== geometry ||
      !/^[a-f0-9]{64}$/.test(catalog.catalogHash) || catalog.catalogHash !== glbHash ||
      catalog.animations?.low?.duration !== .46875 || catalog.animations?.high?.duration !== 1.5 ||
      !Array.isArray(catalog.props) || !catalog.props.length || catalog.props.length > 108)
    throw Error("原物件目录与画面版本不一致");
  const identities = new Set<string>();
  for (const [index, prop] of catalog.props.entries()) {
    if (!prop || prop.index !== index || typeof prop.id !== "string" || identities.has(prop.id) ||
        prop.node !== `prop_${index}` || prop.motionNode !== `prop_${index}_motion` ||
        prop.clips?.low !== `prop_${index}_destroy_low_speed` || prop.clips?.high !== `prop_${index}_destroy_high_speed` ||
        !Array.isArray(prop.ruleEdges) || !prop.ruleEdges.length || prop.ruleEdges.some(edge => !Number.isInteger(edge) || edge < 0))
      throw Error("原物件目录身份无效");
    identities.add(prop.id);
  }
  return catalog;
}

/** Native ambient transform loops only; texture loops have their own renderer. */
export function classicAmbientClipNames(catalog: ClassicVisualPropCatalog, animations: ReadonlyArray<{ name?: string; extras?: { trigger?: string } }>): ReadonlySet<string> {
  const collision = new Set(catalog.props.flatMap(prop => [prop.clips.low, prop.clips.high]));
  return new Set(animations.filter(clip => clip.name && /^object_\d+_loop$/.test(clip.name) &&
    clip.extras?.trigger !== "collision" && !collision.has(clip.name)).map(clip => clip.name!));
}

/** Confirmed bits own visibility. Retained hit state never depends on the event ring. */
export class ClassicPropPlayback {
  private readonly poses = new Map<number, ClassicPropPose>();
  private readonly catalog: ClassicVisualPropCatalog;
  constructor(catalog: ClassicVisualPropCatalog) { this.catalog = catalog; }

  update(world: ClassicPropWorld | undefined, events: readonly RaceEvent[], serverNow: number, reduced: boolean): ClassicPropPose[] {
    if (world && (world.catalog !== this.catalog.catalogHash || world.geometry !== this.catalog.geometry))
      throw Error("原物件状态与画面版本不一致");
    const recent = new Map<number, RaceEvent>();
    for (const event of events) {
      const index = event.prop;
      if (event.type !== "prop" || !Number.isInteger(index) || index! < 0 || index! >= this.catalog.props.length ||
          !Number.isFinite(event.at) || !Number.isSafeInteger(event.id) || event.id <= 0) continue;
      const old = recent.get(index!);
      if (!old || event.id < old.id) recent.set(index!, event);
    }
    return this.catalog.props.map(prop => {
      let pose = this.poses.get(prop.index);
      if (!pose && classicPropBroken(world, prop.index)) {
        const event = recent.get(prop.index);
        const high = !!event?.high, duration = high ? 1.5 : .46875;
        const age = event ? (serverNow - event.at) / 1000 : Infinity;
        if (age < 0) return { index: prop.index, phase: "standing", time: 0, duration: 0, high: false };
        pose = event && Number.isFinite(age) && age < duration
          ? { index: prop.index, phase: "animating", time: age, duration, high, eventId: event.id, at: event.at, clip: high ? prop.clips.high : prop.clips.low }
          : { index: prop.index, phase: "hidden", time: 0, duration: 0, high: false };
      }
      if (pose?.phase === "animating") {
        const time = reduced ? pose.duration : Math.min(pose.duration, Math.max(pose.time, (serverNow - pose.at!) / 1000));
        pose = { ...pose, time, phase: time >= pose.duration ? "complete" : "animating" };
      }
      if (pose) this.poses.set(prop.index, pose);
      return pose ? { ...pose } : { index: prop.index, phase: "standing", time: 0, duration: 0, high: false };
    });
  }
}

/** A separate paused mixer evaluates native tracks only at authority time. */
export class ClassicPropClipPlayback {
  private readonly clips: ReadonlyMap<string, AnimationClip>;
  private readonly actions = new Map<number, { action: AnimationAction; time: number }>();
  private readonly mixer: AnimationMixer;
  private readonly once: AnimationActionLoopStyles;
  constructor(mixer: AnimationMixer, clips: readonly AnimationClip[], once: AnimationActionLoopStyles) {
    this.mixer = mixer; this.once = once;
    this.clips = new Map(clips.map(clip => [clip.name, clip]));
  }
  apply(poses: readonly ClassicPropPose[]): boolean {
    let changed = false;
    for (const pose of poses) {
      if (!pose.clip || (pose.phase !== "animating" && pose.phase !== "complete")) continue;
      let owned = this.actions.get(pose.index);
      if (!owned) {
        const clip = this.clips.get(pose.clip);
        if (!clip || Math.abs(clip.duration - pose.duration) > 1e-7) throw Error("原物件碰撞动作缺失或时长不一致");
        const action = this.mixer.clipAction(clip).reset().setLoop(this.once, 1);
        action.clampWhenFinished = true;
        action.play(); action.paused = true;
        owned = { action, time: -1 }; this.actions.set(pose.index, owned);
      }
      if (owned.time === pose.time) continue;
      owned.action.time = pose.time; owned.time = pose.time; changed = true;
    }
    if (changed) this.mixer.update(0);
    return changed;
  }
  stop() {
    this.mixer.stopAllAction(); this.actions.clear();
  }
}
