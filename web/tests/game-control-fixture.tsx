import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { GameFullscreen, type GameFullscreenHandle } from "../src/ui/room-fullscreen";
import { TouchControls } from "../../games/hotlinemiami/web/TouchControls";
import { BottomHint } from "../../games/hotlinemiami/web/BottomHint";
import "../../games/hotlinemiami/web/style.css";

import { installControlContextMenuGuard } from "../src/control-context-menu";

type Stick = { x: number; y: number };
type Contact = { id: number; target: string; x: number; y: number };
type NativeRecord = { serial: number; type: string; trusted: boolean; kind: string; target: string; id?: number; pointerType?: string; changed?: Contact[]; touches?: Contact[] };
const probe = { move: [] as Stick[], aim: [] as Stick[], edges: [] as string[], native: [] as NativeRecord[], events: [] as NativeRecord[], menus: [] as { target: string; prevented: boolean; trusted: boolean }[], custom: 0, clicks: 0 };
const eventIds = new WeakMap<Event, number>();
let serial = 0;
Object.assign(window, { fixture: probe });

function nativeRecord(event: Event): NativeRecord {
  if (!eventIds.has(event)) eventIds.set(event, ++serial);
  const target = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-hotlinemiami-stick]")?.dataset.hotlinemiamiStick ?? "" : "";
  const base = { serial: eventIds.get(event)!, type: event.type, trusted: event.isTrusted, target };
  if (event instanceof TouchEvent) {
    const contact = (point: Touch): Contact => ({ id: point.identifier, target: point.target instanceof Element ? point.target.closest<HTMLElement>("[data-hotlinemiami-stick]")?.dataset.hotlinemiamiStick ?? "" : "", x: point.clientX, y: point.clientY });
    return { ...base, kind: "touch", changed: Array.from(event.changedTouches, contact), touches: Array.from(event.touches, contact) };
  }
  if (event instanceof PointerEvent) return { ...base, kind: "pointer", id: event.pointerId, pointerType: event.pointerType };
  return { ...base, kind: "other" };
}

function observe(event: Event) {
  const record = nativeRecord(event);
  if (record.target || record.changed?.some(contact => contact.target)) probe.events.push(record);
}

function Fixture() {
  useEffect(() => installControlContextMenuGuard(document), []);
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [ready, setReady] = useState(false);
  return <main onContextMenu={event => {
    const target = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-probe]")?.dataset.probe ?? event.target.tagName : "";
    probe.menus.push({ target, prevented: event.defaultPrevented, trusted: event.nativeEvent.isTrusted });
  }}>
    <button data-enter disabled={!ready} onClick={() => fullscreen.current?.enter()}>全屏</button>
    <GameFullscreen ref={fullscreen} scope="guard-fixture" enabled toolbarActions={<span data-probe="toolbar">托管状态</span>} onReadyChange={setReady} result={<aside data-probe="overlay">聊天和玩法说明可正常选择</aside>}>
      <section className="hm-board"
        onPointerDownCapture={event => observe(event.nativeEvent)} onPointerMoveCapture={event => observe(event.nativeEvent)} onPointerUpCapture={event => observe(event.nativeEvent)} onPointerCancelCapture={event => observe(event.nativeEvent)}
        onTouchStartCapture={event => observe(event.nativeEvent)} onTouchMoveCapture={event => observe(event.nativeEvent)} onTouchEndCapture={event => observe(event.nativeEvent)} onTouchCancelCapture={event => observe(event.nativeEvent)}>
        <div className="hm-battle-heading" data-fullscreen-protected>
          <div><span className="hm-map-label" data-probe="heading">无声行动</span><strong data-probe="hint">自由混战</strong></div>
          <time className="hm-timer">2:49</time><div className="hm-ammo"><strong>M16 步枪</strong><span>30 发</span></div>
        </div>
        <div className="hm-score-strip"><div className="hm-score hm-score-self"><span>当前玩家</span><strong>0</strong></div></div>
        <div className="hm-surface">
          <canvas data-probe="canvas" width="320" height="480" />
          <div className="fixture-pictures"><img data-probe="image" src="demo.svg" /><a data-probe="link" href="#image">地图图片</a><div tabIndex={0} data-probe="custom" onContextMenu={() => probe.custom++}>游戏自定义右键</div></div>
          <TouchControls disabled={false} native={event => probe.native.push(nativeRecord(event))} move={value => probe.move.push(value)} aim={value => probe.aim.push(value)} edge={value => probe.edges.push(value)} />
          <BottomHint player={{ weapon: "m16", ammo: 30, alive: true, downUntil: 0, protectedUntil: 0 }} frame={100} finished={false} />
        </div>
      </section>
      <section data-probe="editors"><input data-probe="input" defaultValue="可以复制的输入" /><textarea data-probe="textarea" defaultValue="可以复制的文本域" /><div data-probe="editable" contentEditable suppressContentEditableWarning>可以复制的编辑内容</div><select data-probe="select"><option>保留选择器</option></select></section>
    </GameFullscreen>
    <button data-probe="ordinary" onClick={() => probe.clicks++}><span>普通按钮</span></button>
    <aside data-outside><p data-probe="outside">聊天和玩法说明仍然允许正常选择复制</p><input data-probe="outside-input" defaultValue="区外输入" /></aside>
  </main>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
