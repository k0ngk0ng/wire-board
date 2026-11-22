import { maps, weapons } from './content.ts';
import type { AssetManifest, EffectSprite, Resource } from './types.ts';

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

export function validateEffectSprite(sprite: EffectSprite) {
  if (!sprite) throw Error('缺少原版战斗特效');
  validateResource(sprite.atlas);
  if (![sprite.width, sprite.height, sprite.textureFormat, sprite.orientation].every(Number.isSafeInteger) || sprite.width <= 0 || sprite.height <= 0 || ![0, 1, 2, 3].includes(sprite.textureFormat) || ![0, 1, 2, 3, 4].includes(sprite.orientation) || !['normal', 'additive'].includes(sprite.blend) || !/^[a-f0-9]{64}$/.test(sprite.sourceSHA256) || !Array.isArray(sprite.frames) || !sprite.frames.length) throw Error('原版战斗特效格式无效');
  if (sprite.tint && (sprite.tint.length !== 3 || !sprite.tint.every(value => Number.isSafeInteger(value) && value >= 0 && value <= 255)) || sprite.paletteIndex !== undefined && (!Number.isSafeInteger(sprite.paletteIndex) || sprite.paletteIndex < 0 || sprite.paletteIndex > 255)) throw Error('原版战斗特效调色无效');
  for (const frame of sprite.frames) {
    if (!Array.isArray(frame.origin) || frame.origin.length !== 2 || ![frame.x, frame.y, frame.width, frame.height, ...frame.origin].every(Number.isSafeInteger) || frame.x < 0 || frame.y < 0 || frame.width <= 0 || frame.height <= 0 || frame.x + frame.width > sprite.width || frame.y + frame.height > sprite.height) throw Error('原版战斗特效帧越界');
  }
  return sprite;
}

export function validateManifest(value: AssetManifest): AssetManifest {
  if (!value || value.schema !== 1 || !value.maps || !value.models || !value.sounds) throw Error('素材清单格式无效');
  for (const map of maps) {
    const selected = value.maps[map.id];
    if (!selected || selected.mode !== map.mode) throw Error(`缺少原版地图：${map.name}`);
    for (const resource of [selected.render, selected.rules, selected.preview, ...selected.sky ?? []]) validateResource(resource!);
  }
  for (const resource of [...Object.values(value.models), ...Object.values(value.sounds)]) validateResource(resource);
  if (value.effects) for (const name of ['blood', 'bloodspray', 'fexplo', 'eexplo'] as const) validateEffectSprite(value.effects[name]);
  if (value.hud) {
    const hud = value.hud;
    validateResource(hud.atlas);
    if (hud.schema !== 1 || !Number.isSafeInteger(hud.width) || hud.width <= 0 || !Number.isSafeInteger(hud.height) || hud.height <= 0) throw Error('原版界面素材无效');
    for (const bank of Object.values(hud.sprites)) for (const sprite of Object.values(bank)) {
      if (![sprite.x, sprite.y, sprite.width, sprite.height].every(Number.isSafeInteger) || sprite.x < 0 || sprite.y < 0 || sprite.width <= 0 || sprite.height <= 0 || sprite.x + sprite.width > hud.width || sprite.y + sprite.height > hud.height) throw Error('原版界面图标越界');
    }
  }
  if (value.runtime) {
    if (value.runtime.version !== 1 || !/^[a-f0-9]{64}$/.test(value.runtime.sourceSHA256) || !/^[a-f0-9]{64}$/.test(value.runtime.buildSHA256)) throw Error('预测运行时身份无效');
    validateResource(value.runtime.wasm);
    validateResource(value.runtime.script);
  }
  return value;
}

export function selectSoundKeys(sounds: Record<string, unknown>, mode: string) {
  return Object.keys(sounds).filter(key => key.startsWith('weapons/') || key.startsWith('player/') || ['items/9mmclip1.wav', 'items/flashlight1.wav', 'items/gunpickup2.wav', 'items/kevlar.wav', 'items/equip_nvg.wav', 'items/nvg_on.wav', 'items/nvg_off.wav'].includes(key) || /radio\/(?:bombpl|bombdef|ctwin|terwin|rounddraw|rescued)/.test(key) || mode === 'hostage' && key.startsWith('hostage/'));
}

export function selectModelKeys(characters: string[], mode = 'bomb') {
  const names = weapons.map(weapon => weapon.id === 'mp5navy' ? 'mp5' : weapon.id);
  return [...new Set([...characters.map(id => `player-${id}`), ...names.flatMap(id => [`v-${id}`, `p-${id}`, `w-${id}`]), ...['knife', 'hegrenade', 'flashbang', 'smokegrenade', 'c4'].flatMap(id => [`v-${id}`, `p-${id}`, `w-${id}`]), 'w-backpack', ...(mode === 'bomb' ? ['w-defuser'] : []), ...(mode === 'hostage' ? ['hostage'] : [])])];
}
