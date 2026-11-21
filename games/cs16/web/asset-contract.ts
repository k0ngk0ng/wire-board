import { maps, weapons } from './content.ts';
import type { AssetManifest, Resource } from './types.ts';

export function assetURL(base: string, file: string) {
  if (/^https?:\/\//.test(file)) return file;
  if (file.startsWith('/')) return file;
  return `${base.replace(/\/$/, '')}/cs16/${file}`;
}

export function validateResource(value: Resource): Resource {
  if (!value || typeof value.file !== 'string' || !value.file || /\\|(?:^|\/)\.\.(?:\/|$)/.test(value.file) || /^[a-z][a-z0-9+.-]*:/i.test(value.file) && !/^https?:\/\//.test(value.file)) throw Error('素材路径无效');
  if (!Number.isSafeInteger(value.bytes) || value.bytes < 0 || !/^[a-f0-9]{64}$/.test(value.sha256)) throw Error('素材身份无效');
  return value;
}

export function validateRulesIdentity(resource: Resource | undefined, authoritative: string | undefined) {
  const identity = resource?.encoding === 'gzip' ? resource.decodedSHA256 : resource?.sha256;
  if (!authoritative || !/^[a-f0-9]{64}$/.test(authoritative) || authoritative !== identity) throw Error('地图规则版本不一致，请重新加载本端');
}

export function validateManifest(value: AssetManifest): AssetManifest {
  if (!value || value.schema !== 1 || !value.maps || !value.models || !value.sounds) throw Error('素材清单格式无效');
  for (const map of maps) {
    const selected = value.maps[map.id];
    if (!selected || selected.mode !== map.mode) throw Error(`缺少原版地图：${map.name}`);
    for (const resource of [selected.render, selected.rules, selected.preview, ...selected.sky ?? []]) validateResource(resource!);
  }
  for (const resource of [...Object.values(value.models), ...Object.values(value.sounds)]) validateResource(resource);
  if (value.runtime) {
    if (value.runtime.version !== 1 || !/^[a-f0-9]{64}$/.test(value.runtime.sourceSHA256) || !/^[a-f0-9]{64}$/.test(value.runtime.buildSHA256)) throw Error('预测运行时身份无效');
    validateResource(value.runtime.wasm);
    validateResource(value.runtime.script);
  }
  return value;
}

export function selectModelKeys(characters: string[], mode = 'bomb') {
  const names = weapons.map(weapon => weapon.id === 'mp5navy' ? 'mp5' : weapon.id);
  return [...new Set([...characters.map(id => `player-${id}`), ...names.flatMap(id => [`v-${id}`, `p-${id}`, `w-${id}`]), ...['knife', 'hegrenade', 'flashbang', 'smokegrenade', 'c4'].flatMap(id => [`v-${id}`, `p-${id}`, `w-${id}`]), 'w-backpack', ...(mode === 'hostage' ? ['hostage'] : [])])];
}
