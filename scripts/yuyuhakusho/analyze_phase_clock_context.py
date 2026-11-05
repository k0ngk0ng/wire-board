"""Summarize the original joined ledger; never predicts a future bus event."""
import argparse,gzip,json
from collections import Counter
from pathlib import Path
from reference import ROOT,sha

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
 out=args.out.resolve();assert out.is_relative_to(ROOT)
 rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();reports=[]
 for scenario in ['baseline','pause']:
  path=args.capture.resolve()/f'phase-clock-{scenario}.json.gz';raw=path.read_bytes();s=json.loads(gzip.decompress(raw));bf={name:i for i,name in enumerate(s['bus_fields'])};ef={name:i for i,name in enumerate(s['event_fields'])}
  bus=s['bus_events'];markers=[e for e in s['events']if e[ef['kind']]in[13,14]]
  read_bad=[];stalled=[]
  for i,e in enumerate(bus):
   mapped=e[bf['mapped_address']]
   if not(0<=mapped<len(rom))or rom[mapped]!=e[bf['value']]:read_bad.append(i)
   if e[bf['z80_after']]-e[bf['z80_before']]>45:stalled.append({'bus_index':i,'frame':e[bf['frame']],'ordinal':e[bf['ordinal']],'before':e[bf['z80_before']],'after':e[bf['z80_after']],'dma_end':e[bf['dma_end']],'dma_remaining':e[bf['dma_length']],'dma_mode':e[bf['dma_type']]})
  assert not read_bad,('Bank payload does not match pinned ROM',scenario,read_bad[:3])
  reports.append({'scenario':scenario,'source':{'path':str(path.relative_to(ROOT)),'sha256':sha(raw)},'bank_read_count':len(bus),'mapped_rom_min':min(e[bf['mapped_address']]for e in bus),'mapped_rom_max':max(e[bf['mapped_address']]for e in bus),'all_observed_bank_values_match_exact_ROM':True,'bank_writes_observed':sum(e[ef['kind']]==14 for e in markers),'markers_during_irq':sum(e[ef['irq_depth']]>0 for e in markers),'markers_during_world_callback':sum(e[ef['callback_id']]!=0 for e in markers),'markers_during_default_arena':sum(e[ef['arena']]!=0 for e in markers),'reads_started_at_or_after_video_frame_end':sum(e[bf['z80_before']]>=896040 for e in bus),'DMA_modes':dict(Counter(e[bf['dma_type']]for e in bus)),'zstate_before':dict(Counter(e[bf['zstate_before']]for e in bus)),'DMA_waits':stalled})
 result={'pass':True,'scope':'Original-ROM bank values and joined IRQ/callback/arena context only; these contexts do not infer future interrupt or producer timing','rom_sha256':sha(rom),'reports':reports,'producer_sha256':sha(Path(__file__).read_bytes())}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'pass':True,'bank_reads':sum(x['bank_read_count']for x in reports)}))
if __name__=='__main__':main()
