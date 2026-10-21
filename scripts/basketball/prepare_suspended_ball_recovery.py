"""Pin actual roll-off/end states and unchanged original ball-dispatch results."""
import argparse,gzip,hashlib,json
from probe_rest_stall import ROOT,ROM,PRG,encode,run,SoundMemory


def main():
 parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
 records=[];runtimes=[]
 for label,path,video in [('home0','.local/basketball/rea/campaign-home-stall/solo/first-entry',3384),('newyork1','.local/basketball/rea/ball-rest-stall/first-entry',6300),('court7','.local/basketball/rea/court-seven-stall/first-entry',13989)]:
  directory=ROOT/path;runtimes.append(dict(name=label,state=json.loads((directory/'stalled.json').read_text())))
  history=json.loads((directory/'history.json').read_text());row=next(r for r in history if r['Video']==video)
  for kind,before,after in [('trigger',row['Before'],row['After']),('stalled',json.loads((directory/'stalled.json').read_text()),None)]:
   actual,sounds=run(SoundMemory(PRG,encode(before)))
   if after is not None:assert actual==after['playing']['world']['Preparation']['Actions']['ball']
   state=(after or before)['playing']['world']['Preparation']['Actions']
   state=dict(state,ball=actual)
   records.append(dict(name=label+'-'+kind,court=before['config']['match']['Court'],beforeVideo=before['videoFrames'],beforeBall=before['playing']['world']['Preparation']['Actions']['ball'],originalState=state,sounds=sounds))
 payload=(json.dumps(records,separators=(',',':'))+'\n').encode();data=gzip.compress(payload,mtime=0)
 target=ROOT/'games/basketball/engine/testdata/suspended-ball-recovery.json.gz'
 if args.check:assert target.read_bytes()==data
 else:target.write_bytes(data)
 meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(data).hexdigest(),records=len(records),source='three unchanged original complete E149 rolloff calls and three stalled calls from retained Go states; prior whole-machine source agreement documented separately',originalInstructionsUnmodified=True,repairImplementedInOracle=False)
 if not args.check:target.with_name('suspended-ball-recovery-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 runtimeData=gzip.compress((json.dumps(runtimes,separators=(',',':'))+'\n').encode(),mtime=0)
 runtimeTarget=target.with_name('suspended-ball-runtime.json.gz')
 if args.check:assert runtimeTarget.read_bytes()==runtimeData
 else:runtimeTarget.write_bytes(runtimeData)
 meta['runtimeSHA256']=hashlib.sha256(runtimeData).hexdigest()
 if not args.check:target.with_name('suspended-ball-recovery-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
