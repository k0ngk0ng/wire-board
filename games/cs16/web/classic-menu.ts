import { itemName } from './content.ts';

export type BuyEntry = { key: number; label: string; item?: string; category?: number };
export const buyCategories = ['购买装备', '手枪', '霰弹枪', '冲锋枪', '步枪', '机枪', '', '', '装备'];

export function classicBuyEntries(team: string, category: number, mode: string): BuyEntry[] {
  const ct = team === 'ct';
  if (category === 0) return [1, 2, 3, 4, 5, 6, 7, 8].map(key => key === 6 || key === 7 ? { key, item: key === 6 ? 'primammo' : 'secammo', label: key === 6 ? '主武器弹药' : '副武器弹药' } : { key, category: key, label: buyCategories[key] });
  const groups: Record<number, string[]> = {
    1: ['glock18', 'usp', 'p228', 'deagle', ct ? 'fiveseven' : 'elite'],
    2: ['m3', 'xm1014'],
    3: [ct ? 'tmp' : 'mac10', 'mp5navy', 'ump45', 'p90'],
    4: ct ? ['famas', 'scout', 'm4a1', 'aug', 'sg550', 'awp'] : ['galil', 'ak47', 'scout', 'sg552', 'awp', 'g3sg1'],
    5: ['m249'],
    8: ['vest', 'vesthelm', 'flashbang', 'hegrenade', 'smokegrenade'],
  };
  const entries = (groups[category] ?? []).map((item, index) => ({ key: index + 1, item, label: itemName(item) }));
  if (category === 8 && ct && mode === 'bomb') entries.push({ key: 7, item: 'defuser', label: itemName('defuser') });
  return entries;
}

export function classicBuyChoice(team: string, category: number, key: number, mode: string): { category?: number; item?: string; close?: boolean } {
  if (key === 0) return { close: true };
  const entry = classicBuyEntries(team, category, mode).find(entry => entry.key === key);
  return entry?.item ? { item: entry.item, close: true } : entry?.category ? { category: entry.category } : {};
}
