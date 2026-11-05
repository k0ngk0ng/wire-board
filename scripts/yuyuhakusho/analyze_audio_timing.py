#!/usr/bin/env python3
"""Validate and project retained original sound timing, without executing core."""
import argparse
from collections import Counter
import gzip
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]

def sha(raw):return hashlib.sha256(raw).hexdigest()
def load(path):
 raw=path.read_bytes();return json.loads(gzip.decompress(raw)if path.suffix=='.gz'else raw)
def write(path,value):path.write_text(json.dumps(value,indent=2)+'\n')
def state(row):return row['state']

def analyze(capture,kind):
 manifest=load(capture/('audio-timing-'+kind+'-manifest.json'))
 path=ROOT/manifest['fixture_path'];raw=gzip.decompress(path.read_bytes())
 assert manifest['pass']and sha(raw)==manifest['fixture_json_sha256']and sha(path.read_bytes())==manifest['fixture_gzip_sha256']
 payload=json.loads(raw);markers=payload['markers'];calls=payload['scheduler_calls']
 phases=[]
 for call in calls:
  if call['kind']!='interrupt':
   raise ValueError('Catchup natural call needs explicit repeated-phase projection; do not flatten or infer it')
  before,after=call['before'],call['after'];span=markers[before['ordinal']:after['ordinal']+1]
  begins=[m for m in span if m['pc']==0x219de and m['sp']==call['entry_sp']]
  if begins:
   assert len(begins)==1
   end=[m for m in span if m['pc']==0x219e4 and m['sp']==call['entry_sp']]
   assert len(end)==1
   phases.append({'scenario':kind,'name':f'{kind}-{before["ordinal"]}-begin','kind':'begin-interrupt','before':state(before),'after':state(begins[0]),'drive':True})
   phases.append({'scenario':kind,'name':f'{kind}-{before["ordinal"]}-finish','kind':'finish-interrupt','before':state(end[0]),'after':state(after),'drive':False})
  else:
   assert after['pc']==0x219d6
   phases.append({'scenario':kind,'name':f'{kind}-{before["ordinal"]}-busy','kind':'begin-interrupt','before':state(before),'after':state(after),'drive':False})
 consumers=[];stack=[]
 for marker in markers:
  if marker['pc']==0x37c44:stack.append(marker)
  elif marker['pc']in[0x37c8e,0x37cb2]:
   assert stack and stack[-1]['sp']==marker['sp']
   before=stack.pop();consumers.append({'scenario':kind,'name':f'{kind}-consumer-{before["ordinal"]}','before':state(before),'after':state(marker)})
 assert not stack
 driver_ticks=[m for m in markers if m['pc']==0x375ae]
 consumed=[m for m in markers if m['pc']==0x37c44]
 dispatches=[m for m in markers if m['pc']==0x37cb4 and m['state']['selected']!=255]
 pauses=[m for m in driver_ticks if m['state']['audio_pause']!=0]
 summary={'scenario':kind,'video_steps':manifest['video_steps'],'scheduler_closed_calls':len(calls),'driver_ticks':len(driver_ticks),'queue_consumers':len(consumed),'non_ff_dispatches':len(dispatches),'requests':len(payload['request_calls']),'ticks_in_object_callback':sum(bool(m['callback_id'])for m in driver_ticks),'consumers_in_object_callback':sum(bool(m['callback_id'])for m in consumed),'consumer_callback_behaviors':dict(Counter(hex(m['callback_behavior'])for m in consumed if m['callback_id'])),'driver_pause_entries':dict(Counter(str(m['state']['audio_pause'])for m in driver_ticks)),'max_irq_depth':max(m['irq_depth']for m in markers),'max_backlog':max(m['state']['backlog']for m in markers),'pause_entry_consumers':sum(bool(m['state']['audio_pause'])for m in consumed),'driver_cycles_min':min(m['cpu_cycles']for m in driver_ticks),'driver_cycles_max':max(m['cpu_cycles']for m in driver_ticks),'phase_records':len(phases),'consumer_records':len(consumers),'source_manifest':str((capture/('audio-timing-'+kind+'-manifest.json')).relative_to(ROOT)),'source_manifest_sha256':sha((capture/('audio-timing-'+kind+'-manifest.json')).read_bytes())}
 return manifest,payload,phases,consumers,summary

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--baseline',type=Path,default=ROOT/'.local/md-yuyuhakusho/audio-timing-first')
 p.add_argument('--pause',type=Path,default=ROOT/'.local/md-yuyuhakusho/audio-timing-pause')
 p.add_argument('--private',type=Path,default=ROOT/'.local/md-yuyuhakusho/audio-timing-private-v2')
 p.add_argument('--out',type=Path,default=ROOT/'games/yuyuhakusho/engine/testdata')
 p.add_argument('--expect-frame-end-only',action='store_true')
 args=p.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 all_phases=[];all_consumers=[];summaries=[];sources=[]
 for capture,kind in [(args.baseline,'baseline'),(args.pause,'pause')]:
  capture=capture.resolve();assert capture.is_relative_to(ROOT)
  manifest,payload,phases,consumers,summary=analyze(capture,kind)
  all_phases+=phases;all_consumers+=consumers;summaries.append(summary)
  for suffix in ['.json.gz','-manifest.json','-original.json.gz']:
   path=capture/('audio-timing-'+kind+suffix);target=out/path.name
   target.write_bytes(path.read_bytes());sources.append({'source':str(path.relative_to(ROOT)),'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())})
 if args.expect_frame_end_only:
  failed=sum(s['consumers_in_object_callback']for s in summaries)
  result={'pass':False,'hypothesis':'All queue consumers occur only after object callbacks; a frame-end-only playback adapter suffices','counterexample_consumers_inside_callbacks':failed,'summaries':summaries,'producer_sha256':sha(Path(__file__).read_bytes())}
  write(out/'audio-timing-frame-end-red.json',result)
  assert failed==0,('frame-end-only audio hypothesis contradicted by actual consumer entries inside callbacks',failed)
 private=args.private.resolve();assert private.is_relative_to(ROOT)
 for suffix in ['.json.gz','-manifest.json']:
  path=private/('audio-timing-boundaries'+suffix);target=out/path.name;target.write_bytes(path.read_bytes());sources.append({'source':str(path.relative_to(ROOT)),'path':str(target.relative_to(ROOT)),'sha256':sha(target.read_bytes())})
 priorities=load(ROOT/'games/yuyuhakusho/engine/testdata/sound-queue-priorities.json')
 raw=(json.dumps({'schema':1,'priorities':priorities['priorities'],'phases':all_phases,'consumers':all_consumers},separators=(',',':'))+'\n').encode()
 path=out/'audio-timing-natural-phases.json.gz';path.write_bytes(gzip.compress(raw,mtime=0))
 manifest={'pass':True,'scope':'Original219ac begin/finish phase and37c44 actual consumer projections; normal driver synthesis is outside these pure helpers','summaries':summaries,'phase_records':len(all_phases),'consumer_records':len(all_consumers),'fixture_json_sha256':sha(raw),'fixture_gzip_sha256':sha(path.read_bytes()),'producer_sha256':sha(Path(__file__).read_bytes()),'original_sources':sources,'priority_bytes_sha256':sha(bytes(priorities['priorities'])),'limits':['No natural catchup/backlog nonzero was observed; private original phased streams prove that mechanism separately','Current219ac frame-end-only hypothesis is genuinely rejected; CPU cycles retain raw units, not PCM positions','Normal driver-side phase changes are separate external inputs to individual finish helper comparisons; no expected snapshots are injected into a claimed complete independent driver']}
 write(out/'audio-timing-natural-phases-manifest.json',manifest)
 print(json.dumps({'pass':True,'summaries':summaries,'phases':len(all_phases),'consumers':len(all_consumers)}),flush=True)

if __name__=='__main__':main()
