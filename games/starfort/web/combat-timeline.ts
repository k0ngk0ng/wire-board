// Keep authoritative snapshots intact; only their presentation is buffered.
// Whole movement segments can share the combat clock without exposing early
// packets before their logical interval begins.
export const FLIGHT_MS = 180;
export class CombatTimeline<T> {
  private current: T;
  private pending: { value: T; at: number }[] = [];
  constructor(initial: T) {
    this.current = initial;
  }
  push(value: T, receivedAt: number) {
    this.pushAt(value, receivedAt + FLIGHT_MS);
  }
  pushAt(value: T, at: number) {
    this.pending.push({ value, at });
    this.pending.sort((a, b) => a.at - b.at);
  }
  reset(value: T) {
    this.current = value;
    this.pending = [];
  }
  read(at: number): T {
    while (this.pending.length && this.pending[0].at <= at)
      this.current = this.pending.shift()!.value;
    return this.current;
  }
}

import type { Event, View } from "./types.ts";

// The server publishes one second of simulation per snapshot. Mapping logical
// time to a stable local clock preserves intervals across packet boundaries.
export const PLAYBACK_BUFFER_MS = 1000;
export type TimedEvent = Event & { born: number };
export type CombatStatus = { baseHP: number };
type CombatSnapshot = Pick<
  View,
  "elapsedMS" | "tick" | "phase" | "enemies" | "baseHP" | "craters" | "iceWalls"
>;
type Pending = {
  at: number;
  order: number;
  event?: Event;
  snapshot?: CombatSnapshot;
};
const immediateEvents = new Set([
  "build",
  "upgrade",
  "sell",
  "gift",
  "objective",
  "boon",
  "expand",
  "pause",
  "resume",
]);
export function eventTime(event: Event): number {
  return event.atMS ?? event.tick * 200;
}
function logicalTime(snapshot: CombatSnapshot): number {
  return snapshot.elapsedMS ?? snapshot.tick * 200;
}

export function activeEffects<T extends TimedEvent>(
  effects: T[],
  at: number,
  reduced: boolean,
): T[] {
  if (reduced) return [];
  return effects.filter(
    (event) => at - event.born < (event.type === "shot" ? FLIGHT_MS : 900),
  );
}

// Consume cues once on the rendering clock. A blocked tab must not play the
// sounds of projectiles whose entire visible lifetime was missed.
export function dueSoundEvents<T extends TimedEvent>(
  events: T[],
  at: number,
): [T[], T[]] {
  return [
    events.filter((event) => event.born <= at && at - event.born <= 120),
    events.filter((event) => event.born > at),
  ];
}

export class CombatPlayback {
  private current: CombatSnapshot;
  private pending: Pending[] = [];
  private offset: number;
  private anchorLogical: number;
  private started = false;
  private publishedBaseHP: number;
  private observers = new Set<(state: CombatStatus) => void>();
  constructor(initial: CombatSnapshot, receivedAt: number) {
    this.current = initial;
    this.anchorLogical = logicalTime(initial);
    this.publishedBaseHP = initial.baseHP;
    this.offset = receivedAt + PLAYBACK_BUFFER_MS - logicalTime(initial);
  }
  reset(snapshot: CombatSnapshot, receivedAt: number) {
    this.current = snapshot;
    this.anchorLogical = logicalTime(snapshot);
    this.started = false;
    this.pending = [];
    this.offset = receivedAt + PLAYBACK_BUFFER_MS - logicalTime(snapshot);
    this.publish();
  }
  subscribe(observer: (state: CombatStatus) => void): () => void {
    this.observers.add(observer);
    observer({ baseHP: this.current.baseHP });
    return () => {
      this.observers.delete(observer);
    };
  }
  private publish() {
    if (this.publishedBaseHP === this.current.baseHP) return;
    this.publishedBaseHP = this.current.baseHP;
    for (const observer of this.observers)
      observer({ baseHP: this.current.baseHP });
  }
  time(logicalMS: number): number {
    return this.offset + logicalMS;
  }
  push(
    snapshot: CombatSnapshot,
    events: Event[],
    receivedAt: number,
  ): TimedEvent[] {
    if (!this.started && logicalTime(snapshot) > this.anchorLogical) {
      // Loading readiness can delay the first timer batch. Anchor once to the
      // first advancing packet; subsequent packets must preserve this clock.
      const nextOffset = Math.max(
        this.offset,
        receivedAt + PLAYBACK_BUFFER_MS - logicalTime(snapshot),
      );
      const shift = nextOffset - this.offset;
      for (const item of this.pending)
        if (!item.event || !immediateEvents.has(item.event.type))
          item.at += shift;
      this.offset = nextOffset;
      this.started = true;
    }
    const walls = new Map(
      [
        ...(this.current.iceWalls ?? []),
        ...(snapshot.iceWalls ?? []),
        ...events.flatMap((event) => (event.iceWall ? [event.iceWall] : [])),
      ].map((wall) => [wall.id, wall]),
    );
    const effects = events.map((event) => {
      let born = immediateEvents.has(event.type)
        ? receivedAt
        : this.time(eventTime(event)) +
          (["shot", "spawn", "summon", "frostbound"].includes(event.type)
            ? 0
            : FLIGHT_MS);
      if (event.type === "frostbound" && event.iceWallId) {
        const wall = walls.get(event.iceWallId);
        if (wall) born = Math.max(born, this.time(wall.createdMS) + FLIGHT_MS);
      }
      if (
        event.target ||
        event.damage?.length ||
        event.damageGroups?.length ||
        event.vitals ||
        event.crater ||
        event.slowTargets?.length ||
        (event.type === "frostbound" && event.targetId) ||
        event.iceWall
      )
        this.pending.push({ at: born, order: event.id, event });
      return { ...event, born };
    });
    this.pending.push({
      at: this.time(logicalTime(snapshot)) + FLIGHT_MS,
      // A final snapshot follows all impacts at its logical boundary.
      order: Number.MAX_SAFE_INTEGER,
      snapshot,
    });
    this.pending.sort((a, b) => a.at - b.at || a.order - b.order);
    return effects;
  }
  read(at: number, publish = true): CombatSnapshot {
    while (this.pending.length && this.pending[0].at <= at) {
      const item = this.pending.shift()!;
      if (item.snapshot) {
        this.current = item.snapshot;
        continue;
      }
      const event = item.event!;
      const target = event.target;
      const enemies = new Map(
        this.current.enemies.map((enemy) => [enemy.id, enemy]),
      );
      if (event.type === "frostbound") {
        // Contact changes status only; projectile HP remains on its own clock.
        const current = enemies.get(event.targetId ?? target?.id ?? 0);
        if (current) enemies.set(current.id, { ...current, frostbound: true });
      } else if (target) {
        if (
          event.type === "base_hit" ||
          event.type === "death" ||
          target.hp <= 0
        )
          enemies.delete(target.id);
        else if (event.type !== "shot" || !enemies.has(target.id))
          enemies.set(
            target.id,
            enemies.get(target.id)?.frostbound
              ? { ...target, frostbound: true }
              : target,
          );
      }
      if (event.vitals && event.targetId) {
        const victim = enemies.get(event.targetId);
        if (victim)
          enemies.set(event.targetId, {
            ...victim,
            ...event.vitals,
            health: event.vitals.hp + event.vitals.shield,
          });
      }
      const applyDamage = (id: number, amount: number) => {
        const victim = enemies.get(id);
        if (!victim) return;
        const absorbed = Math.min(victim.shield, amount),
          shield = victim.shield - absorbed;
        const hp = Math.max(0, victim.hp - (amount - absorbed));
        enemies.set(id, { ...victim, hp, shield, health: hp + shield });
      };
      for (const group of event.damageGroups ?? [])
        for (const id of group.targets)
          applyDamage((group.base ?? 0) + id, group.amount);
      for (const delta of event.damage ?? []) {
        applyDamage(delta.id, delta.amount);
      }
      for (const id of event.slowTargets ?? []) {
        const enemy = enemies.get(id);
        if (enemy)
          enemies.set(id, {
            ...enemy,
            slowUntil: Math.max(enemy.slowUntil, event.slowUntil ?? 0),
          });
      }
      let craters = this.current.craters;
      if (event.crater) {
        const updates = new Map((craters ?? []).map((c) => [c.id, c]));
        updates.set(event.crater.id, event.crater);
        craters = [...updates.values()];
        if (event.type === "crater_filled") {
          for (const [id, enemy] of enemies)
            if (enemy.repairing === event.crater.id)
              enemies.set(id, { ...enemy, repairing: 0 });
        }
      }
      let iceWalls = this.current.iceWalls;
      if (event.iceWall) {
        const walls = new Map((iceWalls ?? []).map((w) => [w.id, w]));
        walls.set(event.iceWall.id, event.iceWall);
        iceWalls = [...walls.values()];
      }
      this.current = {
        ...this.current,
        iceWalls,
        enemies: [...enemies.values()],
        craters,
        baseHP:
          event.type === "base_hit"
            ? Math.max(0, this.current.baseHP - (event.amount ?? 0))
            : this.current.baseHP,
      };
    }
    if (publish) this.publish();
    return this.current;
  }
}
