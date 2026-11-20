import { useEffect, useState } from "react";
import type { OptionsProps, PreparationProps } from "../../../web/src/game-api/module";
import { assetURL, imageResource, loadManifest } from "./assets";
import { characterNames, mapNames, weaponNames, type AssetManifest, type GameOptions } from "./types";
import "./style.css";

export function useManifest(base: string) {
  const [manifest, setManifest] = useState<AssetManifest>();
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    void loadManifest(base).then(data => { if (active) setManifest(data); }).catch(error => { if (active) setError(error instanceof Error ? error.message : "参考图读取失败"); });
    return () => { active = false; };
  }, [base]);
  return { manifest, error };
}

export function AssetImage({ manifest, reference, base, label, className = "" }: { manifest: AssetManifest; reference: string; base: string; label: string; className?: string }) {
  const image = imageResource(manifest, reference);
  if (!image?.resource) return <span className={`hm-image-empty ${className}`} aria-label={`${label}参考图尚未就绪`} />;
  const { resource, frame } = image;
  if (!frame) return <span className={`hm-image ${className}`}><img src={assetURL(base, resource.file)} alt={label} draggable={false} /></span>;
  const unit = 100 / frame.w;
  return <span className={`hm-image hm-sprite-image ${className}`} style={{ aspectRatio: `${frame.w}/${frame.h}` }}>
    <img src={assetURL(base, resource.file)} alt={label} draggable={false} style={{ width: `${resource.width * unit}%`, maxWidth: "none", left: `${-frame.x * unit}%`, top: `${-frame.y * 100 / frame.h}%` }} />
  </span>;
}

function MapChoices({ value, disabled, onChange, manifest, base }: { value: string; disabled: boolean; onChange: (id: string) => void; manifest?: AssetManifest; base: string }) {
  return <div className="hm-map-choices" role="group" aria-label="比赛地图">
    {Object.entries(mapNames).map(([id, name]) => {
      const map = manifest?.maps.find(candidate => candidate.id === id);
      return <button type="button" key={id} data-hotlinemiami-map={id} aria-pressed={value === id} disabled={disabled} onClick={() => onChange(id)}>
        {manifest && map ? <AssetImage manifest={manifest} reference={map.preview} base={base} label={`${name}俯视参考图`} className="hm-map-image" /> : <span className="hm-map-skeleton" />}
        <span>{name}</span>
      </button>;
    })}
  </div>;
}

export function Options({ value, onChange, disabled, capacity }: OptionsProps<GameOptions>) {
  const { manifest, error } = useManifest("");
  useEffect(() => {
    if (value.characters.length === capacity && value.weapons.length === capacity) return;
    const characters = Array.from({ length: capacity }, (_, seat) => value.characters[seat] ?? Object.keys(characterNames)[seat % Object.keys(characterNames).length]);
    const weapons = Array.from({ length: capacity }, (_, seat) => value.weapons[seat] ?? "pistol");
    onChange({ ...value, characters, weapons });
  }, [capacity, value, onChange]);
  return <div className="hm-options">
    <MapChoices value={value.map} disabled={disabled} onChange={map => onChange({ ...value, map })} manifest={manifest} base="" />
    <label className="hm-time-field">比赛时长<select value={value.roundSeconds} disabled={disabled} aria-label="比赛时长" onChange={event => onChange({ ...value, roundSeconds: Number(event.target.value) })}>
      <option value={180}>3 分钟</option><option value={300}>5 分钟</option>
    </select></label>
    {error && <p role="alert" className="hm-error">{error}</p>}
  </div>;
}

export function Preparation({ room, options, host, busy, assets, prepare }: PreparationProps<GameOptions>) {
  const { manifest, error } = useManifest(assets);
  const participating = room.you >= 0 && !room.spectating;
  return <section className="hm-preparation" aria-label="混战准备">
    <MapChoices value={options.map} disabled={busy || !host || !participating} onChange={map => prepare({ type: "map", map })} manifest={manifest} base={assets} />
    <label className="hm-time-field">比赛时长<select value={options.roundSeconds} disabled={busy || !host || !participating} aria-label="比赛时长" onChange={event => prepare({ type: "time", roundSeconds: Number(event.target.value) })}>
      <option value={180}>3 分钟</option><option value={300}>5 分钟</option>
    </select></label>
    <div className="hm-roster">
      {Array.from({ length: room.capacity }, (_, seat) => {
        const player = room.seats[seat];
        const editable = participating && (seat === room.you || host && !!player?.bot) && !!player && !busy;
        const character = options.characters[seat] ?? "jacket";
        const weapon = options.weapons[seat] ?? "pistol";
        const characterAsset = manifest?.characters.find(candidate => candidate.id === character);
        const weaponAsset = manifest?.weapons.find(candidate => candidate.id === weapon);
        return <div className="hm-seat-choices" key={seat}>
          <strong>{player?.name ?? "等待玩家"}{seat === room.you ? " · 你" : ""}</strong>
          <details className="hm-picker">
            <summary aria-label={`${player?.name ?? "空席"}的造型：${characterNames[character]}`} aria-disabled={!editable} onClick={event => { if (!editable) event.preventDefault(); }}>
              {manifest && characterAsset && <AssetImage manifest={manifest} reference={characterAsset.portrait} base={assets} label={characterNames[character]} className="hm-portrait" />}
              <span>{characterNames[character]}</span><span aria-hidden="true">⌄</span>
            </summary>
            <div className="hm-picker-grid" role="group" aria-label={`${player?.name ?? "空席"}的造型`}>
              {Object.entries(characterNames).map(([id, name]) => {
                const item = manifest?.characters.find(candidate => candidate.id === id);
                return <button type="button" key={id} data-hotlinemiami-character={id} data-seat={seat} disabled={!editable} aria-pressed={character === id} onClick={event => { prepare({ type: "character", seat, character: id }); event.currentTarget.closest("details")?.removeAttribute("open"); }}>
                  {manifest && item && <AssetImage manifest={manifest} reference={item.portrait} base={assets} label={name} className="hm-portrait" />}<span>{name}</span>
                </button>;
              })}
            </div>
          </details>
          <details className="hm-picker hm-weapon-picker">
            <summary aria-label={`${player?.name ?? "空席"}的武器：${weaponNames[weapon]}`} aria-disabled={!editable} onClick={event => { if (!editable) event.preventDefault(); }}>
              {manifest && weaponAsset && <AssetImage manifest={manifest} reference={weaponAsset.sprite} base={assets} label={weaponNames[weapon]} className="hm-weapon-image" />}
              <span>{weaponNames[weapon]}</span><span aria-hidden="true">⌄</span>
            </summary>
            <div className="hm-picker-grid hm-weapon-grid" role="group" aria-label={`${player?.name ?? "空席"}的武器`}>
              {Object.entries(weaponNames).filter(([id]) => id).map(([id, name]) => {
                const item = manifest?.weapons.find(candidate => candidate.id === id);
                return <button type="button" key={id} data-hotlinemiami-weapon={id} data-seat={seat} disabled={!editable} aria-pressed={weapon === id} onClick={event => { prepare({ type: "weapon", seat, weapon: id }); event.currentTarget.closest("details")?.removeAttribute("open"); }}>
                  {manifest && item && <AssetImage manifest={manifest} reference={item.sprite} base={assets} label={name} className="hm-weapon-image" />}<span>{name}</span>
                </button>;
              })}
            </div>
          </details>
        </div>;
      })}
    </div>
    {error && <p className="hm-error" role="alert">{error}</p>}
  </section>;
}
