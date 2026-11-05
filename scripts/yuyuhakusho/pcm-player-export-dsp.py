"""Export nonzero native private PCM and typed DSP states, no model expected."""
from pathlib import Path
import argparse,gzip,json,runpy
from reference import ROOT,sha
p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)
def typed(n,time=0,ahead=0):
 return {'Time':time,'DACValue':n['dac_value'],'Enabled':n['dac_enabled'],'Pan':(128 if n['dac_pan_masks'][0]else 0)+(64 if n['dac_pan_masks'][1]else 0),'LastChipOutput':n['previous_chip_output'],'Integrator':n['resampler']['integrator'],'Tail':n['resampler']['tail'],'InputAhead':ahead,'Pending':None}
cases=[]
for path in sorted(capture.glob('*.json.gz')):
 body=json.loads(gzip.decompress(path.read_bytes()));assert body['initial_dsp'];native=body['initial_dsp']['initial_state'];initial=typed(native);samples=[None if s is None else{'id':s['id'],'encoded_hex':s['encoded_hex'],'rate':s['rate']}for s in body['samples']]
 frames=[];cursor=0;nonzero=0
 for f in body['frames']:
  events=[e for e in body['events']if e['kind']in[1,2]and e['frame']==f['frame']]
  if events:cursor=f['frame']*896040+events[-1]['cycles_before']
  end=(f['frame']+1)*896040;ahead=max(0,cursor-end);raw=bytes.fromhex(f['pcm_hex']);nonzero+=sum(raw[i:i+2]!=b'\0\0'for i in range(0,len(raw),2))
  assert sha(raw)==f['pcm_sha256'];frames.append({'pcm_hex':f['pcm_hex'],'state':typed(f['native_dsp'],end,ahead)})
 assert nonzero>0,body['case']['name']
 cases.append({'name':body['case']['name'],'mode':body['case']['mode'],'samples':samples,'master_origin':native['master_origin'],'initial':initial,'frames':frames,'nonzero_samples':nonzero,'source_sha256':sha(path.read_bytes())})
raw=(json.dumps({'version':1,'cases':cases,'scope':'Native initialized DAC-enabled private player, continuous PCM and full typed DAC DSP state'},separators=(',',':'))+'\n').encode();out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress(raw,mtime=0))
print(json.dumps({'cases':len(cases),'frames':sum(len(c['frames'])for c in cases),'stereo_frames':sum(len(f['pcm_hex'])//8 for c in cases for f in c['frames']),'nonzero_samples':sum(c['nonzero_samples']for c in cases),'raw_bytes':len(raw),'gzip_bytes':out.stat().st_size,'sha256':sha(out.read_bytes())}))
