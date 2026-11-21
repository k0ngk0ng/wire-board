import type { GameOptions } from './types.ts';

export const maps = [
  { id: 'de_dust2', name: '沙漠 2', mode: 'bomb' },
  { id: 'de_inferno', name: '炼狱小镇', mode: 'bomb' },
  { id: 'de_aztec', name: '阿兹特克', mode: 'bomb' },
  { id: 'cs_assault', name: '仓库突击', mode: 'hostage' },
  { id: 'fy_iceworld', name: '冰雪世界', mode: 'elimination' },
  { id: 'cs_bloodstrike', name: '血战回廊', mode: 'elimination' },
] as const;

export const characters = [
  { id: 'terror', name: '凤凰战士', team: 't' }, { id: 'leet', name: '精英部队', team: 't' },
  { id: 'arctic', name: '北极复仇者', team: 't' }, { id: 'guerilla', name: '游击队员', team: 't' },
  { id: 'urban', name: '海豹突击队', team: 'ct' }, { id: 'gsg9', name: '德国边防军', team: 'ct' },
  { id: 'sas', name: '英国特勤队', team: 'ct' }, { id: 'gign', name: '法国宪兵', team: 'ct' },
] as const;

export const weapons = [
  { id: 'glock18', name: 'Glock 18', group: '手枪', slot: 2, team: '', price: 400 },
  { id: 'usp', name: 'USP', group: '手枪', slot: 2, team: '', price: 500 },
  { id: 'p228', name: 'P228', group: '手枪', slot: 2, team: '', price: 600 },
  { id: 'deagle', name: '沙漠之鹰', group: '手枪', slot: 2, team: '', price: 650 },
  { id: 'elite', name: '双持 Beretta', group: '手枪', slot: 2, team: 't', price: 800 },
  { id: 'fiveseven', name: 'Five-SeveN', group: '手枪', slot: 2, team: 'ct', price: 750 },
  { id: 'm3', name: 'M3', group: '霰弹枪', slot: 1, team: '', price: 1700 },
  { id: 'xm1014', name: 'XM1014', group: '霰弹枪', slot: 1, team: '', price: 3000 },
  { id: 'mac10', name: 'MAC-10', group: '冲锋枪', slot: 1, team: 't', price: 1400 },
  { id: 'tmp', name: 'TMP', group: '冲锋枪', slot: 1, team: 'ct', price: 1250 },
  { id: 'mp5navy', name: 'MP5', group: '冲锋枪', slot: 1, team: '', price: 1500 },
  { id: 'ump45', name: 'UMP45', group: '冲锋枪', slot: 1, team: '', price: 1700 },
  { id: 'p90', name: 'P90', group: '冲锋枪', slot: 1, team: '', price: 2350 },
  { id: 'galil', name: 'Galil', group: '步枪', slot: 1, team: 't', price: 2000 },
  { id: 'famas', name: 'FAMAS', group: '步枪', slot: 1, team: 'ct', price: 2250 },
  { id: 'ak47', name: 'AK-47', group: '步枪', slot: 1, team: 't', price: 2500 },
  { id: 'm4a1', name: 'M4A1', group: '步枪', slot: 1, team: 'ct', price: 3100 },
  { id: 'sg552', name: 'SG552', group: '步枪', slot: 1, team: 't', price: 3500 },
  { id: 'aug', name: 'AUG', group: '步枪', slot: 1, team: 'ct', price: 3500 },
  { id: 'scout', name: 'Scout', group: '狙击枪', slot: 1, team: '', price: 2750 },
  { id: 'awp', name: 'AWP', group: '狙击枪', slot: 1, team: '', price: 4750 },
  { id: 'g3sg1', name: 'G3SG1', group: '狙击枪', slot: 1, team: 't', price: 5000 },
  { id: 'sg550', name: 'SG550', group: '狙击枪', slot: 1, team: 'ct', price: 4200 },
  { id: 'm249', name: 'M249', group: '机枪', slot: 1, team: '', price: 5750 },
] as const;

export const equipment = [
  { id: 'vest', name: '防弹衣', price: 650 }, { id: 'vesthelm', name: '防弹衣与头盔', price: 1000 },
  { id: 'defuser', name: '拆弹器', price: 200 }, { id: 'hegrenade', name: '高爆手雷', price: 300 },
  { id: 'flashbang', name: '闪光弹', price: 200 }, { id: 'smokegrenade', name: '烟雾弹', price: 300 },
  { id: 'primammo', name: '主武器弹药', price: 0 }, { id: 'secammo', name: '副武器弹药', price: 0 },
] as const;

export const itemName = (id: string) => weapons.find(weapon => weapon.id === id)?.name ?? equipment.find(item => item.id === id)?.name ?? ({ knife: '小刀', c4: 'C4' } as Record<string, string>)[id] ?? id;
export const mapName = (id: string) => maps.find(map => map.id === id)?.name ?? id;
export const mapMode = (id: string) => maps.find(map => map.id === id)?.mode;
export const modeName = (mode: string) => ({ bomb: '爆破', hostage: '人质营救', elimination: '回合歼灭' } as Record<string, string>)[mode] ?? mode;
export function inventorySlots(inventory: { id: string; ammo: number }[], current: string) {
  const utilities = inventory.filter(item => ['hegrenade', 'flashbang', 'smokegrenade'].includes(item.id) && item.ammo > 0);
  const selected = utilities.findIndex(item => item.id === current);
  const slots = inventory.filter(item => !['hegrenade', 'flashbang', 'smokegrenade'].includes(item.id)).map(item => ({ id: item.id, next: item.id, label: itemName(item.id) }));
  if (utilities.length) {
    const active = utilities[Math.max(0, selected)];
    slots.push({ id: active.id, next: utilities[selected < 0 ? 0 : (selected + 1) % utilities.length].id, label: '投掷物' });
  }
  return slots;
}
export const defaultOptions: GameOptions = { map: 'de_dust2', roundSeconds: 120, maxRounds: 10, friendlyFire: false, teams: [], characters: [] };

export function normalizeOptions(value: GameOptions, capacity: number): GameOptions {
  const teams = Array.from({ length: capacity }, (_, seat) => value.teams?.[seat] === 'ct' || value.teams?.[seat] === 't' ? value.teams[seat] : seat % 2 ? 'ct' : 't');
  const selected = Array.from({ length: capacity }, (_, seat) => characters.find(character => character.id === value.characters?.[seat] && character.team === teams[seat])?.id ?? (teams[seat] === 'ct' ? 'urban' : 'terror'));
  return { ...defaultOptions, ...value, friendlyFire: value.friendlyFire ?? false, teams, characters: selected };
}
