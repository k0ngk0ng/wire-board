"""Bind arena semantic work to static architecture costs; never runtime data."""
import argparse,json,re
from pathlib import Path
from reference import ROOT,sha

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT) and not out.exists()
 source=ROOT/'docs/research/md-yuyuhakusho/static/arena-assets-scenes.txt';rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes();assert sha(rom)=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 base=ROOT/'.local/md-yuyuhakusho/tools/Genesis-Plus-GX-60bee4c724859710ce05672239e5b601ad01382e/core/m68k';table=base/'m68kops.h';text=table.read_text();entries=re.findall(r'\{(m68k_op_\w+)\s*,\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(\d+)\}',text);rows=[]
 for line in source.read_text().splitlines():
  m=re.match(r'([0-9a-f]{8}) (.*)',line)
  if not m:continue
  pc=int(m[1],16)
  if not(0x20918<=pc<=0x20ade or 0x21556<=pc<=0x215f8 or 0x21690<=pc<=0x216ee):continue
  opcode=int.from_bytes(rom[pc:pc+2],'big');entry=[e for e in entries if opcode&int(e[1],16)==int(e[2],16)][-1];cost=int(entry[3]);shift=None
  if re.search(r'm68k_op_(asr|asl|lsr)_\d+_s',entry[0]):shift=(((opcode>>9)-1)&7)+1;cost+=shift*2
  rows.append({'pc':pc,'source':m[2],'handler':entry[0],'base_cycles':int(entry[3]),'fixed_cycles':cost,'immediate_shift':shift})
 assert len(rows)==225
 report={'pass':True,'scope':'Offline source provenance only; no PCs/opcodes are loaded by production work observers.','instructions':rows,'dynamic':{'short_branch_not_taken':8,'word_branch_not_taken':12,'branch_taken':10,'dbf_continue':10,'dbf_expired':14,'camera_division':'136 - 2*popcount(floor(word_sum/(2*sample_count))); divisor 1..16, zero-extended wrapped 16-bit sum; internal refresh skip before zero fixed cost','refresh_master_delay':14,'refresh_period':896},'sources':{str(f.relative_to(ROOT)):sha(f.read_bytes())for f in [source,rompath,table,base/'m68kcpu.c',base/'m68kcpu.h',Path(__file__)]}}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'pass':True,'instructions':len(rows)}))
if __name__=='__main__':main()
