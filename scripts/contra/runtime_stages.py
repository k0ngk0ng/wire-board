"""Generate scene resource sets from normalized source data; never open a ROM.

Failures: missed alternate/base Boss/common player pages, counting disabled CHR
rows as live, missing late-lair/Interlude animation pages, cross-stage BGM and
missing percussion or completion callbacks. Audio IDs describe on-event assets;
no media download or animation/rule execution is performed here.
"""
import hashlib,json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];ENG=ROOT/'games/contra/engine';OUT=ROOT/'games/contra/assets/runtime-stages.json'
ROM='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def load(p):return json.loads((ROOT/p).read_text())
def numbers(s):return [int(x,0)for x in re.findall(r'0x[0-9a-fA-F]+|\b\d+\b',s)]
def table(text,name):
 m=re.search(r'var '+name+r'\s*=.*?\{([^}]+)\}',text,re.S);assert m,name;return numbers(m[1])
def main():
 audio=load('games/contra/assets/audio-source.json');defs={d['id']:d for d in load('games/contra/engine/audio_control_data.json')['definitions']};media={d['id']:d for d in audio['sounds']};animation=load('docs/research/nes-contra/evidence/levels-chr-animation.json')
 assert audio['romSHA256']==animation['romSHA256']==ROM
 scene=(ENG/'campaign_scene_data.go').read_text();m=re.search(r'var campaignScenePresets =.*?\{\n(.*?)\n\}',scene,re.S);assert m
 presets={int(k):numbers(v)for k,v in re.findall(r'(\d+):\s*\{([^}]+)\}',m[1])}
 interlude_ids=table(scene,'campaignSceneInterludeCHR');particles=table(scene,'campaignSceneParticleCHR')
 common=set(table((ENG/'weapons_tables.go').read_text(),'weaponSounds'))|set(table((ENG/'enemies_collision_tables.go').read_text(),'collisionHitSounds'))|{11,28,47,56,57,61,114,116}
 # These are emitter scopes, not enemy AI or sound timing. Shared destruction,
 # pickup, hit/shot, mortar and pause requests are conservatively stage-wide.
 extra=[{51,119},{52,62,119},{13,119},{52,62,119},{48,54,117,98},{25,43,117,98},{51,119},{117,123,102}]
 def resources(pages,music=(),sfx=()):
  roots=set(music)|set(sfx);seen=set();pending=list(roots)
  while pending:
   n=pending.pop()
   if n in seen:continue
   seen.add(n);assert n in defs or n in media,('unknown audio',n)
   for slot in defs.get(n,{}).get('slots',[]):
    pending.extend(e['id']for key in ['prefix','loop','end']for e in slot[key])
  return {'chrPages':sorted(set(pages)),'music':sorted(set(music)),'sfx':sorted((seen&media.keys())-set(music))}
 stages=[];reasons=[]
 for i in range(8):
  data=load(f'games/contra/engine/leveldata/stage-{i+1}.json');assert data['romSHA256']==ROM;info=data['stages'][0];byid={p['id']:p['pages']for p in data['chrPresets']}
  pages={p for row in data['chrPresets']for p in row['pages']};rule=animation['stageRules'][i]
  if not rule.get('disabled'):
   pages.update(animation['rows'][rule['row']]['pages'])
   if i==7:pages.update(animation['rows'][8]['pages']);pages.update(animation['secondaryLair']['pages'])
  # The shared nonzero-transition guard precedes the stage switch. Retain this
  # one-page conservative dependency; do not include disabled stage5/6 rows.
  pages.add(animation['transition']['page'])
  battle=resources(pages,[audio['stageMusic'][i]],common|extra[i]);boss=None
  if info.get('baseBoss'):boss=resources(set(byid[info['baseBoss']['chrPreset']])|{animation['transition']['page']},[audio['baseBossMusic']['id']],common|extra[i])
  interlude=None
  if i<7:
   ip=set(presets[interlude_ids[int(i>=5)]])
   if i>=5:ip.update(particles)
   interlude=resources(ip,(),[16,32]if i>=5 else[16,19])
  intro=resources(presets[10],(),[20,22,24]);cont=resources(presets[10],(),[110])
  # Any stage can fail at its game-over display. The sole success terminal is
  # stage8's retained battlefield: Session does not run original credits.
  final=resources(set(presets[10])|(pages if i==7 else set()),(),[110,102,123]if i==7 else[110])
  stages.append({'level':i,'intro':intro,'battle':battle,'baseBoss':boss,'interlude':interlude,'continue':cont,'final':final})
  reasons.append({'level':i,'presetIDs':sorted(byid),'animationRows':([]if rule.get('disabled')else[rule['row']])+([8]if i==7 else[]),'sharedTransitionPage':animation['transition']['page'],'commonAudioRoots':sorted(common),'stageAudioRoots':sorted(extra[i]),'battleIncludesBaseBossForWithinStageTransition':bool(info.get('baseBoss'))})
 sources=[ENG/'chr_animation.go',ENG/'terrain_scroll.go',ENG/'campaign_initialize.go',ENG/'campaign.go',ENG/'campaign_scene_data.go',ENG/'campaign_scene_intro.go',ENG/'campaign_scene_interlude.go',ENG/'campaign_reset_display.go',ENG/'session_terminal.go',ENG/'weapons.go',ENG/'weapons_tables.go',ENG/'player.go',ENG/'player_indoor.go',ENG/'player_animation.go',ENG/'player_lifecycle.go',ENG/'audio_control_data.json',ROOT/'games/contra/assets/audio-source.json',ROOT/'docs/research/nes-contra/evidence/levels-chr-animation.json']
 sources+=sorted(p for p in ENG.glob('enemies*.go')if not p.name.endswith('_test.go'));sources+=sorted((ENG/'leveldata').glob('stage-*.json'))
 manifest={'schema':1,'romSHA256':ROM,'sources':{str(p.relative_to(ROOT)):sha(p)for p in sorted(set(sources))},'stages':stages}
 OUT.write_text(json.dumps(manifest,indent=2)+'\n')
 report={'scope':'Conservative per-stage reachable graphics and on-event audio declarations; no assets are downloaded. Battle includes same-stage alternate/baseBoss dependencies.','sourceSHA256':sha(OUT),'reasons':reasons,'allReachableCHRPages':sorted({p for s in stages for key,v in s.items()if key!='level'and v for p in v['chrPages']}),'pageCounts':[{ 'level':s['level'],**{key:len(v['chrPages'])if v else 0 for key,v in s.items()if key!='level'}}for s in stages],'reconnectRule':'Also ensure the snapshot currently Applied Patterns are available; do not guess from new level alone. In particular stage5->6 can still present preset18 while new stage uses19.','audioRule':'Music and SFX lists are descriptors only. Fetch media for actual Audio events/active voices, including native percussion/end-callback closure.','finalRule':'Failed terminal can retain continue preset10 at any level. Level8 success preserves its last battlefield; no credits BGM/preset is introduced.'}
 (ROOT/'docs/research/nes-contra/evidence/runtime-stages-sources.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'manifest':str(OUT.relative_to(ROOT)),'bytes':OUT.stat().st_size,'pageCounts':report['pageCounts'],'unionPages':len(report['allReachableCHRPages'])}))
if __name__=='__main__':main()
