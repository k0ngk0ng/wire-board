"""Static controller/prefix architecture provenance; not executable runtime data."""
import argparse,json,re
from pathlib import Path
from reference import ROOT,sha

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists()
 rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();tablepath=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k/m68kops.h';entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',tablepath.read_text());sources=[ROOT/'games/yuyuhakusho/verification/world-audio-barriers/controller-scan/vdp-and-irq-setup.txt',ROOT/'games/yuyuhakusho/verification/world-audio-barriers/controller-teamplayer-leaves/vdp-and-irq-setup.txt'];rows={}
 for path in sources:
  for line in path.read_text().splitlines():
   m=re.match(r'([0-9a-f]{8}) (.*)',line)
   if not m:continue
   pc=int(m[1],16);opcode=int.from_bytes(rom[pc:pc+2],'big');entry=[e for e in entries if opcode&int(e[1],16)==int(e[2],16)][-1];cost=int(entry[3])
   if re.search(r'm68k_op_(asl|lsr)_\d+_s',entry[0]):cost+=2*((((opcode>>9)-1)&7)+1)
   rows[pc]={'source_pc':pc,'source':m[2],'handler':entry[0],'base_cycles':int(entry[3]),'fixed_cycles':cost}
 prefix=ROOT/'games/yuyuhakusho/verification/battle-sample-irq/portable-export.json'
 report={'pass':True,'scope':'Offline source provenance only; production uses fixed semantic branches and loops, never loads this table.','controller_instructions':list(rows.values()),'prefix_costs':json.loads(prefix.read_text())['costs'],'dynamic_costs':{'short_branch_not_taken':8,'word_branch_not_taken':12,'taken':10,'dbf_continue':10,'dbf_expired':14,'successful_DBcc_condition':12,'temporary_save_internal_cycles':112,'temporary_save_base_after_refresh_skip':8},'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [*sources,rompath,tablepath,prefix,Path(__file__)]}}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'controller_instructions':len(rows)}))
if __name__=='__main__':main()
