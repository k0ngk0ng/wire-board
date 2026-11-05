#!/usr/bin/env python3
"""Independently classify every frozen original CPU RAM write by instruction site."""
import argparse
from collections import Counter, defaultdict
import gzip
import json
from pathlib import Path
import sys

sys.dont_write_bytecode = True
from reference import ROOT, sha


def site_table():
    sites = {}
    def add(pcs, start, end, width):
        for pc in pcs: sites[pc] = (start, end, width)
    def fixed(pc, address, width): add([pc], address, address+width, width)
    for pc, address in [(0x21588,0xa00e),(0x2158c,0xa018),(0x21598,0xa018),(0x215ac,0xa018),(0x215c4,0xa01a),(0x215d2,0xa014),(0x215de,0xa010),(0x215e6,0xa012),(0x215f4,0xa016)]: fixed(pc,address,2)
    fixed(0x216aa,0xf598,4);fixed(0x216ae,0xf59c,4)
    add([0x216be,0x216ca,0x216d6,0x216e2],0x6000,0x61c0,2)
    add([0x216c4,0x216d0,0x216dc,0x216e8],0x6200,0x63c0,2)
    # 4: each fill/copy has its own instruction sites and exact output interval.
    fixed(0x20ccc,0xf6e7,1);fixed(0x20cd2,0x81a0,2);fixed(0x20cd6,0x81a2,2);fixed(0x20d28,0x8186,2)
    add([0x20ce8,0x20cea,0x20cec,0x20cee],0xe8a0,0xe8e0,4)
    add([0x20d02,0x20d04,0x20d06,0x20d08],0xe960,0xe9e0,4)
    add([0x20d1c,0x20d1e,0x20d20,0x20d22],0xeaf0,0xeb50,4)
    for pc,address,width in [(0x20d3c,0xe500,2),(0x20d40,0xe528,2),(0x20d4e,0xf598,4),(0x20d58,0xf59c,4),(0x20d6e,0x8188,2)]:fixed(pc,address,width)
    add([0x20d8c,0x20d8e,0x20d90,0x20d92],0xe8e0,0xe960,4)
    add(list(range(0x20da0,0x20db0,2)),0xea10,0xeaf0,2)
    add([0x20dbc,0x20dbe,0x20dc0,0x20dc2],0xeb50,0xebf0,4)
    fixed(0x20dd6,0x81a2,2);fixed(0x20ddc,0x81a2,2);fixed(0x20dec,0x81a0,2)
    for pc,address in [(0x20df8,0xe092),(0x20dfa,0xe094),(0x20dfc,0xe096)]:fixed(pc,address,2)
    # c: phase +1c is a word, unlike the two full long accumulators.
    for pc,address,width in [(0x21312,0xf6e7,1),(0x21318,0xf6df,1),(0x2131e,0x819c,2),(0x21322,0x819e,2),(0x21326,0x8190,4),(0x2132a,0x8194,4),(0x2132e,0x8186,2),
                             (0x2133e,0xe500,2),(0x21348,0xe528,2),(0x2135e,0xf598,4),(0x2136e,0xf59c,4),(0x21378,0x8190,4),(0x21380,0x8194,4),(0x21388,0x819c,2),(0x2139c,0x8188,2)]:fixed(pc,address,width)
    add([0x213ca,0x213cc],0xe8a0,0xe9e0,2)
    add([0x21404,0x21406,0x21408,0x2140a],0xea20,0xebe0,4)
    # e: half-word writes at +14/+18 do not own their low halves.
    for pc,address,width in [(0x2142c,0xf6e7,1),(0x21432,0xf6df,1),(0x21438,0x8188,4),(0x2143c,0x8186,2),(0x2144c,0xe500,2),(0x21456,0xe528,2),(0x2146c,0xf598,4),(0x2147c,0xf59c,4),(0x21486,0x8188,4),
                             (0x214aa,0x818c,2),(0x214ae,0x8190,4),(0x214b4,0x8194,2),(0x214c8,0x8198,2)]:fixed(pc,address,width)
    add([0x214de,0x214e0,0x214e2,0x214e4],0xe8a0,0xe920,4)
    add([0x214f8,0x214fa,0x214fc,0x214fe],0xe920,0xe970,4)
    add([0x2150c,0x2150e,0x21510,0x21512],0xe970,0xe9f0,4)
    add([0x2152c,0x2152e,0x21530,0x21532],0xea20,0xeb90,4)
    add([0x21548,0x2154a,0x2154c,0x2154e],0xeb90,0xebe0,4)
    return sites


CALLS = {0x20cbe:0x20cc0,0x20d2c:0x20d30,0x20d30:0x20d34,0x21304:0x21306,0x21332:0x21336,0x2141e:0x21420,0x21440:0x21444}


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',required=True,type=Path);args=p.parse_args()
    output=args.out.resolve()
    if not output.is_relative_to(ROOT):raise ValueError('output outside worktree')
    sites=site_table();counts=Counter();irq_entries=Counter();rtes=Counter();sound=Counter();external=defaultdict(Counter);sources={};exception_frames=[]
    def audit(row):
        trace=row['audit']['trace'];ram=bytearray.fromhex(row['audit']['ram_before']);exception_indices=set()
        assert len(ram)==65536 and row['audit']['ram_bytes']==65536 and row['audit']['unknown_writes']==0
        for i,e in enumerate(trace):
            if e[5]!=2:continue
            assert i>=2
            a,b=trace[i-2:i]
            assert a[5]==b[5]==1 and a[2]==4 and b[2]==2 and (b[1]&65535)+2==(a[1]&65535)
            assert a[4]==b[4]==e[4]-1
            exception_indices.update([i-2,i-1]);irq_entries[hex(e[0])]+=1
            exception_frames.append({'arena':row['arena_id'],'frame':row['frame'],'callback':row['kind'],'irq_pc':hex(e[0]),
                                     'pc_word_address':hex(a[1]),'return_pc':hex(a[3]),'sr_word_address':hex(b[1]),'sr':hex(b[3]),'bytes':6})
        assert sum(e[5]==2 for e in trace)==sum(e[5]==3 for e in trace)
        for i,e in enumerate(trace):
            pc,address,width,value,depth,kind=e
            if kind==3:rtes[hex(pc)]+=1
            if kind==4 and 0x21960<=pc&0xffffff<=0x219e8:sound[hex(pc)]+=1
            if kind!=1:continue
            assert width in [1,2,4]
            lo=address&65535;pc24=pc&0xffffff
            if i in exception_indices:counts['exception_stack_bytes']+=width
            elif depth:
                external[hex(pc)][hex(address)]+=1;counts['explicit_irq_writes']+=1
            elif pc24 in CALLS:
                assert width==4 and value==CALLS[pc24] and (row['stack']&65535)-12<=lo<(row['stack']&65535)
                counts['exact_bsr_stack_writes']+=1
            else:
                assert pc24 in sites,('unknown instruction writes RAM',hex(pc),hex(address),width)
                start,end,expected_width=sites[pc24]
                assert width==expected_width and start<=lo and lo+width<=end,('wrong write target',hex(pc),hex(address),width,(start,end,expected_width))
                counts['exact_component_writes']+=1
            raw=(value&((1<<(8*width))-1)).to_bytes(width,'big')
            for j,b in enumerate(raw):ram[(lo+j)&65535]=b
        assert sha(ram)==row['audit']['ram_after_sha256']
        counts['calls']+=1;counts['audited_ram_bytes']+=65536
    fixture_root=ROOT/'games/yuyuhakusho/engine/testdata'
    for id in ['4','c','e']:
        for part in ['natural','boundaries']:
            path=fixture_root/f'arenas-4-c-e-{id}-{part}.jsonl.gz';sources[str(path.relative_to(ROOT))]=sha(path.read_bytes())
            with gzip.open(path,'rt')as f:
                for line in f:audit(json.loads(line))
        path=fixture_root/f'arenas-4-c-e-{id}-sequences.json.gz';sources[str(path.relative_to(ROOT))]=sha(path.read_bytes())
        with gzip.open(path,'rt')as f:
            for stream in json.load(f)['streams']:
                for row in stream['calls']:audit(row)
    report={'pass':True,'scope':'frozen original CPU writes; exact per-instruction targets, exact BSR return words, separately identified exception 6-byte stack frames, explicit IRQ effects; complete 65536-byte reconstruction',
            'unknown_writes':0,'counts':dict(counts),'irq_entry_pc_counts':dict(irq_entries),'rte_pc_counts':dict(rtes),
            'sound_219xx_execute_counts':dict(sound),'external_irq_write_sites':{pc:dict(a)for pc,a in external.items()},
            'exception_stack_frames':exception_frames,'sources':sources,'auditor_sha256':sha(Path(__file__).read_bytes()),
            'limits':['IRQ effects are retained as exact original writes with raw 32-bit PCs; they are not attributed to arena rules.',
                      '37cac was included by the observer but has zero writes in these natural inputs.']}
    output.parent.mkdir(parents=True,exist_ok=True);output.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({k:report[k]for k in ['pass','unknown_writes','counts','irq_entry_pc_counts','rte_pc_counts']},indent=2))


if __name__=='__main__':main()
