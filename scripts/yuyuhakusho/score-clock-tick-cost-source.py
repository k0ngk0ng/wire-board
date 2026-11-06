"""Offline binding of named fixed tick operations to pinned architecture cost."""
import argparse,json,re
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)
 base=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k';table=base/'m68kops.h';rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',table.read_text())
 declarations=[('domain-clear',0x375ae,16),('pause-first-read',0x375b2,12),('pause-nonzero-branch',0x375b6,10),('hold-compare-read',0x37b90,16),('hold-branch',0x37b96,10),('hold-return',0x37b9a,16),('resume-read',0x37b9c,12),('resume-branch',0x37ba0,10),('pause-ack-store',0x37ba2,16),('resume-clear-store',0x37c08,16),('tick-return',0x37656,16),('priority-byte-clear',0x390f6,16)]
 rows=[]
 for name,pc,cpu in declarations:
  op=int.from_bytes(rom[pc:pc+2],'big');found=[v for v in entries if op&int(v[1],16)==int(v[2],16)][-1];assert int(found[3])==cpu
  rows.append({'operation':name,'source_pc':pc,'source_opcode':op,'architecture_handler':found[0],'cpu_cycles':cpu})
 report={'pass':True,'scope':'Only fixed gate/return/byte-clear costs; source PCs/opcodes are offline provenance, never production input','operations':rows,'short_branch_not_taken':8,'long_branch_not_taken':12,'master_multiplier':7,'refresh_delay_master':14,'refresh_new_deadline_delta':896,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[table,base/'m68kcpu.h',base/'m68kcpu.c',rompath]},'producer_sha256':sha(Path(__file__).read_bytes()),'limits':['This primitive priority clear does not cover CLR.L bulk resets or MOVE.B priority restoration','Normal gate stops before duck/tempo/fade housekeeping; hold includes its early return']}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'operations':len(rows)}))
if __name__=='__main__':main()
