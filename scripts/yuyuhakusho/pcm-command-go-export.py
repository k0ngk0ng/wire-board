"""Portable original-only command vectors. Never run the prediction model."""
from pathlib import Path
import argparse,gzip,json,hashlib
ROOT=Path(__file__).resolve().parents[2];F=896040
sha=lambda b:hashlib.sha256(b).hexdigest()
p=argparse.ArgumentParser();p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();capture=a.capture.resolve();out=a.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT)
def dsp(n,time=0,ahead=0):return {'Time':time,'DACValue':n['dac_value'],'Enabled':n['dac_enabled'],'Pan':(128 if n['dac_pan_masks'][0]else 0)+(64 if n['dac_pan_masks'][1]else 0),'LastChipOutput':n['previous_chip_output'],'Integrator':n['resampler']['integrator'],'Tail':n['resampler']['tail'],'InputAhead':ahead,'Pending':None}
cases=[]
for path in sorted(capture.glob('*.json.gz')):
 body=json.loads(gzip.decompress(path.read_bytes()));resources=body['resources'];sources=list({s['id']:s for s in resources.values()}.values());payloads={s['id']:s['encoded_hex']for s in sources};descriptors=[]
 for key,s in resources.items():descriptors.append({'ID':key,'PayloadID':s['id'],'NativeRate':s['rate'],'Flags':s['flags'],'RoutingOrPan':s['ordinary_routing'],'Protocol':2 if key.endswith('music')else 1})
 normalized=[];single_header=None
 for e in body['events']:
  if e['kind']not in[1,2,3]:continue
  v={'Cycle':e['frame']*F+e['cycles_before'],'Kind':'dac'if e['kind']==1 else'pan'if e['address']==0x1b6 else'enable','Value':e['data']if e['kind']in[1,2]else 0,'PayloadID':'','Offset':0,'Bus':None,'Frame':e['frame']}
  if e['kind']==3:
   address=e['bank']+(e['address']&32767)
   if e['pc']in[0xa5,0xa8,0xc3,0xc6]:
    choices=[s for s in sources if s['source']['header_address']<=address<s['source']['header_address']+4];assert len(choices)==1;(s,)=choices;kind='metadata';offset=address-s['source']['header_address'];single_header=s
   elif e['pc']==0x149:
    assert single_header;s=single_header;kind='dummy';offset=(address-s['source']['header_address'])&65535
   elif e['pc']in[0xb7,0xb9,0xbb,0xbd,0x168,0x16a]:
    assert single_header;s=single_header;kind='bank';offset=address-s['source']['sample_address']
   else:
    choices=[s for s in sources if s['source']['sample_address']<=address<s['source']['sample_address']+len(bytes.fromhex(s['encoded_hex']))+4];assert len(choices)==1;(s,)=choices;kind='bank';offset=address-s['source']['sample_address']
   v.update(Kind=kind,PayloadID=s['id'],Offset=offset,Bus={'At':e['z80_before'],'Z80After':e['z80_after'],'M68KDelay':e['m68k_after']-e['m68k_before'],'SetWaitFlag':False})
  normalized.append(v)
 inputs=[];transactions=[]
 for tr in body['transactions']:
  action=tr['input'];kind=0 if action['kind']=='hold'else 2 if action['protocol']=='music'else 1;dest={'single':0,'a':1,'b':2}.get(action.get('target'),0);grant=(action['begin']//F)*F+tr['after_grant_cycles'];mode=0
  for e in body['events']:
   if e['frame']*F+e['cycles_before']>grant:break
   mode=bytes.fromhex(e['state'])[124]
  inputs.append({'Kind':kind,'Begin':action['begin'],'End':action['end'],'DescriptorID':str(action['code'])+action['protocol']if kind else'','Destination':dest,'StartDual':kind==1 and dest!=0 and not mode&1,'Pan':action.get('pan',0)if kind==2 else 0})
  transactions.append({'Begin':action['begin'],'End':action['end'],'Grant':grant,'Resume':(tr['release']//F)*F+tr['after_release_cycles']})
 frames=[];cursor=0
 for row in body['frames']:
  f=row['frame'];events=[{k:v for k,v in e.items()if k!='Frame'}for e in normalized if e['Frame']==f];chip=[e for e in events if e['Kind']in['dac','pan','enable']]
  if chip:cursor=chip[-1]['Cycle']
  state=bytes.fromhex(row['player_state_hex']);status={'ModeFlags':state[124],'StartMarker':state[125],'RateRequest':state[126],'SlotBusy':[state[6],state[38]],'SlotFlags':[state[7],state[39]],'Busy':row['busy'],'Held':row['held']}
  frames.append({'Events':events,'PCMHex':row['pcm_hex'],'DSP':dsp(row['native_dsp'],(f+1)*F,max(0,cursor-(f+1)*F)),'Status':status})
 cases.append({'Name':body['case']['name'],'Payloads':payloads,'Descriptors':descriptors,'Inputs':inputs,'Transactions':transactions,'Frames':frames,'MasterOrigin':body['initial_dsp']['initial_state']['master_origin'],'InitialDSP':dsp(body['initial_dsp']['initial_state']),'SourceSHA256':sha(path.read_bytes())})
raw=(json.dumps({'Version':1,'Cases':cases},separators=(',',':'))+'\n').encode();out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress(raw,mtime=0));print(json.dumps({'cases':len(cases),'frames':sum(len(c['Frames'])for c in cases),'events':sum(len(f['Events'])for c in cases for f in c['Frames']),'raw_bytes':len(raw),'gzip_bytes':out.stat().st_size,'sha256':sha(out.read_bytes())}))
