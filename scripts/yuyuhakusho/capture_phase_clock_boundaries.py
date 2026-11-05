"""Corroborate the already frozen boundary expectations with both native cores.

These are declared private handler states, not natural game execution. The
native ROM memory handler performs each access; no CPU instruction is emulated.
"""
import argparse,ctypes as C,json
from pathlib import Path
from extract_audio_assets import AudioSession
from reference import ROOT,sha

class Z80(C.Structure):
 _fields_=[(k,C.c_uint32)for k in ['pc','sp','af','bc','de','hl','ix','iy','wz','af2','bc2','de2','hl2']]+[(k,C.c_uint8)for k in ['r','r2','iff1','iff2','halt','im','i','nmi_state','nmi_pending','irq_state','after_ei']]+[('cycles',C.c_uint32),('daisy',C.c_void_p),('irq_callback',C.c_void_p)]

# cycle, DMA end/mode/remaining, prior zstate, native resume/main-delta/flags.
# Public helper boundary expectations were frozen before bus.go existed.
CASES=[
 (0,0,0,0,1,45,70,1),(5,0,0,0,1,50,77,1),(6,0,0,0,1,51,77,1),
 (0,5,0,1,1,50,77,5),(0,5,1,0,1,50,77,1),
 (0,5,2,1,1,45,70,1),(0,5,3,1,1,45,70,1),
 (5,5,0,1,1,50,77,1),(0xffffffff-45,0,0,0,1,0xffffffff,70,1),
 (0,5,0,1,5,50,77,5),(0,0,0,0,5,45,70,5),
]

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 out=args.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 test=ROOT/'games/yuyuhakusho/audio/phaseclock/bus_test.go';test_first=json.loads((ROOT/'games/yuyuhakusho/verification/phase-clock/go-test-first.json').read_text())
 assert sha(test.read_bytes())==test_first['files'][str(test.relative_to(ROOT))],'Frozen boundary declarations changed'
 all_rows=[];identities={}
 for mode,core in [('original',ROOT/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib'),('hook',ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/genesis_plus_gx_libretro.dylib')]:
  rows=[]
  with AudioSession(ROOT,out/('core-'+mode),core=core)as ref:
   assert C.sizeof(Z80)==88 and Z80.cycles.offset==64
   z=Z80.in_dll(ref.lib,'Z80');bank=C.c_uint.in_dll(ref.lib,'zbank');end=C.c_uint.in_dll(ref.lib,'dma_endCycles');kind=C.c_ubyte.in_dll(ref.lib,'dma_type');remaining=C.c_uint.in_dll(ref.lib,'dma_length');flags=C.c_ubyte.in_dll(ref.lib,'zstate');zr=(C.c_ubyte*8192).in_dll(ref.lib,'zram')
   native=ref.api('z80_memory_r',C.c_ubyte,C.c_uint)
   for i,(cycle,dma_end,dma_mode,dma_left,zstate,resume,steal,wait_state)in enumerate(CASES):
    z.cycles=cycle;ref.cpu.cycles=4096;bank.value=0x48000;end.value=dma_end;kind.value=dma_mode;remaining.value=dma_left;flags.value=zstate
    before=sha(ref.ram_cpu()),sha(bytes(zr));regs={k:int(getattr(z,k))for k,_ in Z80._fields_ if k not in ['cycles','daisy','irq_callback']}
    value=native(0x8123)
    after=sha(ref.ram_cpu()),sha(bytes(zr));assert before==after,'ROM read changed guest RAM'
    assert regs=={k:int(getattr(z,k))for k in regs},'ROM read changed guest registers'
    result={'case':i,'cycle':cycle,'dma_end':dma_end,'dma_mode':dma_mode,'dma_remaining':dma_left,'zstate_before':zstate,'data':value,'z80_after':int(z.cycles),'m68k_delta':int(ref.cpu.cycles)-4096,'zstate_after':int(flags.value),'RAM_and_registers_unchanged':True}
    rows.append(result)
    assert (result['z80_after'],result['m68k_delta'],result['zstate_after'])==(resume,steal,wait_state),('Native boundary disagrees with prior declaration',mode,result)
   identities[mode]=ref.identity
  if all_rows:assert rows==all_rows,'Two original native builds disagree on bus effects'
  all_rows=rows
 report={'pass':True,'scope':'Declared private native ROM handler states validating prior boundary expectations; not natural DMA-mode coverage or autonomous scheduling','cases_per_core':len(CASES),'both_native_builds_equal':True,'rows':all_rows,'identity':identities,'prior_test_first':test_first,'producer_sha256':sha(Path(__file__).read_bytes()),'test_sha256_unchanged':sha(test.read_bytes()),'limits':['Mode 2/3 and nonzero remaining/wait-flag cases are private observations; the natural two streams only reached DMA modes 0/1 and prior zstate 1','Invalid arithmetic inputs are rejected by the independent helper; uint32 wrap is not accepted as a profile effect']}
 (out/'boundaries.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'cases_per_core':len(CASES)}))
if __name__=='__main__':main()
