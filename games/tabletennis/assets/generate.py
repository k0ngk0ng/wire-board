#!/usr/bin/env python3
"""Original, deterministic struck-paddle/table effects; no external recordings."""
from pathlib import Path
import math,random,wave,struct,subprocess,hashlib,json
ROOT=Path(__file__).resolve().parents[3];HERE=Path(__file__).resolve().parent;TMP=ROOT/'.local/tabletennis/audio-source';TMP.mkdir(parents=True,exist_ok=True);OUT=HERE/'public/tabletennis/v1';OUT.mkdir(parents=True,exist_ok=True)
rows=[]
for index,(name,freq,duration,noise,amp) in enumerate([('hit',1500,.085,.18,.65),('bounce',2350,.050,.12,.43),('net',650,.075,.55,.25),('smash',1100,.110,.25,.82)]):
 rng=random.Random(7201+index);rate=44100;pcm=[]
 for i in range(round(rate*duration)):
  t=i/rate;envelope=math.exp(-t/(duration*.19))*min(1,t/.0008)
  value=(.65*math.sin(2*math.pi*freq*t)+.25*math.sin(2*math.pi*freq*.47*t)+noise*(rng.random()*2-1))*envelope*amp
  pcm.append(struct.pack('<h',round(max(-1,min(1,value))*32767)))
 source=TMP/(name+'.wav')
 with wave.open(str(source),'wb') as w:w.setnchannels(1);w.setsampwidth(2);w.setframerate(rate);w.writeframes(b''.join(pcm))
 output=OUT/(name+'.mp3');subprocess.run(['ffmpeg','-nostdin','-hide_banner','-loglevel','error','-y','-i',str(source),'-map_metadata','-1','-codec:a','libmp3lame','-b:a','64k',str(output)],check=True)
 rows.append({'urlPath':'/tabletennis/v1/'+output.name,'output':str(output.relative_to(ROOT)),'sha256':hashlib.sha256(output.read_bytes()).hexdigest(),'bytes':output.stat().st_size,'mime':'audio/mpeg','sourceSHA256':hashlib.sha256(source.read_bytes()).hexdigest()})
manifest={'assetsBaseURL':'https://cdn.ichenj.com/bga/v20261002-1','license':'CC0-1.0','source':'original procedural synthesis, no third-party samples','generator':str(Path(__file__).relative_to(ROOT)),'generatorSHA256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'encoder':subprocess.check_output(['ffmpeg','-version'],text=True).splitlines()[0],'entries':rows}
(HERE/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n');print('Generated',len(rows),'sound effects')
