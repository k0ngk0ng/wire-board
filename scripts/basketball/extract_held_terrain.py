"""Keep the original held-ball terrain regression small and source-bound."""
import gzip
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
DATA=ROOT/'games/basketball/engine/testdata'


def main():
    ball_path=DATA/'ball.json';world_path=DATA/'world-actions.json'
    balls=json.loads(ball_path.read_text())
    if world_path.exists():worlds=json.loads(world_path.read_text())
    else:
        world_path=world_path.with_suffix('.json.gz')
        with gzip.open(world_path,'rt') as f:worlds=json.load(f)
    ball=next(r for r in balls if r['scenario']=='cpu-match' and r['frame']==3653)
    world=next(r for r in worlds if r['frame']==3652)
    assert ball['before']['holder']==world['after']['holder']==2
    carrier=world['after']['actors'][2]['player']
    assert carrier['body']['position']['height']==-65536
    result=dict(worldFrame=world['frame'],ballFrame=ball['frame'],carrier=carrier,before=ball['before'],after=ball['after'])
    target=DATA/'held-terrain.json';target.write_text(json.dumps(result,indent=2)+'\n')
    source=dict(source_is_original_execution=True,description='Exact ball action boundary and its containing world-step owner state; one video-frame crossing',sources=[dict(path=str(p.relative_to(ROOT)),sha256=hashlib.sha256(p.read_bytes()).hexdigest()) for p in (ball_path,world_path)],sha256=hashlib.sha256(target.read_bytes()).hexdigest())
    target.with_name('held-terrain-source.json').write_text(json.dumps(source,indent=2)+'\n')
    print('held-terrain',target.stat().st_size)


if __name__=='__main__':main()
