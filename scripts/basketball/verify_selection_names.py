"""Audit retained selection research evidence, without executing gameplay."""
from pathlib import Path
import hashlib,json,subprocess,sys
from PIL import Image,ImageChops
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/selection-names'
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def main():
    path=ROOT/'games/basketball/selection/names.json';initial=digest(path)
    subprocess.run([sys.executable,str(ROOT/'scripts/basketball/export_selection_names.py')],check=True,cwd=ROOT)
    assert digest(path)==initial,'Exporter is not repeatable'
    d=json.loads(path.read_text());assert d['authority']=='label-evidence-not-gameplay-allowlist' and d['accepted_for_release'] is False
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==d['romSHA256']
    assert [x['id'] for x in d['courts']]==list(range(8));assert [x['id'] for x in d['teams']]==list(range(8))
    names=[]
    for team in d['teams']:
        assert [x['character'] for x in team['characters']]==list(range(4))
        for character in team['characters']:
            src=character['source'];offset=16+src['bank']*16384+int(src['pointer'],16)-0x8000
            raw=list(rom[offset:offset+4]);expected=src['encodedBytes'];assert raw==expected+[0]*(4-len(expected))
            assert ''.join(d['characterGlyphTranscriptions'][str(v)] for v in expected)==character['originalLabel']
            names.append(character['id'])
    assert len(set(names))==32
    screenshots=0
    for item in d['courts']+d['teams']:
        source=item['source'];source=source.get('naturalMenu',source)
        run=ROOT/source['path'];assert json.loads((run/'capture-complete.json').read_text())=={'status':'complete'}
        for filename,expected in source['evidenceSHA256'].items():
            assert digest(run/filename)==expected,(run,filename);screenshots+=filename.endswith('.png')
        raw=(run/'ram.bin').read_bytes();frame=source['selectedFrame'];b=raw[(frame-1)*2048:frame*2048]
        assert list(b[0x586:0x588])==source['state']['teams'] and b[0x58b]==source['state']['court']
    for filename,expected in d['sourceRecords'].items():assert digest(OUT/filename)==expected
    pairs=json.loads((OUT/'team-pairs.json').read_text())['pairs'];observed=set()
    for item in pairs:
        run=ROOT/item['path'];raw=(run/'ram.bin').read_bytes();b=raw[(item['frame']-1)*2048:item['frame']*2048]
        assert list(b[0x586:0x588])==item['state']['teams'];observed.add(tuple(b[0x586:0x588]))
    assert observed=={(a,b) for a in range(8) for b in range(8) if a!=b}
    collisions=json.loads((OUT/'member-collisions.json').read_text())['records'];assert {x['team'] for x in collisions}==set(range(8))
    for item in collisions:
        run=ROOT/item['path'];im=[Image.open(run/f"frame-{item[k]:05d}.png").convert('RGB') for k in ('beforeFrame','rejectedFrame','movedFrame')]
        assert ImageChops.difference(im[0],im[1]).getbbox() is None
        assert ImageChops.difference(im[1],im[2]).getbbox() is not None
    result=dict(status='passed',scope='original labels and original menu selection evidence; no gameplay completeness claim',courts=8,teams=8,characterNames=32,distinctOrderedTeamPairs=len(observed),teamMemberCollisionRoutes=len(collisions),boundNameScreenshots=screenshots,metadataSHA256=initial,repeatExportIdentical=True)
    (OUT/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
