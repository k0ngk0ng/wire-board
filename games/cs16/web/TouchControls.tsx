import { useEffect, useRef, useState, type PointerEvent } from 'react';
import { TouchInput, type TouchState } from './touch-input';
import type { WeaponState } from './types';

export type { TouchState } from './touch-input';
export function TouchControls({ disabled, changed, look, native, weapon, weaponState, useLabel, interrupted }: { disabled: boolean; changed: (state: TouchState) => void; look: (x: number, y: number) => void; native: (event: Event) => void; weapon: string; weaponState?: WeaponState; useLabel: string; interrupted: () => void }) {
  const input = useRef(new TouchInput());
  const [state, setState] = useState<TouchState>({ moveX: 0, moveY: 0, actions: new Set() });
  const latest = useRef(changed);
  latest.current = changed;
  const publish = () => { const value = input.current.sample(); setState(value); latest.current(value); };
  const down = (event: PointerEvent<HTMLElement>, kind: string) => {
    if (disabled || event.button !== 0) return;
    native(event.nativeEvent);
    event.preventDefault();
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    input.current.press(event.pointerId, kind, event.clientX, event.clientY);
    publish();
  };
  const move = (event: PointerEvent<HTMLElement>) => {
    if (disabled) return;
    const delta = input.current.move(event.pointerId, event.clientX, event.clientY);
    native(event.nativeEvent);
    event.preventDefault();
    if (delta) look(delta.x, delta.y);
    else publish();
  };
  const up = (event: PointerEvent<HTMLElement>, cancelled = false) => {
    const kind = input.current.release(event.pointerId, cancelled);
    if (!kind) return;
    if (cancelled && kind === 'fire') interrupted();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    publish();
  };
  useEffect(() => {
    if (!disabled) return;
    input.current.clear();
    publish();
  }, [disabled]);
  useEffect(() => {
    const clear = () => { input.current.clear(); publish(); };
    const hidden = () => { if (document.hidden) clear(); };
    window.addEventListener('blur', clear);
    document.addEventListener('visibilitychange', hidden);
    return () => { window.removeEventListener('blur', clear); document.removeEventListener('visibilitychange', hidden); };
  }, []);
  const handlers = { onPointerMove: move, onPointerUp: (event: PointerEvent<HTMLElement>) => up(event), onPointerCancel: (event: PointerEvent<HTMLElement>) => up(event, true), onLostPointerCapture: (event: PointerEvent<HTMLElement>) => up(event, true) };
  const fireLabel = weapon === 'knife' ? '攻击' : weapon === 'c4' ? '安装' : ['hegrenade', 'flashbang', 'smokegrenade'].includes(weapon) ? '投掷' : '开火';
  const secondary = weapon === 'knife' ? '重击' : weapon === 'm4a1' || weapon === 'usp' ? weaponState?.silenced ? '卸消音' : '装消音' : weapon === 'glock18' || weapon === 'famas' ? weaponState?.burst ? weapon === 'famas' ? '连发' : '单发' : '三连发' : weaponState?.zoom ? '退镜' : '开镜';
  const buttons = [['secondary', secondary], ['use', useLabel], ['reload', '装弹'], ['crouch', state.actions.has('crouch') ? '起身' : '蹲下'], ['jump', '跳跃'], ['fire', fireLabel]];
  return <div className="cs16-touch" aria-label="触屏控制">
    <div className="cs16-stick cs16-stick-move" data-cs16-stick="move" aria-label="移动摇杆" onPointerDown={event => down(event, 'move')} {...handlers}><span style={{ transform: `translate(${state.moveX * 27 / 1000}px,${-state.moveY * 27 / 1000}px)` }} /></div>
    <div className="cs16-look-zone" data-cs16-stick="look" aria-label="拖动转向" onPointerDown={event => down(event, 'look')} {...handlers} />
    <div className="cs16-touch-actions">{buttons.map(([action, label]) => <button type="button" key={action} data-cs16-action={action} disabled={disabled} aria-label={label} aria-pressed={action === 'crouch' ? state.actions.has('crouch') : undefined} onPointerDown={event => down(event, action)} {...handlers}>{label}</button>)}</div>
  </div>;
}
