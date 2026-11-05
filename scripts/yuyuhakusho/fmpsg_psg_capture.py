"""Original native PSG state/edge oracle, with an observer no-change control.

Research only. No native source is copied or modified. The fine observer
advances the unchanged exported PSG API one internal clock at a time; its
complete state, blip buffer and PCM must equal the original batched API calls.
Each observed edge is also injected into a separate original blip buffer.
"""
from pathlib import Path
import argparse,ctypes as C,gzip,json,random,struct,wave
from reference import ROOT,ReferenceSession,sha
from inspect_dac_dsp_state import Config,Snd,Blip,symbols

F=896040
ORIGINAL=ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
OBSERVED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'

class PSG(C.Structure):
 _fields_=[(key,C.c_int)for key in ['clocks','latch','zeroFreqInc','noiseShiftValue','noiseShiftWidth','noiseBitMask']]+[('regs',C.c_int*8)]+[(key,C.c_int*4)for key in ['freqInc','freqCounter','polarity']]+[(key,(C.c_int*2)*4)for key in ['chanDelta','chanOut','chanAmp']]

def operations(writes,frames):
 result=[];boundary=F
 for cycle,data in writes:
  while cycle>=boundary:result.append({'Kind':'frame','Cycle':boundary,'Data':0});boundary+=F
  result.append({'Kind':'write','Cycle':cycle,'Data':data})
 while boundary<=frames*F:result.append({'Kind':'frame','Cycle':boundary,'Data':0});boundary+=F
 return result

def cases():
 latch=[];t=0
 for data in [0x05,0x80,0,0x90,0x91,0x9e,0x9f,0xaf,0x3f,0xb0,0xc0,0,0xd0,0xe3,0xf0,0xe7]:
  latch.append((t,data));t+=[0,1,239,240,241][len(latch)%5]
 for channel in range(4):
  for level in range(16):latch.append((t,0x90+channel*0x20+level));t+=337
 for channel in range(3):
  for divider in [0,1,2,15,16,255,256,1023,0]:
   latch.extend([(t,0x80+channel*0x20+(divider&15)),(t,divider>>4)]);t+=523
 for control in range(8):
  latch.extend([(t,0xe0|control),(t,0xf0)]);t+=61441
 randomizer=random.Random(0x76489);fuzz=[];t=0
 for _ in range(1400):
  t+=randomizer.choice([0,1,239,240,241,479,480,997,5003]);fuzz.append((t,randomizer.randrange(256)))
 ahead=[{'Kind':'write','Cycle':0,'Data':0x90},{'Kind':'advance','Cycle':F-1,'Data':0},
        {'Kind':'write','Cycle':F,'Data':0x98},{'Kind':'write','Cycle':F+195,'Data':0x92},
        {'Kind':'frame','Cycle':F,'Data':0},{'Kind':'write','Cycle':F+195,'Data':0x97},
        {'Kind':'write','Cycle':F+240,'Data':0x93},{'Kind':'write','Cycle':2*F+1008,'Data':0x94},
        {'Kind':'frame','Cycle':2*F,'Data':0},{'Kind':'write','Cycle':2*F+1008,'Data':0x95},
        {'Kind':'frame','Cycle':3*F,'Data':0}]
 return [('latch-tone-volume-noise',operations(latch,2)),('raw-byte-fuzz',operations(fuzz,3)),
         ('white-noise-complete',operations([(0,0xc1),(0,0),(0,0xe7),(0,0xf0)],40)),
         ('periodic-noise-complete',operations([(0,0xc0),(0,0),(0,0xe3),(0,0xf0)],2)),
         ('frame-ahead-boundaries',ahead)]

def typed(psg,base,cursor):
 return {'Cursor':cursor,'NextTick':base+psg.clocks,'Latch':psg.latch,
  'ToneDivider':[psg.regs[i*2]for i in range(3)],'Level':[psg.regs[i*2+1]for i in range(4)],
  'NoiseControl':psg.regs[6]&7,'NoiseShift':psg.noiseShiftValue,
  'NextEdge':[base+x for x in psg.freqCounter],'High':[x>0 for x in psg.polarity],
  'PendingDelta':[list(x)for x in psg.chanDelta]}

def blip_state(pointer):
 b=Blip.from_address(pointer)
 raw=b''.join(C.string_at(x,(b.size+18)*4)for x in b.buffer)
 return {'factor':b.factor,'offset':b.offset,'size':b.size,'integrator':list(b.integrator),'buffer_sha256':sha(raw)}

def write_gzip(path,value):
 path.parent.mkdir(parents=True,exist_ok=True)
 raw=(json.dumps(value,separators=(',',':'))+'\n').encode();path.write_bytes(gzip.compress(raw,mtime=0))

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);a=parser.parse_args()
 out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 controls={};final=[];identities={};configuration=None
 for mode,core,fine in [('original',ORIGINAL,False),('fine',ORIGINAL,True),('observed',OBSERVED,False)]:
  lookup=symbols(core)
  with ReferenceSession(ROOT,out/mode,core=core)as ref:
   identities[mode]=ref.identity
   image=C.cast(ref.lib.psg_reset,C.c_void_p).value-lookup['_psg_reset'][0]
   psg=PSG.from_address(image+lookup['_psg'][0]);assert C.sizeof(PSG)==200
   cfg=Config.in_dll(ref.lib,'config');assert cfg.hq_psg==1 and cfg.psg_preamp==150
   snd=Snd.in_dll(ref.lib,'snd');pointer=snd.blips[0]
   native_write=ref.api('psg_write',None,C.c_uint,C.c_uint)
   configure=ref.api('psg_config',None,C.c_uint,C.c_uint,C.c_uint)
   end_psg=ref.api('psg_end_frame',None,C.c_uint)
   end_blip=ref.api('blip_end_frame',None,C.c_void_p,C.c_uint)
   samples=ref.api('blip_samples_avail',C.c_int,C.c_void_p)
   read=ref.api('blip_read_samples',C.c_int,C.c_void_p,C.c_void_p,C.c_int)
   clear=ref.api('blip_clear',None,C.c_void_p)
   add=ref.api('blip_add_delta',None,C.c_void_p,C.c_uint,C.c_int,C.c_int)
   new=ref.api('blip_new',C.c_void_p,C.c_int);delete=ref.api('blip_delete',None,C.c_void_p)
   rate=ref.api('blip_set_rates',None,C.c_void_p,C.c_double,C.c_double)
   for name,ops in cases():
    ref.api('psg_init',None,C.c_int)(1);ref.api('psg_reset',None)();configure(0,150,255);clear(pointer)
    assert psg.zeroFreqInc==240 and psg.noiseShiftWidth==15 and psg.noiseBitMask==9
    assert [list(x)for x in psg.chanAmp]==[[150,150]]*4
    configuration={'model':'integrated','clock_period_master':240,'noise_feedback_bit':15,'white_feedback_taps':[0,3],'zero_divider':1,'hq_psg':cfg.hq_psg,'preamp':cfg.psg_preamp,'panning':255,'sample_rate':snd.sample_rate,'psg_scalar_bytes':C.sizeof(PSG)}
    verifier=new(Blip.from_address(pointer).size);assert verifier
    rate(verifier,53693175,44100);clear(verifier);assert blip_state(pointer)==blip_state(verifier)
    base=cursor=0;rows=[];parts=[];initial=typed(psg,base,cursor)
    ram=sha(ref.ram_cpu());video=sha(ref.image);seen={};shifts=0;period=None
    try:
     for index,op in enumerate(ops):
      edges=[];target=op['Cycle']
      if fine:
       while base+psg.clocks<target:
        clock=psg.clocks;at=base+clock
        pending=[list(x)for x in psg.chanDelta];old_high=[x>0 for x in psg.polarity]
        old_noise=psg.noiseShiftValue;old_out=[list(x)for x in psg.chanOut]
        assert all(x>=clock for x in psg.freqCounter)
        configure(clock+1,150,255)
        for channel in range(4):
         if pending[channel]!=[0,0]:edges.append({'Cycle':at,'Delta':pending[channel]})
         if channel<3:
          sign=int(psg.polarity[channel]>0)-int(old_high[channel])
         else:
          sign=(psg.noiseShiftValue&1)-(old_noise&1)
         delta=[sign*x for x in old_out[channel]]
         if delta!=[0,0]:edges.append({'Cycle':at,'Delta':delta})
        if not old_high[3] and psg.polarity[3]>0:
         shifts+=1
         if name=='white-noise-complete' and period is None:
          key=psg.noiseShiftValue
          if key in seen:period=shifts-seen[key]
          else:seen[key]=shifts
       for edge in edges:add(verifier,edge['Cycle']-base,*edge['Delta'])
      local=target-base
      if op['Kind']=='write':native_write(local,op['Data']);cursor=target
      elif op['Kind']=='advance':configure(local,150,255);cursor=target
      else:
       assert target==base+F
       end_psg(F);end_blip(pointer,F)
       if fine:end_blip(verifier,F)
       count=samples(pointer);buf=(C.c_int16*(count*2))();assert read(pointer,buf,count)==count
       pcm=C.string_at(buf,count*4);parts.append(pcm)
       if fine:
        other=(C.c_int16*(count*2))();assert samples(verifier)==count and read(verifier,other,count)==count
        assert pcm==C.string_at(other,count*4),(name,index,'edge injection PCM differs')
       base=target;cursor=max(cursor,target)
      assert sha(ref.ram_cpu())==ram and sha(ref.image)==video
      state=typed(psg,base,cursor);buffer=blip_state(pointer)
      if fine:assert buffer==blip_state(verifier),(name,index,'edge injection buffer differs')
      row={**op,'State':state,'Edges':edges,'Blip':buffer}
      if mode=='original':rows.append(row)
      else:
       expected=controls[name]['rows'][index]
       assert state==expected['State'] and buffer==expected['Blip'],(mode,name,index,'observer changes native result',state,expected['State'])
       rows.append(row)
     pcm=b''.join(parts)
     if mode=='original':controls[name]={'initial':initial,'rows':rows,'pcm':pcm}
     else:
      assert pcm==controls[name]['pcm'],(mode,name,'full native PCM differs')
      if fine:
       final.append({'Name':name,'Initial':initial,'Operations':[{k:v for k,v in row.items()if k!='Blip'}for row in rows]})
       if name=='white-noise-complete':assert period and shifts>period,(period,shifts)
     write_gzip(out/(name+'-'+mode+'.json.gz'),{'initial':initial,'rows':rows,'pcm_sha256':sha(pcm),'stereo_frames':len(pcm)//4,'white_noise_period_shifts':period,'noise_shifts':shifts})
     with wave.open(str(out/(name+'-'+mode+'.wav')),'wb')as wav:wav.setnchannels(2);wav.setsampwidth(2);wav.setframerate(44100);wav.writeframes(pcm)
     print(json.dumps({'mode':mode,'case':name,'operations':len(rows),'edges':sum(len(x['Edges'])for x in rows),'stereo_frames':len(pcm)//4,'white_period':period,'shifts':shifts}),flush=True)
    finally:delete(verifier)
 fixture={'Cases':final,'Configuration':configuration};write_gzip(out/'native.json.gz',fixture)
 result={'pass':True,'scope':'PSG typed state and individual nonzero amplitude edges from unchanged native APIs; batched/fine/original/observed blip buffers and PCM equal','configuration':configuration,'identities':identities,'cases':[{'name':c['Name'],'operations':len(c['Operations']),'edges':sum(len(o['Edges'])for o in c['Operations'])}for c in final],'producer_sha256':sha(Path(__file__).read_bytes()),'fixture_sha256':sha((out/'native.json.gz').read_bytes()),'no_runtime_firmware':True,'independent_event_clock':False,'full_world_audio':False}
 (out/'capture.json').write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps(result))

if __name__=='__main__':main()
