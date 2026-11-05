"""Verify retained PCM-pair evidence without running an emulator or player."""
from pathlib import Path
from array import array
import argparse,hashlib,json,sys,wave
ROOT=Path(__file__).resolve().parents[2]
def sha(raw):return hashlib.sha256(raw).hexdigest()
def read(path):
 with wave.open(str(path),'rb')as f:
  assert(f.getnchannels(),f.getsampwidth(),f.getframerate())==(2,2,44100);raw=f.readframes(f.getnframes())
 out=array('h');out.frombytes(raw)
 if sys.byteorder!='little':out.byteswap()
 return raw,out
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--directory',type=Path,default=Path('games/yuyuhakusho/engine/testdata/pcm-mix-counterexample'));p.add_argument('--out',type=Path,required=True);p.add_argument('--expect-linear',action='store_true');a=p.parse_args()
directory=a.directory.resolve();out=a.out.resolve();assert directory.is_relative_to(ROOT)and out.is_relative_to(ROOT)
manifest=json.loads((directory/'manifest.json').read_text());values=[]
for name in ['single-a','single-b-late','pair-a-b']:
 path=directory/(name+'.wav');raw,samples=read(path);assert sha(path.read_bytes())==manifest['files'][name]['wav_sha256'];assert sha(raw)==manifest['files'][name]['pcm_sha256'];values.append(samples)
x,y,m=values;assert len(x)==len(y)==len(m);cut=manifest['second_request_stereo_frame']*2
assert x[:cut]==m[:cut]and not any(y[:cut])
sum_diffs=0;saturations=0;peak=0;saa=sbb=sab=sam=sbm=0
for i,j,k in zip(x,y,m):
 raw=i+j;candidate=max(-32768,min(32767,raw));sum_diffs+=candidate!=k;saturations+=raw!=candidate;peak=max(peak,abs(candidate-k))
 saa+=i*i;sbb+=j*j;sab+=i*j;sam+=i*k;sbm+=j*k
# Exact normal equations: if any constant gains reproduced every sample,
# the nondegenerate least-squares solution would have zero rational residual.
den=saa*sbb-sab*sab;assert den>0;na=sam*sbb-sbm*sab;nb=sbm*saa-sam*sab
residuals=sum(k*den!=i*na+j*nb for i,j,k in zip(x,y,m))
result={'pass':not a.expect_linear or sum_diffs==0,'scope':'Same-checkpoint/timebase original mono-pair WAV evidence, not a mixing implementation','stereo_frames':len(m)//2,'second_request_stereo_frame':cut//2,'pre_second_request_a_pcm_identical':True,'pre_second_request_b_pcm_exact_zero':True,'unmodified_saturating_sum_different_sample_values':sum_diffs,'sum_saturation_sample_values':saturations,'maximum_absolute_sum_error':peak,'same_alignment_exact_constant_gain_solution':{'gain_a_numerator':str(na),'gain_b_numerator':str(nb),'denominator':str(den),'gain_a_float':na/den,'gain_b_float':nb/den,'nonzero_rational_residual_sample_values':residuals,'exact_reconstruction':residuals==0},'sample_shift_search_performed':False,'limits':['No arbitrary independent waveform realignment was attempted; fractional/variable timing and DAC/filter state remain unresolved','The nonzero residual excludes one fixed gain per standalone capture at this verified request alignment, not every possible phase-dependent mixer','These decoded source samples are not proof of browser or speaker output'],'analyzer_sha256':sha(Path(__file__).read_bytes())}
out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if not result['pass']:raise SystemExit(1)
