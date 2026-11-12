#!/usr/bin/env python3
"""Make a marked playback copy without changing any original engine FLAC.

The source PCM remains separately identified. A short cyclic head/tail overlap
and a near-zero phase origin reduce discontinuities; there is no silence fade,
normalization, resampling, or lossy codec. Outputs are a reviewable file delta.
"""
import argparse,copy,hashlib,json,math
from pathlib import Path
import shutil,struct,subprocess

ROOT=Path(__file__).resolve().parents[2]
def sha(data):return hashlib.sha256(data).hexdigest()
def encode(value):return json.dumps(value,ensure_ascii=False,separators=(',',':'),allow_nan=False).encode()+b'\n'
def decode(path):return subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(path),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
def rms(values):return math.sqrt(sum(v*v for v in values)/len(values))

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--source',type=Path,default=ROOT/'games/racing/assets/public/racing/v3')
 parser.add_argument('--out',type=Path,required=True)
 parser.add_argument('--model',choices=['p911','fm50','ldia','mcf1','ccor'],required=True)
 parser.add_argument('--patch',type=int,required=True)
 parser.add_argument('--seam-ms',type=float,default=5)
 args=parser.parse_args();source=args.source.resolve();output=args.out.resolve()
 if not output.is_relative_to(ROOT):parser.error('output must stay in this repository')
 if output.exists():parser.error('output must be new; original assets are never overwritten')
 if not math.isfinite(args.seam_ms)or not 0<args.seam_ms<=10:parser.error('overlap must be at most10ms')
 stage=output.with_name(output.name+'.building')
 if stage.exists():parser.error('staging path already exists')
 stage.mkdir(parents=True)
 try:
  metadata_path=f'audio/{args.model}/engine.json';metadata=json.loads((source/metadata_path).read_text());sample=next(s for s in metadata['samples']if s['patch']==args.patch)
  original=(source/sample['path']).read_bytes();pcm=decode(source/sample['path'])
  if sha(original)!=sample['sha256']or sha(pcm)!=sample['pcmSha256']:raise ValueError('source hash mismatch')
  if(sample['sampleRate'],sample['channels'],sample['bits'])!=(22050,1,16):raise ValueError('unsupported original format')
  values=list(struct.unpack('<'+str(len(pcm)//2)+'h',pcm));count=round(sample['sampleRate']*args.seam_ms/1000)
  if not 2<=count<len(values)*.02:raise ValueError('overlap would alter too much of the loop')
  overlap=[]
  for i in range(count):
   t=i/(count-1);w=t*t*(3-2*t);overlap.append(round(values[-count+i]*(1-w)+values[i]*w))
  result=values[count:-count]+overlap
  rotation=min(range(len(result)),key=lambda i:abs(result[i]));result=result[rotation:]+result[:rotation]
  processed=struct.pack('<'+str(len(result))+'h',*result)
  folder=stage/f'audio/{args.model}';folder.mkdir(parents=True)
  name=f'engine-{args.patch}-playback';path=f'audio/{args.model}/{name}.flac';info_path=f'audio/{args.model}/{name}.json';target=stage/path
  subprocess.run(['ffmpeg','-nostdin','-v','error','-f','s16le','-ar','22050','-ac','1','-i','pipe:0','-map_metadata','-1','-c:a','flac','-compression_level','12','-sample_fmt','s16','-metadata_header_padding','0','-fflags','+bitexact','-flags:a','+bitexact',str(target)],input=processed,capture_output=True,check=True)
  if decode(target)!=processed:raise ValueError('processed FLAC failed lossless PCM roundtrip')
  data=target.read_bytes();playback={'origin':'seam-overlap-playback','path':path,'metadata':info_path,'sampleRate':22050,'channels':1,'bits':16,
    'frames':len(result),'duration':len(result)/22050,'loopStart':0,'loopEnd':len(result),'bytes':len(data),'sha256':sha(data),'pcmSha256':sha(processed),
    'sourceSha256':sample['sha256'],'sourcePcmSha256':sample['pcmSha256']}
  info={'version':1,'model':args.model,'patch':args.patch,**playback,
    'description':'播放派生循环；原始极品飞车PCM另行保留，不把本文件称为原始字节',
    'recipe':{'generator':'scripts/racing/nfs-engine-playback.py','algorithm':'cyclic-smoothstep-overlap-then-near-zero-phase-origin','revision':1,
      'requestedOverlapMS':args.seam_ms,'overlapFrames':count,'rotationFrames':rotation,'normalize':False,'fadeToSilence':False,'resample':False},
    'source':{'path':sample['path'],'frames':len(values),'flacSha256':sample['sha256'],'pcmSha256':sample['pcmSha256']},
    'metrics':{'sourceRms':rms(values),'playbackRms':rms(result),'sourceSeamJump':abs(values[-1]-values[0]),'playbackSeamJump':abs(result[-1]-result[0]),'firstSample':result[0]}}
  (stage/info_path).write_bytes(encode(info));sample['playback']=playback;(stage/metadata_path).write_bytes(encode(metadata))
  assets=[]
  for p in sorted(x for x in stage.rglob('*')if x.is_file()):
   b=p.read_bytes();assets.append({'path':p.relative_to(stage).as_posix(),'bytes':len(b),'sha256':sha(b),'mime':'audio/flac'if p.suffix=='.flac'else'application/json'})
  audio_manifest=json.loads((source/'audio-manifest.json').read_text());entries={a['path']:a for a in audio_manifest['assets']}
  for a in assets:entries[a['path']]=a
  audio_manifest['assets']=[entries[k]for k in sorted(entries)];audio_manifest['totalBytes']=sum(a['bytes']for a in audio_manifest['assets'])
  audio_manifest['encoding']='FLAC lossless original loop intervals plus separately identified seam-overlap playback copies'
  audio_manifest['models'][args.model]['playbackLoops']=[{'patch':args.patch,'path':path,'pcmSha256':sha(processed)}]
  (stage/'audio-manifest.json').write_bytes(encode(audio_manifest));b=(stage/'audio-manifest.json').read_bytes();assets.append({'path':'audio-manifest.json','bytes':len(b),'sha256':sha(b),'mime':'application/json'})
  (stage/'audio-manifest.delta.json').write_bytes(encode({'version':1,'description':'Only these files change; the global v3 manifest must merge this delta once.','assets':assets,'totalDeltaFileBytes':sum(a['bytes']for a in assets)}))
  stage.rename(output);print(json.dumps({'output':str(output.relative_to(ROOT)),'sourceUnchanged':True,'playback':playback,'metrics':info['metrics']},ensure_ascii=False))
 except BaseException:
  shutil.rmtree(stage)
  raise

if __name__=='__main__':main()
