import { useEffect, useRef, useState, type PointerEvent } from 'react';
import type { TouchState } from './touch-input';
import { TouchOwnership, bindTouchOwnership } from './touch-ownership';
import type { WeaponState } from './types';

export type { TouchState } from './touch-input';
export function TouchControls({ disabled, changed, look, native, weapon, weaponState, useLabel, interrupted, begin, error }: { disabled: boolean; changed: (state: TouchState) => void; look: (x: number, y: number) => void; native: (event: Event) => void; weapon: string; weaponState?: WeaponState; useLabel: string; interrupted: () => void; begin: () => void; error: (message: string) => void }) {
  const [state, setState] = useState<TouchState>({ moveX: 0, moveY: 0, actions: new Set() });
  const latest = useRef({ disabled, changed, look, native, interrupted, begin, error });
  latest.current = { disabled, changed, look, native, interrupted, begin, error };
  const input = useRef<TouchOwnership | undefined>(undefined);
  if (!input.current) input.current = new TouchOwnership({
    locked: () => !!document.pointerLockElement,
    begin: () => latest.current.begin(),
    changed: value => { setState(value); latest.current.changed(value); },
    look: (x, y) => latest.current.look(x, y),
    interrupted: () => latest.current.interrupted(),
    failed: () => latest.current.error('触屏捕获失败，请松开后重试。'),
  });
  const down = (event: PointerEvent<HTMLElement>, kind: string) => {
    if (disabled || event.button !== 0) return;
    native(event.nativeEvent);
    event.preventDefault();
    event.stopPropagation();
    input.current!.press(event.pointerId, kind, event.clientX, event.clientY, event.currentTarget);
  };
  useEffect(() => {
    if (!disabled) return;
    input.current!.clear();
  }, [disabled]);
  useEffect(() => {
    const clear = () => input.current!.clear();
    const hidden = () => { if (document.hidden) clear(); };
    window.addEventListener('blur', clear);
    document.addEventListener('visibilitychange', hidden);
    return () => { window.removeEventListener('blur', clear); document.removeEventListener('visibilitychange', hidden); };
  }, []);
  useEffect(() => bindTouchOwnership(window, document, input.current!, () => latest.current.disabled, event => latest.current.native(event)), []);
  const fireLabel = weapon === 'knife' ? '攻击' : weapon === 'c4' ? '安装' : ['hegrenade', 'flashbang', 'smokegrenade'].includes(weapon) ? '投掷' : '开火';
  const secondary = weapon === 'knife' ? '重击' : weapon === 'm4a1' || weapon === 'usp' ? weaponState?.silenced ? '卸消音' : '装消音' : weapon === 'glock18' || weapon === 'famas' ? weaponState?.burst ? weapon === 'famas' ? '连发' : '单发' : '三连发' : weaponState?.zoom ? '退镜' : '开镜';
  const buttons = [['secondary', secondary], ['use', useLabel], ['reload', '装弹'], ['crouch', state.actions.has('crouch') ? '起身' : '蹲下'], ['jump', '跳跃'], ['fire', fireLabel]];
  return <div className="cs16-touch" aria-label="触屏控制">
    <div className="cs16-stick cs16-stick-move" data-cs16-stick="move" aria-label="移动摇杆" onPointerDown={event => down(event, 'move')}><span style={{ transform: `translate(${state.moveX * 27 / 1000}px,${-state.moveY * 27 / 1000}px)` }} /></div>
    <div className="cs16-look-zone" data-cs16-stick="look" aria-label="拖动转向" onPointerDown={event => down(event, 'look')} />
    <div className="cs16-touch-actions">{buttons.map(([action, label]) => <button type="button" key={action} data-cs16-action={action} disabled={disabled} aria-label={label} aria-pressed={action === 'crouch' ? state.actions.has('crouch') : undefined} onPointerDown={event => down(event, action)}>{label}</button>)}</div>
  </div>;
}
