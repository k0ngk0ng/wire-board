"""Private raw-YM boundary oracle; no CPU/ROM execution or production core."""
from pathlib import Path
import argparse,base64,ctypes as C,gzip,hashlib,json,random,subprocess
from inspect_dac_dsp_state import YM,OPN,ST,symbols
from extract_audio_assets import AudioSession

ROOT=Path(__file__).resolve().parents[2]
PINNED=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib'
ORIGINAL=ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'
ORDER=[0,2,1,3]
def sha(raw):return hashlib.sha256(raw).hexdigest()
def rate(value):return 0 if value==0 else (value-32)//2
def normal(ym):
 opn=ym.OPN;st=opn.ST;dtab=C.addressof(ym)+YM.OPN.offset+OPN.ST.offset+ST.dt_tab.offset
 channels=[]
 for ch in ym.CH:
  ops=[]
  for i in ORDER:
   s=ch.SLOT[i]
   ops.append(dict(Detune=(s.DT-dtab)//(32*4),Multiple=0 if s.mul==1 else s.mul//2,Level=s.tl//8,KeyScale=3-s.KSR,Attack=rate(s.ar),Decay=rate(s.d1r),SustainRate=rate(s.d2r),Release=(s.rr-34)//4,SustainLevel=15 if s.sl==992 else s.sl//32,SSG=s.ssg,AMEnabled=bool(s.AMmask),Phase=s.phase,Envelope=s.volume,EnvelopePhase=s.state,Negated=bool(s.ssgn),Key=bool(s.key),LatchedKeyScale=s.ksr))
  channels.append(dict(Frequency=ch.block_fnum,Algorithm=ch.ALGO,Feedback=10-ch.FB,Pan=(128 if opn.pan[len(channels)*2]else 0)|(64 if opn.pan[len(channels)*2+1]else 0),AMDepth={8:0,3:1,1:2,0:3}[ch.ams],PMDepth=ch.pms//32,FeedbackHistory=list(ch.op1_out),Memory=ch.mem_value,Operators=ops))
 period=opn.lfo_timer_overflow
 return dict(Channels=channels,Address=st.address,Status=st.status,Mode=st.mode,FrequencyLatch=st.fn_h,SpecialLatch=opn.SL3.fn_h,SpecialFrequency=list(opn.SL3.block_fnum),TimerAValue=st.TA,TimerACounter=st.TAC,TimerBValue=st.TB,TimerBCounter=st.TBC,CSM=opn.SL3.key_csm,EnvelopeCount=opn.eg_cnt,EnvelopeDivider=opn.eg_timer,LFOCycle=opn.lfo_cnt,LFODivider=opn.lfo_timer,LFOEnabled=bool(period),LFOSpeed=[108,77,71,67,62,44,8,5].index(period)if period else 0,DACValue=(ym.dacout//64)+128,DACEnabled=bool(ym.dacen))

def reg(bank,address,value):return [[bank*2,address],[bank*2+1,value]]
def tone(channel,algorithm=7,feedback=0,seed=0,ssg=0):
 bank,c=divmod(channel,3);writes=[]
 for logical,offset in enumerate([0,8,4,12]):
  vals={0x30:((seed+logical)%8)<<4|((logical+seed)%16),0x40:((logical*7+seed)%28),0x50:(31|((logical%4)<<6)),0x60:(12|128),0x70:5,0x80:0x2c,0x90:ssg}
  for a,v in vals.items():writes+=reg(bank,a+offset+c,v)
 writes+=reg(bank,0xb0+c,algorithm|(feedback<<3))+reg(bank,0xb4+c,0xc0)
 writes+=reg(bank,0xa4+c,0x24|(seed%4))+reg(bank,0xa0+c,0x69+seed)
 writes+=reg(0,0x28,0xf0|c|(4 if bank else 0))
 return writes

def cases():
 out=[]
 def add(name,steps):out.append(dict(Name=name,Steps=[dict(Writes=w,Count=n)for w,n in steps]))
 add('reset-and-zero', [([],0),([],1),([],2),([],100),([],12288)])
 for algorithm in range(8):
  setup=[]
  for channel in range(6):setup+=tone(channel,algorithm,channel%8,channel)
  add('algorithm-'+str(algorithm),[(setup,1),([],29),([],3073),(reg(0,0xb4,0x40)+reg(1,0xb6,0x80),177),([] ,4500),(sum((reg(0,0x28,c|(4 if bank else 0))for bank in range(2)for c in range(3)),[]),8192)])
 for shape in range(8,16):
  setup=tone(0,7,7,shape,shape)
  add('ssg-'+str(shape),[(setup,1),([],2),([],12289),(reg(0,0x50,0x01)+reg(0,0x90,shape^4),1100),(reg(0,0x28,0),15000),(reg(0,0x28,0xf0),3000)])
 for speed in range(8):
  setup=[]
  for channel in range(6):
   bank,c=divmod(channel,3);setup+=tone(channel,channel%8,channel%8,channel)+reg(bank,0xb4+c,0xc0|((channel%4)<<4)|((channel+speed)%7+1))
  setup+=reg(0,0x22,8|speed)
  add('lfo-'+str(speed),[(setup,1),([],17),([],14003),(reg(0,0x22,0),111),(reg(0,0x22,8|((speed+3)%8)),10000)])
 setup=tone(2,3,5,7)
 for c in range(3):setup+=reg(0,0xac+c,0x10+c*7)+reg(0,0xa8+c,0x30+c*47)
 add('special-csm-timers',[(setup+reg(0,0x27,0x40),555),(reg(0,0x24,0xff)+reg(0,0x25,1)+reg(0,0x26,0xff)+reg(0,0x27,0x8f),999),(reg(0,0x28,2),37),([],513),(reg(0,0x27,0x3f),1),([],141),(reg(0,0x27,0),1000)])
 add('address-and-frequency-latches',[(tone(0,7,0,0)+tone(3,6,1,1),16),([[0,0xa4],[2,0xa5],[1,0x3f],[0,0xa1],[3,0xff]],1),([[2,0x50],[0,0x51],[3,0x80],[0,0xa0],[1,0x01],[0,0x28],[1,0xf0]],0),([],1),([[2,0x2a],[1,255],[0,0x2a],[3,0],[0,0x2b],[1,128]],999),(reg(0,0x2a,255)+reg(1,0xb6,0),1),(reg(0,0x2b,0),771)])
 add('dac-channel-freeze',[(tone(5,7,7,2),321),(reg(0,0x2a,0)+reg(0,0x2b,0x80),141),(reg(0,0x2a,255)+reg(1,0xb6,0),89),(reg(1,0xb6,0x40),55),(reg(0,0x2b,0),999)])
 rng=random.Random(0xf32d17);steps=[]
 for i in range(256):
  writes=[]
  for _ in range(rng.randrange(1,10)):
   bank=rng.randrange(2);addr=rng.choice([0x22,0x24,0x25,0x26,0x27,0x28,0x2a,0x2b]+list(range(0x30,0xb7)));value=rng.randrange(256)
   writes+=reg(bank,addr,value)
  steps.append((writes,rng.randrange(0,193)))
 add('seeded-all-registers',steps)
 return out

def capture(path):
 # Loading a shared library alone leaves optional per-channel volumes at zero.
 # The existing frontend initializes real config. No gameplay frame is run.
 ref=AudioSession(ROOT,ROOT/'.local/md-yuyuhakusho/fmpsg-ym-capture'/path.parent.name,core=path)
 lib=ref.lib;look=symbols(path);base=C.cast(lib.YM2612Update,C.c_void_p).value-look['_YM2612Update'][0];ym=YM.from_address(base+look['_ym2612'][0])
 lib.YM2612Init.argtypes=[];lib.YM2612ResetChip.argtypes=[];lib.YM2612Config.argtypes=[C.c_int];lib.YM2612Write.argtypes=[C.c_uint,C.c_uint];lib.YM2612Update.argtypes=[C.c_void_p,C.c_int]
 rows=[]
 for case in cases():
  lib.YM2612Init();lib.YM2612Config(0);lib.YM2612ResetChip();initial=normal(ym);steps=[]
  for step in case['Steps']:
   for port,value in step['Writes']:lib.YM2612Write(port,value)
   buffer=(C.c_int32*(max(1,step['Count'])*2))();lib.YM2612Update(buffer,step['Count']);raw=C.string_at(buffer,step['Count']*8)
   steps.append(dict(**step,Output=base64.b64encode(raw).decode(),State=normal(ym)))
  rows.append(dict(Name=case['Name'],Initial=initial,Steps=steps))
 ref.close()
 assert any(int.from_bytes(base64.b64decode(step['Output'])[i:i+4],'little',signed=True)!=768 for row in rows for step in row['Steps'] for i in range(0,step['Count']*8,4)), 'all FM output muted; config not initialized'
 return rows

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 original=capture(ORIGINAL);observed=capture(PINNED);assert original==observed,'unaltered original and reference-hook core disagree at raw YM boundary'
 raw=(json.dumps(dict(Profile='YM2612_DISCRETE_RAW_1008',Cases=original),separators=(',',':'))+'\n').encode();body=gzip.compress(raw,mtime=0);(out/'ym-native.json.gz').write_bytes(body)
 report=dict(pass_=True,scope='Declared private raw YM ports and YM2612Update calls; original and hooked native library exactly equal; no CPU/game timing claim',cases=len(original),steps=sum(len(c['Steps'])for c in original),stereo_samples=sum(s['Count']for c in original for s in c['Steps']),original_sha256=sha(ORIGINAL.read_bytes()),observed_sha256=sha(PINNED.read_bytes()),producer_sha256=sha(Path(__file__).read_bytes()),gzip_sha256=sha(body),json_sha256=sha(raw),json_bytes=len(raw),host_context_pointers_discarded=True)
 report['pass']=report.pop('pass_');(out/'native-capture.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
