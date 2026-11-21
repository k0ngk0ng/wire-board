import { useRef } from 'react';
import { equipment, itemName, weapons } from './content';
import type { Player, WeaponInfo } from './types';

export function BuyMenu({ player, prices, allowed, buy, close, error, mode }: { player: Player; prices: WeaponInfo[]; allowed: boolean; buy: (item: string) => void; close: () => void; error: string; mode: string }) {
  const price = (id: string, fallback: number) => prices.find(weapon => weapon.id === id)?.price ?? fallback;
  const panel = useRef<HTMLElement>(null);
  const groups = [...new Set(weapons.map(weapon => weapon.group))];
  const reveal = (group: string) => {
    const selected = [...panel.current?.querySelectorAll<HTMLElement>('[data-cs16-buy-group]') ?? []].find(element => element.dataset.cs16BuyGroup === group);
    if (selected && panel.current) panel.current.scrollTo({ top: selected.offsetTop - panel.current.offsetTop - 112, behavior: 'smooth' });
  };
  return <div className="cs16-buy-backdrop" onPointerDown={event => event.stopPropagation()}>
    <section ref={panel} className="cs16-buy-menu" role="dialog" aria-modal="true" aria-label="购买装备" onKeyDown={event => { if (event.key === 'Escape') { event.stopPropagation(); close(); } }}>
      <header><strong>购买装备</strong><span>${player.money ?? 0}</span><button type="button" data-controller-cancel onClick={close} aria-label="关闭购买菜单">返回战场</button></header>
      {!allowed && <p className="cs16-buy-notice">仅可在购买时段内于己方购买区购买。</p>}
      <nav className="cs16-buy-tabs" aria-label="装备类别">{[...groups, '装备'].map(group => <button type="button" key={group} onClick={() => reveal(group)}>{group}</button>)}</nav>
      <div className="cs16-buy-groups">{groups.map(group => <section key={group} data-cs16-buy-group={group}><h3>{group}</h3><div>{weapons.filter(weapon => weapon.group === group).map(weapon => <button type="button" key={weapon.id} data-cs16-buy={weapon.id} disabled={!allowed || !!weapon.team && weapon.team !== player.team || (player.money ?? 0) < price(weapon.id, weapon.price)} title={weapon.team && weapon.team !== player.team ? weapon.team === 'ct' ? '反恐精英专用' : '恐怖分子专用' : undefined} onClick={() => buy(weapon.id)}><span>{weapon.name}</span><span>${price(weapon.id, weapon.price)}</span></button>)}</div></section>)}</div>
      <section className="cs16-buy-equipment" data-cs16-buy-group="装备"><h3>装备与弹药</h3><div>{equipment.filter(item => item.id !== 'defuser' || player.team === 'ct' && mode === 'bomb').map(item => <button type="button" key={item.id} data-cs16-buy={item.id} disabled={!allowed || (player.money ?? 0) < item.price} onClick={() => buy(item.id)}><span>{itemName(item.id)}</span><span>{item.price ? `$${item.price}` : '按弹药计价'}</span></button>)}</div></section>
      {error && <p className="cs16-error" role="alert">{error}</p>}
    </section>
  </div>;
}
