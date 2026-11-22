import { useState } from 'react';
import { equipment, weapons } from './content';
import { buyCategories, classicBuyEntries } from './classic-menu';
import type { Player, WeaponInfo } from './types';

export function BuyMenu({ player, prices, allowed, buy, close, mode, category = 0, choose }: { player: Player; prices: WeaponInfo[]; allowed: boolean; buy: (item: string) => void; close: () => void; error: string; mode: string; category?: number; choose?: (key: number) => void }) {
  const [localCategory, setLocalCategory] = useState(category);
  const active = choose ? category : localCategory;
  const entries = classicBuyEntries(player.team, active, mode);
  const price = (id: string) => prices.find(weapon => weapon.id === id)?.price ?? weapons.find(weapon => weapon.id === id)?.price ?? equipment.find(item => item.id === id)?.price ?? 0;
  const activate = (key: number) => {
    if (choose) { choose(key); return; }
    const entry = entries.find(entry => entry.key === key);
    if (entry?.category) setLocalCategory(entry.category);
    else if (entry?.item && allowed) { buy(entry.item); close(); }
  };
  return <div className="cs16-buy-backdrop" onPointerDown={event => event.stopPropagation()}>
    <section className="cs16-buy-menu" role="dialog" aria-modal="true" aria-label="购买装备" onKeyDown={event => { if (event.key === 'Escape') { event.stopPropagation(); close(); } }}>
      <header><strong>{buyCategories[active]}</strong><span>${player.money ?? 0}</span></header>
      <div className="cs16-buy-list">{entries.map(entry => <button type="button" key={entry.key} data-cs16-buy={entry.item} data-cs16-buy-key={entry.key} disabled={!!entry.item && (!allowed || (player.money ?? 0) < price(entry.item))} onClick={() => activate(entry.key)}><span>{entry.key}. {entry.label}</span>{entry.item && price(entry.item) > 0 && <span>${price(entry.item)}</span>}</button>)}</div>
      <button type="button" data-controller-cancel data-cs16-buy-key="0" onClick={close} aria-label="关闭购买菜单">0. 退出</button>
    </section>
  </div>;
}
