import assert from "node:assert/strict";

const prefix = "games/hotlinemiami/assets/public/hotlinemiami/";

function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === "object") return Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])]));
  return value;
}

function same(left, right) {
  return JSON.stringify(canonical(left)) === JSON.stringify(canonical(right));
}

function identity(record) {
  return [record.sha256, record.bytes, record.mime, record.encoding ?? ""].join("\0");
}

function validateRecord(file, record) {
  assert(file.startsWith(prefix) && !file.includes("\\") && !file.split("/").some(part => !part || part === "." || part === ".."), "Resource path is outside Hotline scope");
  assert(record && Object.keys(record).every(key => ["key", "bytes", "sha256", "mime", "encoding"].includes(key)), "Invalid resource record fields");
  assert(Number.isSafeInteger(record.bytes) && record.bytes > 0 && /^[0-9a-f]{64}$/.test(record.sha256) && typeof record.mime === "string" && record.mime, "Invalid resource record identity");
  assert([undefined, "", "gzip"].includes(record.encoding) && new RegExp("^bga/hotlinemiami/" + record.sha256 + (record.encoding === "gzip" ? "-gzip" : "") + "\\.[a-z0-9]+$").test(record.key), "Resource key differs from immutable byte identity");
}

function historicalEntry(entry, file, record) {
  return entry.path === file && Object.keys(record).every(key => same(entry[key], record[key]));
}

export function validateHotlineResourceDelta(beforeRegistry, beforeHistory, afterRegistry, afterHistory, sourceCommit) {
  assert(/^[0-9a-f]{40}$/.test(sourceCommit), "Invalid archived source commit");
  for (const registry of [beforeRegistry, afterRegistry]) assert(registry.version === 1 && registry.files && !Array.isArray(registry.files), "Invalid source resource registry");
  for (const history of [beforeHistory, afterHistory]) assert(history.version === 1 && Array.isArray(history.files), "Invalid resource history");
  const foreignBefore = Object.fromEntries(Object.entries(beforeRegistry.files).filter(([file]) => !file.startsWith(prefix)));
  const foreignAfter = Object.fromEntries(Object.entries(afterRegistry.files).filter(([file]) => !file.startsWith(prefix)));
  assert(same(foreignBefore, foreignAfter), "Unrelated resource records changed");
  assert(same({ ...beforeRegistry, files: undefined }, { ...afterRegistry, files: undefined }), "Unrelated registry metadata changed");
  assert(same({ ...beforeHistory, files: undefined }, { ...afterHistory, files: undefined }), "Unrelated history metadata changed");
  assert(same(beforeHistory.files, afterHistory.files.slice(0, beforeHistory.files.length)), "Existing history records changed");
  const previous = Object.fromEntries(Object.entries(beforeRegistry.files).filter(([file]) => file.startsWith(prefix)));
  const current = Object.fromEntries(Object.entries(afterRegistry.files).filter(([file]) => file.startsWith(prefix)));
  for (const [file, record] of [...Object.entries(previous), ...Object.entries(current)]) validateRecord(file, record);
  const known = new Map();
  for (const record of [...Object.values(previous), ...beforeHistory.files.filter(entry => entry.path?.startsWith(prefix))]) if (!known.has(identity(record))) known.set(identity(record), record.key);
  const reusedFiles = [];
  const changedFiles = [];
  const addedFiles = [];
  const requiredHistory = [];
  for (const [file, record] of Object.entries(previous)) {
    assert(current[file], "Existing resource path was removed: " + file);
    if (identity(record) === identity(current[file])) {
      assert(same(record, current[file]), "Unchanged resource got a different key or metadata: " + file);
      reusedFiles.push(file);
    } else {
      assert(record.key !== current[file].key, "Changed resource attempted to overwrite an immutable key: " + file);
      changedFiles.push(file);
      if (!beforeHistory.files.some(entry => historicalEntry(entry, file, record))) requiredHistory.push({ path: file, ...record, sourceCommit });
    }
  }
  for (const [file, record] of Object.entries(current)) {
    if (!previous[file]) addedFiles.push(file);
    const knownKey = known.get(identity(record));
    if (knownKey) assert(record.key === knownKey, "Existing byte identity must reuse its original key: " + file);
  }
  const appended = afterHistory.files.slice(beforeHistory.files.length);
  const orderedAppended = appended.map(entry => JSON.stringify(canonical(entry))).sort();
  const orderedRequired = requiredHistory.map(entry => JSON.stringify(canonical(entry))).sort();
  assert(same(orderedAppended, orderedRequired), "History does not contain exactly the replaced original records");
  const knownKeys = new Set([...Object.values(previous), ...beforeHistory.files.filter(entry => entry.path?.startsWith(prefix))].map(record => record.key));
  const newObjects = [...new Set(Object.values(current).map(record => record.key))].filter(key => !knownKeys.has(key)).sort();
  return { passed: true, reusedFiles: reusedFiles.sort(), changedFiles: changedFiles.sort(), addedFiles: addedFiles.sort(), newObjects, archivedRecords: appended.length };
}
