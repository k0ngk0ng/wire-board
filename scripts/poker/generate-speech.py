#!/usr/bin/env python3
"""Generate licensed Mandarin cues and original cartoony bomb sounds locally.
Model: hexgrad/Kokoro-82M-v1.1-zh (Apache-2.0), pinned revision below.
No commercial game's recordings are used. Run with the workspace TTS venv.
"""
from pathlib import Path
import os,json,hashlib,subprocess,importlib.metadata
ROOT=Path(__file__).resolve().parents[2]
LOCAL=ROOT/'.local/poker-fixes';MODEL=LOCAL/'tts-model';RAW=ROOT/'assets/poker/source/audio/v3';OUT=ROOT/'web/public/poker/v3/audio'
for key,folder in {'HF_HOME':LOCAL/'huggingface','TORCH_HOME':LOCAL/'torch','XDG_CACHE_HOME':LOCAL/'cache','TMPDIR':ROOT/'.local/tmp'}.items():
 folder.mkdir(parents=True,exist_ok=True);os.environ.setdefault(key,str(folder))
import numpy as np
import soundfile as sf
import torch
from scipy.signal import butter,sosfilt
from kokoro import KModel,KPipeline
SR=24000;REV='01e7505bd6a7a2ac4975463114c3a7650a9f7218'
RAW.mkdir(parents=True,exist_ok=True);OUT.mkdir(parents=True,exist_ok=True)
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
torch.set_num_threads(4);torch.manual_seed(0)
model=KModel(repo_id='hexgrad/Kokoro-82M-v1.1-zh',config=str(MODEL/'config.json'),model=str(MODEL/'kokoro-v1_1-zh.pth')).eval()
pipeline=KPipeline(lang_code='z',repo_id='hexgrad/Kokoro-82M-v1.1-zh',model=model)
lines=json.loads((ROOT/'assets/poker/voice-lines.json').read_text())
manifest=json.loads((ROOT/'assets/poker/manifest.json').read_text());manifest['entries']=[e for e in manifest['entries'] if e.get('type') not in ['speech','sfx']]
# Deterministic filtered noise plus a descending low tone; no sampled explosion.
rng=np.random.default_rng(1711);t=np.arange(int(SR*.62))/SR
noise=sosfilt(butter(3,1100,fs=SR,output='sos'),rng.normal(size=len(t)))
boom=.38*noise*np.exp(-9*t)+.26*np.sin(2*np.pi*(88*t-45*t*t))*np.exp(-7*t)
boom*=np.minimum(t/.006,1);boom=boom.astype(np.float32)
sf.write(RAW/'bomb-effect.wav',boom,SR)
for name,text in lines.items():
 raw=RAW/(name+'.wav')
 if not raw.exists():
  result=next(pipeline(text+'。',voice=str(MODEL/'voices/zf_001.pt'),speed=1.08));voice=result.audio.numpy()
  active=np.flatnonzero(np.abs(voice)>max(.002,float(np.max(np.abs(voice)))*.012))
  if not len(active):raise RuntimeError('Silent speech: '+name)
  voice=voice[max(0,active[0]-800):min(len(voice),active[-1]+2400)]
  voice=voice/max(float(np.max(np.abs(voice))),.001)*.76
  offset=int(SR*(.14 if name in ['bomb','rocket'] else .025))
  mix=np.zeros(max(offset+len(voice),len(boom) if name in ['bomb','rocket'] else 0),dtype=np.float32);mix[offset:offset+len(voice)]+=voice
  if name in ['bomb','rocket']:mix[:len(boom)]+=boom*(1.15 if name=='rocket' else 1)
  peak=float(np.max(np.abs(mix)));mix*=min(1,.94/max(peak,.001));sf.write(raw,mix,SR)
 data,sr=sf.read(raw);duration=len(data)/sr;peak=float(np.max(np.abs(data)))
 if not .15<duration<5 or peak<.1:raise RuntimeError('Invalid speech duration or level: '+name)
 dest=OUT/(name+'.mp3');subprocess.run(['ffmpeg','-hide_banner','-loglevel','error','-y','-i',str(raw),'-codec:a','libmp3lame','-b:a','96k','-ar','44100',str(dest)],check=True)
 manifest['entries'].append({'type':'speech','name':name,'text':text,'source':raw.relative_to(ROOT).as_posix(),'sourceSHA256':sha(raw),'output':dest.relative_to(ROOT).as_posix(),'sha256':sha(dest),'durationSeconds':round(duration,3),'model':'hexgrad/Kokoro-82M-v1.1-zh','modelRevision':REV,'voice':'zf_001','license':'Model: Apache-2.0; generated speech and DSP mix','sourceURL':'https://huggingface.co/hexgrad/Kokoro-82M-v1.1-zh','generator':'scripts/poker/generate-speech.py'})
 print(name,text,round(duration,2),flush=True)
manifest['speechGeneration']={'modelRevision':REV,'voiceSHA256':sha(MODEL/'voices/zf_001.pt'),'modelSHA256':sha(MODEL/'kokoro-v1_1-zh.pth'),'kokoroVersion':importlib.metadata.version('kokoro'),'misakiVersion':importlib.metadata.version('misaki'),'seed':0,'sampleRate':SR}
(ROOT/'assets/poker/manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n');(ROOT/'web/public/poker/manifest.json').write_bytes((ROOT/'assets/poker/manifest.json').read_bytes())
(ROOT/'assets/poker/source/licenses/kokoro-README.md').write_bytes((MODEL/'README.md').read_bytes())
print('Generated',len(lines),'spoken cues.',flush=True)
