import type { CSSProperties } from 'react';
import { assetURL } from './asset-contract';
import type { HudAssets } from './hud-assets';
import type { Player, View } from './types';

export function HudSprite({ hud, base, name, label }: { hud?: HudAssets; base: string; name: string; label?: string }) {
  const sprite = hud?.sprites['640']?.[name];
  if (!hud || !sprite) return null;
  const image = `url("${assetURL(base, hud.atlas.file)}")`;
  const position = `${-sprite.x}px ${-sprite.y}px`;
  const style: CSSProperties = { width: sprite.width, height: sprite.height, maskImage: image, WebkitMaskImage: image, maskPosition: position, WebkitMaskPosition: position, maskSize: `${hud.width}px ${hud.height}px`, WebkitMaskSize: `${hud.width}px ${hud.height}px`, maskRepeat: 'no-repeat', WebkitMaskRepeat: 'no-repeat' };
  return <span className="cs16-hud-sprite" data-cs16-sprite={name} role={label ? 'img' : undefined} aria-label={label} aria-hidden={label ? undefined : true} style={style} />;
}

export function HudNumber({ hud, base, value, label }: { hud?: HudAssets; base: string; value: number; label?: string }) {
  const text = String(Math.max(0, Math.floor(value)));
  return <span className="cs16-hud-number" aria-label={label ?? text}>{hud ? [...text].map((digit, index) => <HudSprite hud={hud} base={base} key={index} name={`number_${digit}`} />) : text}</span>;
}

export function ClassicStatus({ hud, base, player, view, frame }: { hud?: HudAssets; base: string; player: Player; view: View; frame: number }) {
  if (!player.alive) return null;
  return <div data-chat-protected className="cs16-status-icons">
    {view.buyZone && <span data-cs16-status="buyzone"><HudSprite hud={hud} base={base} name="buyzone" label="购买区域" /></span>}
    {view.bomb.carrier === player.seat && <span data-cs16-status="c4" data-flashing={!!view.plantZone} style={{ visibility: view.plantZone && Math.floor(frame / 10) % 2 ? 'hidden' : 'visible' }}><HudSprite hud={hud} base={base} name="c4" label="携带 C4" /></span>}
  </div>;
}
