import { useEffect, useState } from 'react';
import type { OptionsProps, PreparationProps } from '../../../web/src/game-api/module';
import { assetURL, loadManifest } from './assets';
import { characters, maps, mapMode, modeName, normalizeOptions } from './content';
import type { AssetManifest, GameOptions } from './types';
import './style.css';

function useManifest(base: string) {
  const [manifest, setManifest] = useState<AssetManifest>();
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    void loadManifest(base).then(value => { if (active) setManifest(value); }).catch(error => { if (active) setError(error instanceof Error ? error.message : '地图预览读取失败'); });
    return () => { active = false; };
  }, [base]);
  return { manifest, error };
}

function MapChoices({ selected, disabled, change, base, manifest }: { selected: string; disabled: boolean; change: (id: string) => void; base: string; manifest?: AssetManifest }) {
  return <div className="cs16-map-grid" role="group" aria-label="经典地图">
    {maps.map(map => <button type="button" key={map.id} data-cs16-map={map.id} disabled={disabled} aria-pressed={selected === map.id} onClick={() => change(map.id)}>
      {manifest?.maps[map.id]?.preview ? <img src={assetURL(base, manifest.maps[map.id].preview.file)} alt={`${map.name}原地图俯视图`} draggable={false} /> : <span className="cs16-map-placeholder" />}
      <strong>{map.name}</strong><span>{modeName(map.mode)}</span>
    </button>)}
  </div>;
}

export function Options({ value, onChange, disabled, capacity }: OptionsProps<GameOptions>) {
  const { manifest, error } = useManifest('');
  useEffect(() => {
    if (value.teams.length === capacity && value.characters.length === capacity) return;
    onChange(normalizeOptions(value, capacity));
  }, [capacity, value, onChange]);
  return <div className="cs16-options">
    <MapChoices selected={value.map} disabled={disabled} change={map => onChange({ ...value, map })} base="" manifest={manifest} />
    <Settings value={value} disabled={disabled} change={changes => onChange({ ...value, ...changes })} />
    {error && <p className="cs16-error" role="alert">{error}</p>}
  </div>;
}

function Settings({ value, disabled, change }: { value: GameOptions; disabled: boolean; change: (value: Partial<GameOptions>) => void }) {
  return <div className="cs16-settings">
    {mapMode(value.map) !== 'elimination' && <label>每回合<select aria-label="每回合时长" value={value.roundSeconds} disabled={disabled} onChange={event => change({ roundSeconds: Number(event.target.value) })}><option value={60}>1 分钟</option><option value={90}>1 分 30 秒</option><option value={120}>2 分钟</option><option value={180}>3 分钟</option><option value={300}>5 分钟</option></select></label>}
    <label>比赛局数<select aria-label="比赛局数" value={value.maxRounds} disabled={disabled} onChange={event => change({ maxRounds: Number(event.target.value) })}>{[2, 4, 6, 10, 16, 24, 30].map(rounds => <option key={rounds} value={rounds}>{rounds} 局</option>)}</select></label>
    <label className="cs16-friendly-fire"><input type="checkbox" aria-label="允许队友伤害" data-cs16-setting="friendly-fire" checked={value.friendlyFire ?? false} disabled={disabled} onChange={event => change({ friendlyFire: event.target.checked })} />允许队友伤害</label>
  </div>;
}

export function Preparation({ room, options, host, busy, assets, prepare }: PreparationProps<GameOptions>) {
  const { manifest, error } = useManifest(assets);
  const value = normalizeOptions(options, room.capacity);
  const participating = room.you >= 0 && !room.spectating;
  return <section className="cs16-preparation" aria-label="反恐精英准备">
    <MapChoices selected={value.map} disabled={busy || !host || !participating} change={map => prepare({ type: 'map', map })} base={assets} manifest={manifest} />
    <Settings value={value} disabled={busy || !host || !participating} change={changes => prepare({ type: 'settings', ...changes })} />
    <div className="cs16-team-grid">{['t', 'ct'].map(team => <section key={team} className={`cs16-team cs16-team-${team}`}>
      <h3>{team === 'ct' ? '反恐精英' : '恐怖分子'}<span>{value.teams.filter(value => value === team).length} / 5</span></h3>
      {Array.from({ length: room.capacity }, (_, seat) => seat).filter(seat => value.teams[seat] === team).map(seat => {
        const player = room.seats[seat];
        const editable = participating && !!player && !busy && (seat === room.you || host && !!player.bot);
        return <div className="cs16-seat" key={seat}>
          <strong>{player?.name ?? '等待玩家'}{seat === room.you ? ' · 你' : ''}</strong>
          <select aria-label={`${player?.name ?? '空席'}的造型`} value={value.characters[seat]} disabled={!editable} onChange={event => prepare({ type: 'character', seat, character: event.target.value })}>{characters.filter(character => character.team === team).map(character => <option key={character.id} value={character.id}>{character.name}</option>)}</select>
          <button type="button" disabled={!editable || value.teams.filter(value => value !== team).length >= 5} aria-label={`${player?.name ?? '空席'}切换队伍`} onClick={() => prepare({ type: 'team', seat, team: team === 'ct' ? 't' : 'ct' })}>换队</button>
        </div>;
      })}
    </section>)}</div>
    {error && <p className="cs16-error" role="alert">{error}</p>}
  </section>;
}
