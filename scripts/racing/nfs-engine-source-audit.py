#!/usr/bin/env python3
"""Inspect verified original engine loops; these are not browser recordings.

Reports PCM seam/adjacent-difference/RMS evidence without claiming perception.
Optional WAVs repeat original PCM with no inserted silence or fades, as source
references only. Actual native browser captures are retained separately.
"""
import argparse,hashlib,json,math
from pathlib import Path
import struct,subprocess,wave

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/racing/assets/public/racing/v3'
def sha(b):return hashlib.sha256(b).hexdigest()
def rms(values):return math.sqrt(sum(v*v for v in values)/max(1,len(values)))
def quantile(values,p):
 values=sorted(values)
 return values[min(len(values)-1,math.floor((len(values)-1)*p))]

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--out',type=Path,default=ROOT/'.local/racing-nfs-reference/audio-continuity-source')
 parser.add_argument('--examples',action='store_true')
 args=parser.parse_args();out=args.out.resolve()
 if not out.is_relative_to(ROOT):parser.error('output must stay inside this repository')
 out.mkdir(parents=True,exist_ok=True)
 rows=[];examples=[]
 for model in ['p911','fm50','ldia','mcf1','ccor']:
  metadata=json.loads((BASE/f'audio/{model}/engine.json').read_text())
  for sample in metadata['samples']:
   source=BASE/sample['path'];compressed=source.read_bytes()
   assert sha(compressed)==sample['sha256'] and len(compressed)==sample['bytes']
   decoded=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(source),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
   assert sha(decoded)==sample['pcmSha256'] and len(decoded)==sample['frames']*2
   values=struct.unpack('<'+str(len(decoded)//2)+'h',decoded);steps=[abs(b-a)for a,b in zip(values,values[1:])]
   size=max(1,round(sample['sampleRate']*.01));windows=[rms(values[i:i+size])for i in range(0,len(values)-size+1,size)]
   seam=abs(values[-1]-values[0]);p99=quantile(steps,.99)
   row={'model':model,'patch':sample['patch'],'path':sample['path'],'flacSha256':sha(compressed),'pcmSha256':sha(decoded),
    'sampleRate':sample['sampleRate'],'channels':sample['channels'],'frames':sample['frames'],'duration':len(values)/sample['sampleRate'],
    'rms':rms(values),'peak':max(map(abs,values)),'dc':sum(values)/len(values),'seamJump':seam,
    'adjacentDifferenceP95':quantile(steps,.95),'adjacentDifferenceP99':p99,'seamToP99':seam/max(1,p99),
    'seamRankAmongAdjacentDifferences':sum(v<=seam for v in steps)/len(steps),
    'window10msRms':{'min':min(windows),'p05':quantile(windows,.05),'median':quantile(windows,.5),'p95':quantile(windows,.95),'max':max(windows)},
    'sourceLoop':sample['source']}
   rows.append(row)
   if args.examples and(model,sample['patch'])in [('p911',36),('ldia',42),('mcf1',25)]:
    path=out/f'{model}-{sample["patch"]}-source-repeat.wav'
    with wave.open(str(path),'wb')as target:target.setnchannels(1);target.setsampwidth(2);target.setframerate(sample['sampleRate']);target.writeframes(decoded*5)
    examples.append({'path':path.name,'sha256':sha(path.read_bytes()),'method':'five exact repetitions of original source PCM; no gap/fade; NOT a browser/output recording'})
 report={'method':'FFmpeg lossless decoding of the published SHA-verified original loop intervals; no target execution',
  'limitations':['PCM seam magnitudes do not establish audibility or a native playback gap.','Example WAVs are source references, not a replacement for actual native browser capture.','Original fixed-point pitch/log-volume mixer and actual Windows output have not been measured.'],
  'loops':rows,'examples':examples}
 (out/'source-loops.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 text=['# 原始发动机循环审计','','这是源 PCM 的客观测量，非浏览器或扬声器录音；seam jump 本身不证明听感缺陷。','','| 车型／patch | 周期 s | seam jump | 相邻差 P99 | seam/P99 |','| --- | ---: | ---: | ---: | ---: |']
 for row in rows:text.append(f"| {row['model']}/{row['patch']} | {row['duration']:.6f} | {row['seamJump']} | {row['adjacentDifferenceP99']} | {row['seamToP99']:.3f} |")
 text.extend(['','如生成示例 WAV，它们只把原 PCM 重复五次，不插静音或淡化；不能把它们称为修复后的网页声音。'])
 (out/'SOURCE.md').write_text('\n'.join(text)+'\n')
 print(json.dumps({'loops':len(rows),'examples':examples,'out':str(out.relative_to(ROOT))},ensure_ascii=False))

if __name__=='__main__':main()
