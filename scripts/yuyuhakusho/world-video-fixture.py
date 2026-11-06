"""Project original ordered VDP observations into portable test-only operations.

No original PC or expected state is used by production. A handler-entry marker
means the external CPU has already acknowledged; the v1 recording does not
directly observe the acknowledge call or CPU pins.
"""
import argparse,gzip,hashlib,json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
LINE=3420

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main(capture,out):
 assert capture.is_relative_to(ROOT) and out.is_relative_to(ROOT)
 report=json.loads((capture/'capture.json').read_text())
 assert report['pass'] and report['checked_frames']==2400 and report['all_reference_equal'] and report['all_pcm_equal']
 p=capture/'events.json.gz';j=json.loads(gzip.decompress(p.read_bytes()))
 assert sha(p)==report['artifacts'][0]['sha256']
 events=[dict(zip(j['fields'],r))for r in j['events']]
 cases=[];case=None;position=-1;edge=False;last_frame=None
 checks=reads=acks=0
 def observe(e):
  nonlocal checks
  case['Operations'].append({'Kind':'observe','Want':{
   'Counter':e['hcounter'],'Reload':e['reg10'],
   'HorizontalEnabled':bool(e['reg0']&16),'VerticalEnabled':bool(e['reg1']&32),'DisplayEnabled':bool(e['reg1']&64),
   'HorizontalPending':bool(e['hint_pending']),'VerticalPending':bool(e['vint_pending']),
   'VCounter':e['vcounter'],'LineCycle':e['line_cycle'],'Status':e['status_irq']}})
  checks+=1
 def vertical():
  nonlocal edge
  if not edge:case['Operations'].append({'Kind':'vertical'});edge=True
 def move(target):
  nonlocal position
  if target>0:vertical()
  while position<target:
   case['Operations'].append({'Kind':'line'});position+=1
 for e in events:
  if e['kind']==0:
   assert e['vcounter']==223 and e['line_cycle']==896040
   if last_frame is None or e['frame']!=last_frame+1:
    case={'Name':'original-steps-'+str(e['frame']),'Initial':{
     'Phase':0,'Counter':e['hcounter'],'Reload':e['reg10'],
     'HorizontalEnabled':bool(e['reg0']&16),'VerticalEnabled':bool(e['reg1']&32),'DisplayEnabled':bool(e['reg1']&64),
     'HorizontalPending':bool(e['hint_pending']),'VerticalPending':bool(e['vint_pending'])},'Frames':0,'Operations':[]}
    cases.append(case)
   else:assert position==262
   observe(e);case['Operations'].append({'Kind':'begin'});position=0;edge=False
   last_frame=e['frame'];case['Frames']+=1;continue
  if e['kind']==9:
   move(262);observe(e);continue
  assert e['frame']==last_frame
  move(e['line_cycle']//LINE)
  if position==0 and not edge and e['kind']in[1,4,5] and e['cpu_cycle']>=788:vertical()
  if e['kind']==4:case['Operations'].append({'Kind':'ack'});acks+=1
  observe(e)
  if e['kind']==2:
   assert e['width']in[2,4]
   words=[e['value']>>16,e['value']&65535]if e['width']==4 else[e['value']]
   for word in words:
    if word&0xc000==0x8000:
     reg=(word>>8)&31;value=word&255
     if reg in[0,1,10,12]:case['Operations'].append({'Kind':'register','Register':reg,'Value':value})
  elif e['kind']==3:
   assert e['width']==2 and not e['address']&1
   case['Operations'].append({'Kind':'status','Cycle':e['cpu_cycle'],'InstructionCycles':e['instruction_cycles'],'Value':e['value']&0x88});reads+=1
  elif e['kind']==6 and not e['address']&1:
   case['Operations'].append({'Kind':'hvc','Cycle':e['cpu_cycle'],'Value':e['value']if e['width']==1 else e['value']>>8});reads+=1
  elif e['kind']==8:raise AssertionError('Z80 status read is outside the declared profile')
 assert len(cases)==2 and sum(c['Frames']for c in cases)==15 and checks>=7700 and reads>=2000
 raw=(json.dumps({'Cases':cases},separators=(',',':'))+'\n').encode()
 out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress(raw,mtime=0))
 manifest={'pass':True,'scope':'Given ordered external scanline/VINT/register operations and handler-entry-inferred acknowledgements; no CPU acceptance or directly observed CPU pin claim',
  'windows':[{'name':c['Name'],'frames':c['Frames'],'operations':len(c['Operations'])}for c in cases],
  'scalar_states':checks,'reads':reads,'handler_entry_acknowledgements':acks,
  'source':{'path':str(p.relative_to(ROOT)),'sha256':sha(p)},
  'capture':{'path':str((capture/'capture.json').relative_to(ROOT)),'sha256':sha(capture/'capture.json')},
  'producer':{'path':str(Path(__file__).relative_to(ROOT)),'sha256':sha(Path(__file__))},
  'fixture':{'path':str(out.relative_to(ROOT)),'sha256':sha(out),'json_sha256':hashlib.sha256(raw).hexdigest()},
  'original_pc_in_fixture':False,'per_frame_reseed':False,'direct_ack_observed':False,'native_cpu_pin_observed':False}
 mp=ROOT/'games/yuyuhakusho/verification/world-video/fixture.json';mp.write_text(json.dumps(manifest,indent=2)+'\n')
 print(json.dumps({k:manifest[k]for k in['pass','scalar_states','reads','handler_entry_acknowledgements']}))

if __name__=='__main__':
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--capture',type=Path,default=ROOT/'games/yuyuhakusho/verification/world-video/native')
 p.add_argument('--out',type=Path,default=ROOT/'games/yuyuhakusho/audio/videoclock/testdata/natural.json.gz')
 a=p.parse_args();main(a.capture.resolve(),a.out.resolve())
