import { battleHint, type HudPlayer } from "./hud";

export function BottomHint({ player, frame, finished }: { player?: HudPlayer; frame: number; finished: boolean }) {
  const hint = battleHint(player, frame, finished);
  return <div className="hm-bottom-line" data-hotlinemiami-hint={hint.kind}>
    <span aria-live="polite">{hint.kind === "controls" ? <><span className="hm-desktop-hint">WASD 移动 · 左键攻击 · 右键投掷 · E 拾取</span><span className="hm-touch-hint">右摇杆外推攻击 · 拾取与投掷换武器</span></> : hint.text}</span>
    <span>击杀 +1</span>
  </div>;
}
