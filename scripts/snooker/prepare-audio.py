#!/usr/bin/env python3
"""Prepare attributed CC0 foley and original two-dimensional game's UI tones."""
from pathlib import Path
import json,hashlib,subprocess,math,wave,struct
root=Path(__file__).resolve().parents[2];source=root/'games/snooker/assets/source/audio';out=root/'games/snooker/assets/public/snooker/v1/audio';source.mkdir(parents=True,exist_ok=True);out.mkdir(parents=True,exist_ok=True)
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
originals=json.loads((root/'games/snooker/assets/source-manifest.json').read_text())
for item in originals['entries']:assert sha(root/item['path'])==item['sha256'],'audited source differs: '+item['path']
assert sha(root/originals['licenseFile'])==originals['licenseSHA256'],'source licence differs'
manifest_path=root/'games/snooker/assets/manifest.json';manifest=json.loads(manifest_path.read_text());manifest['entries']=[e for e in manifest['entries'] if e['type']!='audio']
mapping={'cue':'impactWood_light_000.ogg','pocket':'impactWood_medium_000.ogg'}
gain={}
for kind,family in [('ball','impactGeneric_light'),('cushion','impactSoft_medium')]:
 for tier,volume in [('soft',.45),('medium',.72),('hard',1.0)]:
  for variant in range(3):
   name=f'{kind}-{tier}-{variant}';mapping[name]=f'{family}_{variant:03d}.ogg';gain[name]=volume
for name,notes in {'foul':[330,262],'win':[294,370,440]}.items():
 sr=44100;samples=[]
 for hz in notes:
  for i in range(int(sr*.14)):
   t=i/sr;env=min(t/.009,1)*max(0,1-t/.14)**1.6;samples.append(int(32767*.17*math.sin(2*math.pi*hz*t)*env))
 with wave.open(str(source/(name+'.wav')),'w') as w:w.setnchannels(1);w.setsampwidth(2);w.setframerate(sr);w.writeframes(b''.join(struct.pack('<h',v) for v in samples))
for name in [*mapping,'foul','win']:
 original=source/mapping.get(name,name+'.wav');output=out/(name+'.mp3')
 subprocess.run(['ffmpeg','-nostdin','-y','-v','error','-i',str(original),'-af','loudnorm=I=-23:TP=-7:LRA=7,volume='+str(gain.get(name,1.0)),'-ac','1','-ar','44100','-b:a','96k',str(output)],check=True)
 manifest['entries'].append({'type':'audio','name':name,'author':'Kenney' if name in mapping else 'wire-board contributors','license':'CC0-1.0' if name in mapping else 'Project original','sourceURL':'https://kenney.nl/assets/impact-sounds' if name in mapping else None,'source':str(original.relative_to(root)),'output':str(output.relative_to(root)),'urlPath':'/snooker/v1/audio/'+name+'.mp3','sourceSHA256':sha(original),'sha256':sha(output),'generator':'scripts/snooker/prepare-audio.py','gain':gain.get(name,1.0)})
manifest_path.write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n');(out.parent/'manifest.json').write_bytes(manifest_path.read_bytes());print('Prepared',sum(e['type']=='audio' for e in manifest['entries']),'media-channel MP3 cues.')
