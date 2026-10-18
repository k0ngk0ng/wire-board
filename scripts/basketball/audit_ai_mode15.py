"""Bounded audit of the anomalous original mode15 alias; not gameplay proof."""
import csv,gzip,hashlib,json
from pathlib import Path
from verify_ai_command_attack import setup
from verify_ai_mode5_plan11 import ROOT,ROM,PRG,machine

def execute(cpu):
    cpu.memory[0x8000:0xc000]=PRG[5*16384:6*16384]
    cpu.pc=0xa05d;cpu.x=0;cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
    selected=None;history=[]
    for steps in range(20000):
        if cpu.pc==0x7000:return dict(status='returned',steps=steps,selectedIndex=selected,mode=cpu.memory[0x672],parameter=cpu.memory[0x682],action=cpu.memory[0x512])
        if cpu.pc==0xa666:selected=cpu.y
        history=(history+[cpu.pc])[-12:]
        try:cpu.step()
        except Exception as e:return dict(status='unsupported-write-or-execution',pc=cpu.pc,error=repr(e),history=history,selectedIndex=selected)
    return dict(status='instruction-limit',pc=cpu.pc,history=history,selectedIndex=selected)

def main():
    out=ROOT/'.local/basketball/rea/ai-mode15-audit';out.mkdir(parents=True,exist_ok=True)
    samples={}
    for high in range(256):
        seed=(high<<24)|0x3456
        cpu=machine(setup(0,15,False,(0,100),seed=seed));first=execute(cpu)
        if first['status']!='returned':raise RuntimeError(first)
        index=first['selectedIndex']
        if index not in samples:samples[index]=dict(seed=seed,first=first,next=execute(cpu))
    bank=PRG[5*16384:6*16384];read=lambda a:bank[a-0x8000];word=lambda a:read(a)+256*read(a+1)
    command=[read(word(0x9467+2*t)+i) for t in range(6) for i in range(12)]
    computer=[read(word(word(0x967e+2*t)+2*s)+i) for t in range(8) for s in range(7) for i in range(6)]
    resume=json.loads(gzip.decompress((ROOT/'games/basketball/engine/testdata/ai-command-resume-function.json.gz').read_bytes()))
    csvpath=ROOT/'.local/nes-basket/runs/ai-shot-observation/ai.csv';rows=list(csv.DictReader(csvpath.open()))
    matches=[]
    # Byte-pattern candidates only: data and instruction alignment are not proven.
    for bankid in range(8):
        data=PRG[bankid*16384:(bankid+1)*16384]
        for value in [15,143]:
            for opcode in [0x8d,0x9d,0x99]:
                for address in range(0x672,0x676):
                    needle=bytes([0xa9,value,opcode,address&255,address>>8]);start=0
                    while True:
                        pos=data.find(needle,start)
                        if pos<0:break
                        matches.append(dict(bank=bankid,cpu=0x8000+pos,value=value,pattern=needle.hex()));start=pos+1
    result=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),scope='first mode15 original dispatch and immediate next call from controlled states; bounded known tables, retained default original AI trace and exact immediate-store byte patterns only',aliasEntry=hex(word(0xa09c+15*2)),index=(15&15)-4,maskAddress=hex(0xa691+11),mask=read(0xa691+11),offsetAddress=hex(0xa694+11),offset=read(0xa694+11),samples=[samples[k] for k in sorted(samples)],knownCommandModes=sorted(set(command)),knownComputerModes=sorted(set(computer)),commandResumeHas15=any((r['afterMode']&127)==15 for r in resume),retainedTrace=dict(path=str(csvpath.relative_to(ROOT)),sha256=hashlib.sha256(csvpath.read_bytes()).hexdigest(),calls=len(rows),mode15Calls=sum((int(r['mode_before'])&127)==15 for r in rows)),immediateStorePatternCandidates=matches,ordinaryCallerProven=False,unreachableProven=False,limitations=['No exhaustive indirect-write/dataflow proof.','A table entry or forced state is not a valid menu route.','Undefined device effects in malformed continuation are recorded, never skipped or patched.','No Go handler or claim of full AI completion is added.'])
    (out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
