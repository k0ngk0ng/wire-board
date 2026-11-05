"""Verify direct native bank transactions and their same-ledger arena costs.

This checker consumes original observations only. It is not an independent
world/IRQ scheduler and never decodes or executes the original CPU program.
"""
import argparse, gzip, hashlib, json
from bisect import bisect_left,bisect_right
from collections import Counter
from pathlib import Path
from arena_phase_cost_model import MODEL, budget

ROOT=Path(__file__).resolve().parents[2]
PERIOD=896040

def analyze(source):
    events=source['events']; fields={name:i for i,name in enumerate(source['event_fields'])}
    def ev(event,name):return event[fields[name]]
    bus=source.get('bus_events',[]); bf={name:i for i,name in enumerate(source.get('bus_fields',[]))}
    def bv(event,name):return event[bf[name]]
    errors=[]; bus_checks=[]; phase_checks=[]; external=[]; transactions={}; delays=Counter(); kinds=Counter()
    if not bus:errors.append({'reason':'No direct bank-transaction observations; residual arithmetic alone is not enough'})
    for index,item in enumerate(bus):
        start=bv(item,'z80_before');dma_end=bv(item,'dma_end');dma_type=bv(item,'dma_type')
        after_dma=dma_end if start<dma_end and dma_type<2 else start
        steal=((after_dma%7+72)//7)*7
        expected_z80=after_dma+45
        expected_zstate=bv(item,'zstate_before')|(4 if after_dma!=start and bv(item,'dma_length') else 0)
        mapped=(bv(item,'bank')|(bv(item,'guest_address')&0x7fff))if bv(item,'guest_address')>=0x8000 else 0xc00000|(bv(item,'guest_address')&0xffff)
        row={'index':index,'frame':bv(item,'frame'),'ordinal':bv(item,'ordinal'),'kind':bv(item,'kind'),'m68k_delta':bv(item,'m68k_after')-bv(item,'m68k_before'),'z80_delta':bv(item,'z80_after')-start,'predicted_steal':steal,'dma_wait':after_dma-start}
        row['pass']=(row['m68k_delta']==steal and bv(item,'z80_after')==expected_z80 and bv(item,'zstate_after')==expected_zstate and mapped==bv(item,'mapped_address'))
        bus_checks.append(row);transactions[row['ordinal']]=(index,item)
        delays[row['m68k_delta']]+=1;kinds[row['kind']]+=1
        if not row['pass']:errors.append({'reason':'Native bus transaction differs from declared pinned-core arithmetic','event':row,'raw':item})
    joined=0
    for event in events:
        if ev(event,'kind')not in[13,14]:continue
        ordinal=ev(event,'source_ordinal');index=ev(event,'a0')
        found=transactions.get(ordinal)
        if found is None or found[0]!=index:errors.append({'reason':'Bus marker does not join exactly once by original ordinal','ordinal':ordinal,'index':index});continue
        item=found[1]
        if ev(event,'frame')!=bv(item,'frame')or ev(event,'cycles')!=bv(item,'m68k_after')or ev(event,'value')!=bv(item,'value')or ev(event,'address')!=bv(item,'mapped_address'):errors.append({'reason':'Joined transaction identity differs','ordinal':ordinal})
        joined+=1
    if joined!=len(bus):errors.append({'reason':'Bus ledger count mismatch','joined':joined,'bus':len(bus)})
    ordinals=sorted(transactions)
    fixed={p['start']:p for p in MODEL};pending=None;calls=[]
    for event in events:
        kind=ev(event,'kind')
        if kind==5:
            if pending is not None:errors.append({'reason':'Nested arena callback'})
            pending=[]
        elif kind==6 and pending is not None:pending.append(event)
        elif kind==7 and pending is not None:calls.append(pending);pending=None
    for blocks in calls:
        for index,start in enumerate(blocks[:-1]):
            phase=fixed.get(ev(start,'pc')&0xffffff)
            if phase is None:continue
            end=blocks[index+1]
            if ev(end,'pc')&0xffffff!=phase['end']:errors.append({'reason':'Semantic block endpoint differs'});continue
            begin_ord=ev(start,'source_ordinal');end_ord=ev(end,'source_ordinal')
            selected=[transactions[ordinal][1]for ordinal in ordinals[bisect_right(ordinals,begin_ord):bisect_left(ordinals,end_ord)]]
            elapsed=(ev(end,'frame')-ev(start,'frame'))*PERIOD+ev(end,'cycles')-ev(start,'cycles')
            dma_wait=sum((max(bv(t,'z80_before'),bv(t,'dma_end'))-bv(t,'z80_before'))if bv(t,'dma_type')<2 else 0 for t in selected)
            row={'frame':ev(start,'frame'),'phase':phase['name'],'elapsed_master':elapsed,'base_master':budget(phase)['master_base'],'refreshes':ev(end,'a0'),'irq_entries':ev(end,'a1'),'own_hardware_accesses':ev(end,'a2'),'direct_bus_transactions':len(selected),'direct_bus_steal_master':sum(bv(t,'m68k_after')-bv(t,'m68k_before')for t in selected),'dma_wait_master':dma_wait}
            row['residual_without_bus']=elapsed-row['base_master']-14*row['refreshes']
            row['residual_after_direct_bus']=row['residual_without_bus']-row['direct_bus_steal_master']
            if row['irq_entries']or row['own_hardware_accesses']:external.append(row)
            else:
                row['pass']=row['residual_after_direct_bus']==0;phase_checks.append(row)
                if not row['pass']:errors.append({'reason':'Ordinary semantic block residual remains after direct bus steals','event':row})
    check_digest=hashlib.sha256(json.dumps(bus_checks,separators=(',',':')).encode()).hexdigest()
    return {'pass':not errors,'scope':'Direct pinned-native-core bus-transaction arithmetic and attribution to the same original event ledger; no independent event schedule or hardware claim','bus_count':len(bus),'bus_joined':joined,'bus_kinds':dict(kinds),'m68k_delta_counts':dict(delays),'dma_delayed_transactions':sum(x['dma_wait']>0 for x in bus_checks),'bus_check_digest':check_digest,'arena_callbacks':len(calls),'ordinary_phase_checks':len(phase_checks),'ordinary_with_bus_count':sum(x['direct_bus_transactions']>0 for x in phase_checks),'ordinary_nonzero_residual_before_bus':sum(x['residual_without_bus']!=0 for x in phase_checks),'ordinary_residual_after_bus_failures':sum(not x['pass']for x in phase_checks),'external_intervals':external,'phase_checks':phase_checks,'errors':errors,'limits':['All transaction times and semantic interval boundaries come from the original observer','All raw bus transactions remain in the source; this report retains their complete ordered check digest and every failure','The old pure-CPU model remains frozen and invalid on natural traces','Independent PCM bank events, high-level callback costs and IRQ scheduling remain separate obligations']}

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--source',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
    source=args.source.resolve();out=args.out.resolve();assert out.is_relative_to(ROOT)
    raw=source.read_bytes();result=analyze(json.loads(gzip.decompress(raw)))
    result['source']={'path':str(source.relative_to(ROOT)),'sha256':hashlib.sha256(raw).hexdigest()}
    output=(json.dumps(result,indent=2)+'\n').encode()
    out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress(output,mtime=0)if out.suffix=='.gz'else output)
    print(json.dumps({k:v for k,v in result.items()if k not in['external_intervals','bus_checks','phase_checks','errors','limits','scope','source']}))
    if not result['pass']:
        print(json.dumps(result['errors'][:3]));raise SystemExit(1)
if __name__=='__main__':main()
