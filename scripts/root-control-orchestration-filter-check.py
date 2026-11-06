#!/usr/bin/env python3
"""Execute only the passive C filter, never the native game or reference core."""
import argparse
import ctypes as C
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--source',default='scripts/yuyuhakusho/hook_filter.c')
    parser.add_argument('--entrypoint',default='yyh_cpu_hook')
    parser.add_argument('--physical-address',choices=['port','last-pc'],default='port')
    parser.add_argument('--out',default='games/yuyuhakusho/verification/root-control-orchestration/physical-filter.json')
    parser.add_argument('--retain-source')
    args=parser.parse_args()
    root=Path.cwd()
    source=root/args.source
    raw=source.read_bytes()
    listing=root/'docs/research/md-yuyuhakusho/static/audio-loop-sequencer.txt'
    sites=[int(match[1],16) for match in re.finditer(r'^([0-9a-f]{8}) move\.b .*,\(0x00c00011\)\.l$',listing.read_text(),re.M)]
    assert len(sites)==13
    scratch=root/'.local/root-control-orchestration-filter'
    scratch.mkdir(parents=True,exist_ok=True)
    env=os.environ.copy()
    env.update(TMPDIR=str(scratch),CLANG_MODULE_CACHE_PATH=str(scratch/'module-cache'))
    with tempfile.TemporaryDirectory(dir=scratch) as temporary:
        temporary=Path(temporary)
        snapshot=temporary/'filter.c'
        snapshot.write_bytes(raw)
        library=temporary/'filter.dylib'
        result=subprocess.run(['cc','-shared','-fPIC','-I',str(source.parent),str(snapshot),'-o',str(library)],env=env,capture_output=True,text=True)
        assert result.returncode==0,result.stderr
        lib=C.CDLL(str(library))
        observer=C.CFUNCTYPE(None,C.c_int,C.c_int,C.c_uint,C.c_uint)
        lib.yyh_set_observer.argtypes=[observer,C.POINTER(C.c_uint),C.c_size_t]
        lib.yyh_set_observer.restype=C.c_int
        hook=getattr(lib,args.entrypoint)
        hook.argtypes=[C.c_int,C.c_int,C.c_uint,C.c_uint]
        events=[]
        callback=observer(lambda kind,width,address,value:events.append([kind,width,address,value]))
        points=(C.c_uint*1)(0x375ae)
        assert lib.yyh_set_observer(callback,points,1)==1
        hook(1,2,0x375ae,0)
        pc_forward=events==[[1,2,0x375ae,0]]
        events.clear()
        for value in (0x9f,0xdf,0xff):
            hook(4,1,0xc00011,value)
        physical_address=0x375ae if args.physical_address=='last-pc' else 0xc00011
        physical_forward=events==[[4,1,physical_address,value] for value in (0x9f,0xdf,0xff)]
        physical_events=events.copy()
        events.clear()
        for item in [(4,2,0xc00011,0xff),(4,1,0xc00013,0xff),(3,1,0xc00011,0xff),(1,2,0x38610,0)]:
            hook(*item)
        unrelated_rejected=not events
        events.clear()
        assert lib.yyh_set_observer(callback,None,0)==1
        for pc in sites:
            hook(1,2,pc,0)
            hook(4,1,0xc00011,0x9f)
        expected=[[4,1,pc if args.physical_address=='last-pc' else 0xc00011,0x9f] for pc in sites]
        every_site_forward=events==expected
        events.clear()
        hook(4,1,0xffc00011,0xdf)
        expected_address=sites[-1] if args.physical_address=='last-pc' else 0xffc00011
        alias_forward=events==[[4,1,expected_address,0xdf]]
        events.clear()
        lib.yyh_set_observer(observer(),None,0)
        hook(4,1,0xc00011,0xff)
        detached_rejected=not events
    if args.retain_source:
        (root/args.retain_source).write_bytes(raw)
    output={'version':1,'pass':pc_forward and physical_forward and unrelated_rejected and every_site_forward and alias_forward and detached_rejected,
            'scope':'Only the passive filter exercised with declared synthetic callback arguments; no native core loaded',
            'source':{'path':args.source,'sha256':sha(raw)},
            'entrypoint':args.entrypoint,'physical_address':args.physical_address,
            'include_sources':[{'path':str((source.parent/name).relative_to(root)),'sha256':sha((source.parent/name).read_bytes())} for name in re.findall(r'#include \"([^\"]+)\"',raw.decode())],
            'pc_filter_forward':pc_forward,'type4_byte_C00011_forward':physical_forward,
            'physical_events':physical_events,'unrelated_read_word_pc_rejected':unrelated_rejected,
            'all_13_unselected_sites_forward':every_site_forward,'24bit_port_alias_forward':alias_forward,'detached_callback_rejected':detached_rejected,
            'listing':{'path':str(listing.relative_to(root)),'sha256':sha(listing.read_bytes())},
            'full_port_ledger_proven':False,'whole_goal_complete':False}
    (root/args.out).write_text(json.dumps(output,indent=2,sort_keys=True)+'\n')
    print(json.dumps({'pass':output['pass'],'pc':pc_forward,'physical':physical_forward,'unrelated':unrelated_rejected,'source_sha256':sha(raw)}))


if __name__=='__main__':
    main()
