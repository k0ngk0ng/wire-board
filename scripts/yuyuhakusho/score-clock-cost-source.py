"""Bind the fixed semantic service's arithmetic to pinned native source costs.

This is an offline provenance exporter. Its source PCs/opcodes never enter the
Go service or the portable request/response fixture.
"""
import argparse,json,re
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)
 base=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core';table=base/'m68k/m68kops.h';rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes()
 entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',table.read_text())
 declared=[('range-high',0x38b2e,8),('range-high-branch',0x38b32,10),('range-low',0x38b36,8),('range-low-branch',0x38b3a,10),('normalize-level-register',0x38b3e,8),('select-level-cache',0x38b42,8),('write-level-cache',0x38b46,14),('select-ports',0x38b4a,12),('save-mask',0x38b50,14),('mask-interrupts',0x38b52,20),('request-bus',0x38b56,20),('confirm-bus',0x38b5e,24),('confirm-bus-branch',0x38b66,10),('read-mailbox',0x38b68,16),('mailbox-branch',0x38b6e,10),('release-for-retry',0x38b70,20),('invoke-retry-delay',0x38b78,18),('repeat-acquire',0x38b7c,10),('status-before-address',0x38b7e,8),('status-branch',0x38b80,10),('write-address',0x38b82,12),('address-data-separator',0x38b86,4),('status-before-data',0x38b88,8),('status-branch',0x38b8a,10),('write-data',0x38b8c,12),('release-bus',0x38b90,20),('restore-mask',0x38b98,16),('return-service',0x38b9a,16),('retry-delay-quantum',0x38c0c,4),('return-retry-delay',0x38c2c,16)]
 rows=[]
 for name,pc,expected in declared:
  opcode=int.from_bytes(rom[pc:pc+2],'big');matches=[e for e in entries if opcode&int(e[1],16)==int(e[2],16)];assert matches,(name,hex(opcode));selected=matches[-1];actual=int(selected[3]);assert actual==expected,(name,actual,expected)
  rows.append({'semantic_operation':name,'source_pc':pc,'source_opcode':opcode,'native_handler':selected[0],'cpu_cycles':actual})
 cpu=(base/'m68k/m68kcpu.h').read_text();assert re.search(r'CYC_BCC_NOTAKE_B\s+\(\s*-2\s*\*\s*MUL\)',cpu);assert re.search(r'CYC_BCC_NOTAKE_W\s+\(\s*2\s*\*\s*MUL\)',cpu)
 assert all(rom[pc:pc+2]==bytes.fromhex('4e71')for pc in range(0x38c0c,0x38c2c,2))
 report={'pass':True,'scope':'Static architectural costs for fixed named semantic service operations, not an executable opcode/timing table','operations':rows,'short_branch_not_taken_cpu_cycles':8,'long_branch_not_taken_cpu_cycles':12,'retry_delay_quanta':16,'refresh_master':{'deadline_after_current_boundary':896,'delay':14},'Z80_memory_access_extra_master':7,'source_sha256':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [table,base/'m68k/m68kcpu.h',base/'m68k/m68kcpu.c',base/'mem68k.c',base/'genesis.c',rompath]},'producer_sha256':sha(Path(__file__).read_bytes()),'production_uses_source_addresses_or_opcodes':False}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'semantic_operations':len(rows)}))
if __name__=='__main__':main()
