export function mediaResourceFacts(manifest) {
  const canonical = /^(?!.*(?:^|\/)\.\.?(?:\/|$))[A-Za-z0-9][A-Za-z0-9._/-]*$/;
  const files = [];
  const aliases = {};
  const seen = new Set();
  for (const file of manifest.files) {
    if (!canonical.test(file.path) || seen.has(file.path)) throw Error(`Noncanonical or duplicate public asset path: ${file.path}`);
    seen.add(file.path);
    if (/\.(?:m?js|css|json)$/.test(file.path)) continue;
    if (!Number.isSafeInteger(file.bytes) || file.bytes <= 0 || !/^[a-f0-9]{64}$/.test(file.sha256) || typeof file.mime !== "string") throw Error(`Invalid public asset identity: ${file.path}`);
    files.push({ path: file.path, bytes: file.bytes, sha256: file.sha256, mime: file.mime, ...(file.width ? { width: file.width, height: file.height } : {}) });
    if (file.audio?.logicalPath && file.audio.logicalPath !== file.path) {
      if (!canonical.test(file.audio.logicalPath) || aliases[file.audio.logicalPath]) throw Error(`Invalid audio alias path: ${file.audio.logicalPath}`);
      aliases[file.audio.logicalPath] = file.path;
    }
  }
  for (const [logical, actual] of Object.entries(aliases)) if (seen.has(logical) || !seen.has(actual) || aliases[actual]) throw Error(`Ambiguous audio alias: ${logical}`);
  return { files, aliases };
}
