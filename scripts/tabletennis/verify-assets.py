#!/usr/bin/env python3
"""Verify only table-tennis media, provenance and optional build/decoder copies."""
from pathlib import Path
import argparse,hashlib,json,subprocess
ROOT=Path(__file__).resolve().parents[2];parser=argparse.ArgumentParser();parser.add_argument('--dist',action='store_true');parser.add_argument('--decode-audio',action='store_true');args=parser.parse_args();manifest=json.loads((ROOT/'games/tabletennis/assets/manifest.json').read_text())
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
assert manifest['license']=='CC0-1.0';assert 'CC0 1.0' in (ROOT/'games/tabletennis/assets/LICENSE.txt').read_text();assert sha(ROOT/manifest['generator'])==manifest['generatorSHA256'];assert len(manifest['entries'])==4
results=[]
for row in manifest['entries']:
 p=ROOT/row['output'];assert p.resolve().is_relative_to((ROOT/'games/tabletennis/assets/public').resolve());assert sha(p)==row['sha256'] and p.stat().st_size==row['bytes'];assert row['mime']=='audio/mpeg' and row['urlPath'].startswith('/tabletennis/v1/')
 if args.dist:assert sha(ROOT/'web/dist'/row['urlPath'].lstrip('/'))==row['sha256']
 item={'urlPath':row['urlPath'],'sha256':row['sha256'],'bytes':row['bytes']}
 if args.decode_audio:
  probe=json.loads(subprocess.check_output(['ffprobe','-v','error','-show_entries','format=duration:stream=codec_name,sample_rate,channels','-of','json',str(p)],text=True));assert probe['streams'][0]['codec_name']=='mp3';assert .01<float(probe['format']['duration'])<.3;subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(p),'-f','null','-'],check=True);item['probe']=probe
 results.append(item)
out=ROOT/'.local/tabletennis/assets';out.mkdir(parents=True,exist_ok=True);(out/'verified.json').write_text(json.dumps({'passed':True,'dist':args.dist,'decoded':args.decode_audio,'entries':results},indent=2)+'\n');print('PASS tabletennis:',len(results),'original audio assets')
