import { useEffect, useImperativeHandle, useRef, useState, type PointerEvent, type Ref } from 'react';
import type { TouchState } from './touch-input';
import { TouchOwnership, bindTouchOwnership } from './touch-ownership';
import type { WeaponState } from './types';

export type { TouchState } from './touch-input';
export type TouchControlsHandle = { clear: () => void };
export function TouchControls({ ref, disabled, changed, look, native, weapon, weaponState, useLabel, interrupted, begin, error, buy, score, nightVisionOwned, nightVision, flashlight }: { ref?: Ref<TouchControlsHandle>; disabled: boolean; changed: (state: TouchState) => void; look: (x: number, y: number) => void; native: (event: Event) => void; weapon: string; weaponState?: WeaponState; useLabel: string; interrupted: () => void; begin: () => void; error: (message: string) => void; buy?: () => void; score?: () => void; nightVisionOwned?: boolean; nightVision?: boolean; flashlight?: boolean }) {
  const [state, setState] = useState<TouchState>({ moveX: 0, moveY: 0, actions: new Set() });
  const [more, setMore] = useState(false);
  const latest = useRef({ disabled, changed, look, native, interrupted, begin, error });
  latest.current = { disabled, changed, look, native, interrupted, begin, error };
  const input = useRef<TouchOwnership | undefined>(undefined);
  if (!input.current) input.current = new TouchOwnership({
    locked: () => !!document.pointerLockElement,
    begin: () => latest.current.begin(),
    changed: value => { setState(value); latest.current.changed(value); },
    look: (x, y) => latest.current.look(x, y),
    interrupted: () => latest.current.interrupted(),
    failed: () => {},
  });
  useImperativeHandle(ref, () => ({ clear: () => input.current!.clear() }), []);
  const down = (event: PointerEvent<HTMLElement>, kind: string) => {
    if (disabled || event.button !== 0 || event.pointerType === 'mouse' || more && kind === 'fire') return;
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
  const secondary = weapon === 'knife' ? '重击' : weapon === 'm4a1' || weapon === 'usp' ? weaponState?.silenced ? '卸消音' : '装消音' : weapon === 'glock18' || weapon === 'famas' ? weaponState?.burst ? weapon === 'famas' ? '连发' : '单发' : '三连发' : weaponState?.zoom ? ['awp', 'scout', 'sg550', 'g3sg1'].includes(weapon) && weaponState.zoom === 40 ? '放大' : '退镜' : '开镜';
  const scoped = ['awp', 'scout', 'sg550', 'g3sg1', 'aug', 'sg552'].includes(weapon);
  const buttons = [...(scoped ? [['secondary', secondary]] : []), ['crouch', state.actions.has('crouch') ? '起身' : '蹲下'], ['jump', '跳跃']];
  return <div className="cs16-touch" aria-label="触屏控制">
    <div className="cs16-stick cs16-stick-move" data-touch-control data-cs16-stick="move" aria-label="移动摇杆" onPointerDown={event => down(event, 'move')}><span style={{ transform: `translate(${state.moveX * 27 / 1000}px,${-state.moveY * 27 / 1000}px)` }} /></div>
    <div className="cs16-look-zone" data-cs16-stick="look" data-cs16-fire-area="true" aria-label="按住开火并拖动瞄准" onPointerDown={event => down(event, 'fire')} />
    {more && <div className="cs16-touch-more-panel" role="group" aria-label="更多操作">
        <button type="button" data-touch-control data-cs16-action="use" disabled={disabled} onPointerDown={event => down(event, 'use')}>{useLabel}</button>
        {nightVisionOwned && <button type="button" data-touch-control data-cs16-action="nightVision" aria-pressed={!!nightVision} disabled={disabled} onPointerDown={event => down(event, 'nightVision')}>夜视仪</button>}
        <button type="button" data-touch-control data-cs16-action="drop" disabled={disabled} onPointerDown={event => down(event, 'drop')}>丢弃</button>
        {!scoped && ['knife', 'm4a1', 'usp', 'glock18', 'famas'].includes(weapon) && <button type="button" data-touch-control data-cs16-action="secondary" disabled={disabled} onPointerDown={event => down(event, 'secondary')}>{secondary}</button>}
        {score && <button type="button" data-touch-control onClick={() => { setMore(false); score(); }}>战绩</button>}
    </div>}
    <div className="cs16-touch-utility">
      <button type="button" data-touch-control data-cs16-action="reload" disabled={disabled} aria-label="装弹" onPointerDown={event => down(event, 'reload')}>装弹</button>
      <button type="button" data-touch-control data-cs16-action="flashlight" disabled={disabled} aria-label="手电筒" aria-pressed={!!flashlight} onPointerDown={event => down(event, 'flashlight')}>手电</button>
      {buy && <button type="button" data-touch-control data-cs16-open-buy disabled={disabled} aria-label="购买装备" onClick={() => { input.current!.clear(); setMore(false); buy(); }}>购买</button>}
      <button type="button" data-touch-control data-cs16-controls-menu="true" disabled={disabled} aria-expanded={more} aria-label="更多操作" onClick={() => { input.current!.clear(); setMore(value => !value); }}>更多</button>
    </div>
    <div className="cs16-touch-actions">{buttons.map(([action, label]) => <button type="button" data-touch-control key={action} data-cs16-action={action} disabled={disabled} aria-label={label} aria-pressed={action === 'crouch' ? state.actions.has('crouch') : undefined} onPointerDown={event => down(event, action)}>{label}</button>)}</div>
  </div>;
}
