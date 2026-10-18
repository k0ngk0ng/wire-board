"""Summarize retained original capture evidence, without changing capture state."""
import csv,hashlib,json
from collections import Counter
from pathlib import Path
from PIL import Image,ImageDraw
from capture_reference import ROOT,BASE
from static_sprites import SpriteROM

OUT=ROOT/'.local/postresult-video'

def digest(file):return hashlib.sha256(file.read_bytes()).hexdigest()
def main():
    rom=SpriteROM((BASE/'basketball.nes').read_bytes());reports=[]
    for kind in ['reward','ending']:
        path=BASE/f'runs/postresult-{kind}-observed';reference=BASE/f'runs/postresult-{kind}-reference'
        spec=json.loads((path/'scenario.json').read_text());events=[json.loads(l) for l in (path/'events.jsonl').read_text().splitlines()]
        with (path/'frames.csv').open() as f:frames={int(r['emulator_frame']):int(r['frame']) for r in csv.DictReader(f)}
        allraw=(path/'boundaries.ram').read_bytes();indices=[r['raw'] for r in events if r['raw']>=0]
        assert indices==list(range(len(indices))) and len(allraw)==len(indices)*2048
        identical=['ram.bin','injection.json','injection-before.ram','injection-after.ram']+[f'frame-{n:05d}.png' for n in spec['captures']]
        assert all((path/name).read_bytes()==(reference/name).read_bytes() for name in identical)
        spans=[];open_spans={};unmatched=[]
        ends={'fadeEnd':'fadeBegin','backgroundEnd':'backgroundBegin','messageEnd':'messageBegin','textEnd':'textBegin','rewardEnd':'rewardBegin','endingEnd':'endingBegin'}
        for e in events:
            if e['kind'] in ends.values():
                if e['kind'] in open_spans:unmatched.append(open_spans[e['kind']])
                open_spans[e['kind']]=e
            if e['kind'] in ends:
                start=open_spans.pop(ends[e['kind']],None)
                if start is None:unmatched.append(e);continue
                spans.append(dict(kind=start['kind'],scene=start['scene'],phase=start['phase'],startFrame=start['frame'],endFrame=e['frame'],startLocalFrame=frames.get(start['frame']),videoFrames=e['frame']-start['frame'],cycles=e['cycles']-start['cycles'],fade=start['fade'],message=start['message']))
        unmatched.extend(open_spans.values())
        cadence={}
        for name in ['scene1','scene7']:
            selected=[r for r in events if r['kind']==name]
            cadence[name]=dict(calls=len(selected),intervals=dict(sorted(Counter(b['frame']-a['frame'] for a,b in zip(selected,selected[1:])).items())))
        def identify(data):
            return [[p for p in range(128) if rom.chr[p*1024:(p+1)*1024]==data[i:i+1024]] for i in range(0,8192,1024)]
        snapshots=[]
        for file in sorted(path.glob('ppu-*-ppu.bin')):
            number=int(file.name.split('-')[1]);data=file.read_bytes()
            snapshots.append(dict(file=file.name,emulatorFrame=number,localFrame=frames[number],chrPages=identify(data),nametableSHA256=[hashlib.sha256(data[a:a+1024]).hexdigest() for a in range(8192,12288,1024)],palette=list(data[0x3f00:0x3f20]),sha256=digest(file)))
        irq=[]
        for i,e in enumerate(e for e in events if e['kind'] in ['irqBefore','interruptReturn']):
            file=path/f'irq-{e["frame"]:05d}-{i:03d}-ppu.bin';assert file.exists()
            irq.append(dict(file=file.name,frame=e['frame'],cycles=e['cycles'],pc=e['pc'],kind=e['kind'],chrPages=identify(file.read_bytes())))
        report=dict(kind=kind,scope='whole-console controlled postresult with first-BA40 score/progress injection; not natural earned campaign',capture=json.loads((path/'postresult-report.json').read_text()),cadence=cadence,spans=spans,unmatchedSpanBoundaries=unmatched,eventCounts=dict(Counter(e['kind'] for e in events)),boundarySnapshots=len(indices),snapshots=snapshots,interruptSnapshots=irq,sounds=[e for e in events if e['kind']=='sound'],verifiedEqualFiles=identical,artifacts={name:digest(path/name) for name in ['scenario.json','record.lua','events.jsonl','boundaries.ram','ram.bin','frames.csv','injection.json','injection-before.ram','injection-after.ram']})
        (OUT/f'{kind}-summary.json').write_text(json.dumps(report,indent=2)+'\n');reports.append(dict(kind=kind,cadence=cadence,boundaries=len(indices),snapshots=len(snapshots),interruptSnapshots=len(irq),matchedFiles=len(identical)))
    selected=[('reward',540),('reward',660),('ending',600),('ending',1200),('ending',1800),('ending',4800),('ending',6000),('ending',9000),('ending',10500),('ending',12000),('ending',13500),('ending',14500)]
    image=Image.new('RGB',(1024,780),'#303030');draw=ImageDraw.Draw(image)
    for i,(kind,frame) in enumerate(selected):
        x=i%4*256;y=i//4*260;image.paste(Image.open(BASE/f'runs/postresult-{kind}-observed/frame-{frame:05d}.png'),(x,y+20));draw.text((x+4,y+4),f'Controlled {kind}, frame {frame}',fill='white')
    image.save(OUT/'original-console-contact-sheet.png')
    print(json.dumps(reports))
if __name__=='__main__':main()
