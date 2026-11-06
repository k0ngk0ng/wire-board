#!/usr/bin/env python3
"""Original, quiet mechanical chimes. Encode FLAC and verify decoded PCM."""
import hashlib
import json
import math
from pathlib import Path
import random
import shutil
import struct
import subprocess
import wave
ROOT=Path(__file__).resolve().parents[3]
HERE=Path(__file__).resolve().parent
RATE=22050

def sound(notes,duration,ambient=False):
 out=[];rng=random.Random(194);phase=0
 for i in range(round(duration*RATE)):
  t=i/RATE;v=0.
  for start,freq,length,amp in notes:
   elapsed=t-start
   if 0<=elapsed<length:
    envelope=min(1,elapsed/.012)*math.exp(-elapsed*(2 if ambient else 6)/length)
    v+=amp*envelope*(math.sin(2*math.pi*freq*elapsed)+.22*math.sin(2*math.pi*freq*2.005*elapsed))
  if ambient:v+=(rng.random()-.5)*.004*math.sin(math.pi*t/duration)**2
  v*=min(1,t/.02,(duration-t)/.04)
  out.append(struct.pack('<h',round(max(-.8,min(.8,v))*32767)))
 return b''.join(out)

def main():
 ffmpeg=shutil.which('ffmpeg')
 if not ffmpeg:raise RuntimeError('ffmpeg is required for lossless encoding')
 out=HERE/'public/escape/v1/audio';out.mkdir(parents=True,exist_ok=True)
 scratch=ROOT/'.local/escape/audio';scratch.mkdir(parents=True,exist_ok=True)
 cues={
 'clue':([(0,587,.55,.11),(.12,880,.7,.07)],.9),
 'pickup':([(0,440,.25,.08),(.07,659,.35,.07)],.5),
 'solved':([(0,293,.9,.09),(.11,440,.8,.07),(.24,587,1.1,.09),(.36,880,1.1,.04)],1.6),
 'warning':([(0,294,.35,.1),(.4,294,.35,.08),(.8,349,.4,.06)],1.3),
 'error':([(0,185,.4,.08),(.05,196,.45,.055)],.55),
 'victory':([(i*.19,f,1.7,.07) for i,f in enumerate([293.66,369.99,440,587.33,739.99])],3.2),
 'ambient':([(i*2, f,6,.021) for i,f in enumerate([73.416,110,146.832,164.814,110,73.416])],18),
 }
 entries=[]
 for name,(notes,duration) in cues.items():
  pcm=sound(notes,duration,name=='ambient');wav=scratch/(name+'.wav');target=out/(name+'.flac')
  with wave.open(str(wav),'wb') as w:w.setnchannels(1);w.setsampwidth(2);w.setframerate(RATE);w.writeframes(pcm)
  subprocess.run([ffmpeg,'-v','error','-y','-i',str(wav),'-c:a','flac',str(target)],check=True)
  decoded=subprocess.check_output([ffmpeg,'-v','error','-i',str(target),'-f','s16le','-acodec','pcm_s16le','-'])
  assert pcm==decoded
  entries.append(dict(path='/escape/v1/audio/'+target.name,sha256=hashlib.sha256(target.read_bytes()).hexdigest(),bytes=target.stat().st_size,rate=RATE,channels=1,bitDepth=16,frames=len(pcm)//2,pcmSHA256=hashlib.sha256(pcm).hexdigest(),decodedPCM_SHA256=hashlib.sha256(decoded).hexdigest()))
  wav.unlink()
 (HERE/'audio-manifest.json').write_text(json.dumps(dict(origin='original synthesized composition',assets=entries),indent=2)+'\n')
 print(json.dumps(dict(files=len(entries),bytes=sum(x['bytes'] for x in entries))))
if __name__=='__main__':main()
