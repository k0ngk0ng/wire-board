type ObjectValue = Record<string, unknown>;
const collectionNames = ['players', 'hostages', 'grenades', 'drops', 'brushes'] as const;
const headerKeys = new Set(['schema', 'epoch', 'frame', 'serverAt', 'eventId', 'events', ...collectionNames]);
const forbidden = new Set(['__proto__', 'prototype', 'constructor']);

function invalid(): never { throw Error('CS 1.6 实时增量缺少有效基线或格式无效，请重新同步'); }
function own(value: ObjectValue, key: string) { return Object.prototype.hasOwnProperty.call(value, key); }
function object(value: unknown): ObjectValue {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return invalid();
  const prototype = Object.getPrototypeOf(value);
  if (prototype !== Object.prototype && prototype !== null) return invalid();
  const keys = Object.keys(value);
  if (keys.length > 512 || keys.some(key => forbidden.has(key))) return invalid();
  return value as ObjectValue;
}
function integer(value: unknown): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) return invalid();
  return value;
}
function array(value: unknown, maximum = 4096): unknown[] {
  if (!Array.isArray(value) || value.length > maximum) return invalid();
  return value;
}
function fields(value: ObjectValue, allowed: readonly string[]) { if (Object.keys(value).some(key => !allowed.includes(key))) invalid(); }
function json(value: unknown, depth = 0): void {
  if (depth > 32) invalid();
  if (value === null || typeof value === 'boolean') return;
  if (typeof value === 'string') { if (value.length > 1048576) invalid(); return; }
  if (typeof value === 'number') { if (!Number.isSafeInteger(value)) invalid(); return; }
  if (Array.isArray(value)) { for (const item of array(value)) json(item, depth + 1); return; }
  for (const item of Object.values(object(value))) json(item, depth + 1);
}
function entityId(value: unknown, key: string) { const id = integer(object(value)[key]); if (key === 'seat' && id > 9) invalid(); return id; }
function collection(value: unknown, key: string) {
  const entries = array(value, key === 'seat' ? 10 : 4096);
  const seen = new Set<number>();
  for (const item of entries) { const id = entityId(item, key); if (seen.has(id)) invalid(); seen.add(id); }
  return entries;
}
function events(value: unknown, latest: number, base = -1) {
  const selected = array(value, 256);
  let previous = base;
  for (const event of selected) { const id = integer(object(event).id); if (id <= previous || id > latest) invalid(); previous = id; }
  return selected;
}
function snapshot(value: unknown): ObjectValue {
  const result = object(value);
  if (result.schema !== 1 || typeof result.epoch !== 'string' || !result.epoch || result.epoch.length > 128) return invalid();
  integer(result.frame);
  integer(result.serverAt);
  const latest = integer(result.eventId);
  for (const name of collectionNames) collection(result[name], name === 'players' ? 'seat' : 'id');
  events(result.events, latest);
  if (own(result, 'recover') && typeof result.recover !== 'boolean') invalid();
  json(result);
  return result;
}
function applyObject(previous: unknown, patch: unknown, global = false): ObjectValue {
  const source = object(previous);
  const delta = object(patch);
  fields(delta, ['values', 'remove', 'objects']);
  const result = { ...source };
  const touched = new Set<string>();
  const touch = (key: string) => { if (forbidden.has(key) || touched.has(key) || global && headerKeys.has(key)) invalid(); touched.add(key); };
  if (own(delta, 'values')) for (const [key, value] of Object.entries(object(delta.values))) {
    touch(key);
    json(value);
    result[key] = value;
  }
  if (own(delta, 'remove')) for (const key of array(delta.remove, 512)) {
    if (typeof key !== 'string' || !own(source, key)) invalid();
    touch(key);
    delete result[key];
  }
  if (own(delta, 'objects')) for (const [key, value] of Object.entries(object(delta.objects))) {
    touch(key);
    if (!own(source, key)) invalid();
    result[key] = applyObject(source[key], value);
  }
  return result;
}
function applyCollection(previous: unknown, patch: unknown, key: string): unknown[] {
  const source = collection(previous, key);
  const delta = object(patch);
  fields(delta, ['upsert', 'remove']);
  const byId = new Map(source.map(item => [entityId(item, key), item]));
  const touched = new Set<number>();
  const removed = new Set<number>();
  if (own(delta, 'remove')) for (const value of array(delta.remove)) {
    const id = integer(value);
    if (!byId.has(id) || touched.has(id)) invalid();
    touched.add(id);
    removed.add(id);
  }
  const added: number[] = [];
  if (own(delta, 'upsert')) for (const value of array(delta.upsert)) {
    const update = object(value);
    fields(update, ['id', 'full', 'patch']);
    const id = integer(update.id);
    if (touched.has(id) || own(update, 'full') === own(update, 'patch')) invalid();
    touched.add(id);
    const existed = byId.has(id);
    if (!existed && !own(update, 'full')) invalid();
    const next = own(update, 'full') ? object(update.full) : applyObject(byId.get(id), update.patch);
    if (entityId(next, key) !== id) invalid();
    if (key === 'seat' && existed && !own(update, 'full')) {
      const previous = object(byId.get(id));
      if (next.life !== previous.life || next.visible !== previous.visible) invalid();
    }
    json(next);
    byId.set(id, next);
    if (!existed) added.push(id);
  }
  const result = source.filter(item => !removed.has(entityId(item, key))).map(item => byId.get(entityId(item, key)));
  for (const id of added) result.push(byId.get(id));
  result.sort((left, right) => entityId(left, key) - entityId(right, key));
  collection(result, key);
  return result;
}

export function mergeCS16StreamData(previous: unknown, incoming: unknown): unknown {
  const packet = object(incoming);
  if (packet.schema === 1) { snapshot(packet); return packet; }
  if (packet.schema !== 2) return invalid();
  fields(packet, ['schema', 'epoch', 'baseFrame', 'baseEventId', 'frame', 'serverAt', 'eventId', 'global', 'collections', 'events']);
  const baseline = snapshot(previous);
  const frame = integer(packet.frame);
  const latest = integer(packet.eventId);
  if (packet.epoch !== baseline.epoch || integer(packet.baseFrame) !== baseline.frame || integer(packet.baseEventId) !== baseline.eventId || frame < integer(baseline.frame) || latest < integer(baseline.eventId)) return invalid();
  integer(packet.serverAt);
  const next = own(packet, 'global') ? applyObject(baseline, packet.global, true) : { ...baseline };
  if (own(packet, 'collections')) {
    const changes = object(packet.collections);
    fields(changes, collectionNames);
    for (const name of collectionNames) if (own(changes, name)) next[name] = applyCollection(baseline[name], changes[name], name === 'players' ? 'seat' : 'id');
  }
  const addedEvents = events(packet.events, latest, integer(baseline.eventId));
  json(addedEvents);
  return { ...next, schema: 1, epoch: packet.epoch, frame, serverAt: packet.serverAt, eventId: latest, events: addedEvents, recover: false };
}
