#!/usr/bin/env python3
"""Generate a self-created dual-tone horn, not an NFS original-bank sample."""
import argparse,hashlib,json,math
from pathlib import Path
import shutil,struct,subprocess

def sha(data):return hashlib.sha256(data).hexdigest()
def encode(value):return json.dumps(value,ensure_ascii=False,separators=(',',':'),allow_nan=False).encode()+b'\n'

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--out',type=Path,required=True)
 args=parser.parse_args();repository=Path(__file__).resolve().parents[2];output=args.out.resolve()
 if not output.is_relative_to(repository):parser.error('output must stay inside this repository')
 if output.exists():parser.error('output must be new; existing assets are never overwritten')
 stage=output.with_name(output.name+'.building')
 if stage.exists():parser.error('staging output already exists')
 stage.mkdir(parents=True)
 try:
  folder=stage/'audio/horn';folder.mkdir(parents=True)
  rate=22050;frames=4410
  samples=[round(32767*.7*(.55*math.sin(2*math.pi*350*i/rate)+.45*math.sin(2*math.pi*440*i/rate)))for i in range(frames)]
  pcm=struct.pack('<'+str(frames)+'h',*samples)
  target=folder/'horn.flac'
  subprocess.run(['ffmpeg','-nostdin','-v','error','-f','s16le','-ar',str(rate),'-ac','1','-i','pipe:0','-map_metadata','-1','-c:a','flac','-compression_level','12','-sample_fmt','s16','-metadata_header_padding','0','-fflags','+bitexact','-flags:a','+bitexact',str(target)],input=pcm,capture_output=True,check=True)
  decoded=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(target),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
  if decoded!=pcm:raise ValueError('FLAC roundtrip changed the synthesized PCM')
  data=target.read_bytes()
  metadata={'version':1,'path':'audio/horn/horn.flac','origin':'original-synthesis','description':'自制双音喇叭，非极品飞车原声','license':'CC0-1.0','generator':'scripts/racing/nfs-horn.py','frequenciesHz':[350,440],'waveform':'0.7 * (0.55*sin(350Hz) + 0.45*sin(440Hz)); signed 16-bit nearest integer','sampleRate':rate,'channels':1,'bits':16,'frames':frames,'duration':frames/rate,'loopStartFrame':0,'loopEndFrameExclusive':frames,'bytes':len(data),'sha256':sha(data),'pcmSha256':sha(pcm)}
  (folder/'horn.json').write_bytes(encode(metadata))
  assets=[]
  for path in sorted(p for p in folder.iterdir()if p.is_file()):
   data=path.read_bytes();assets.append({'path':path.relative_to(stage).as_posix(),'bytes':len(data),'sha256':sha(data),'mime':'audio/flac'if path.suffix=='.flac'else'application/json'})
  manifest={'version':1,'origin':'original-synthesis','assets':assets,'totalBytes':sum(a['bytes']for a in assets)}
  (stage/'horn-manifest.json').write_bytes(encode(manifest))
  stage.rename(output)
  print(json.dumps({'output':str(output.relative_to(repository)),'assets':len(assets),'bytes':manifest['totalBytes'],'pcmSha256':sha(pcm)}))
 except BaseException:
  shutil.rmtree(stage)
  raise

if __name__=='__main__':main()
