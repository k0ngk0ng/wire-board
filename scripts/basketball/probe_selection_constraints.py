"""Natural-menu probes of team collision handling; no memory writes."""
import json
from PIL import Image, ImageChops
from capture_selection_names import OUT,ROOT,capture,key,state

def main():
    rows=[]
    prefix=[key(30,'down'),key(60,'A'),key(180,'A')]
    for first in range(8):
        for step in range(7):
            choose=350+60*first
            segments=prefix+[key(300+60*j,'right') for j in range(first)]+[key(choose,'A')]
            segments += [key(choose+80+50*j,'right') for j in range(step)]
            confirm=choose+100+50*step;segments += [key(confirm,'A')]
            end=confirm+200
            path=capture(f'team-pair-{first}-{step}',segments,end,[confirm-10,end])
            rows.append(dict(firstOrdinal=first,secondStep=step,state=state(path,end),path=str(path.relative_to(ROOT)),frame=end))
    pairs={tuple(row['state']['teams']) for row in rows}
    assert pairs=={(a,b) for a in range(8) for b in range(8) if a!=b},pairs
    (OUT/'team-pairs.json').write_text(json.dumps(dict(scope='56 natural menu routes, right navigation skips already selected team',pairs=rows),indent=2)+'\n')
    collisions=[]
    for first in range(8):
        base=json.loads((OUT/f'team-{first}.json').read_text())
        n=base['frames']+50
        segments=base['segments']+[key(n,'A'),key(n+100,'A'),key(n+160,'up'),key(n+220,'right')]
        before=n+130;rejected=n+190;moved=n+250
        path=capture(f'member-collision-{first}',segments,moved,[before,rejected,moved])
        images=[Image.open(path/f'frame-{f:05d}.png').convert('RGB') for f in (before,rejected,moved)]
        assert ImageChops.difference(images[0],images[1]).getbbox() is None,(first,'duplicate not rejected')
        assert ImageChops.difference(images[1],images[2]).getbbox() is not None,(first,'positive control did not move')
        collisions.append(dict(team=state(path,moved)['teams'][0],path=str(path.relative_to(ROOT)),beforeFrame=before,rejectedFrame=rejected,movedFrame=moved,duplicateAttempt='partner cursor 1 up toward occupied character 0',positiveControl='right moves cursor to unoccupied character 3'))
    (OUT/'member-collisions.json').write_text(json.dumps(dict(scope='all eight teams; selected character 0, partner attempts occupied 0; original screenshots byte-identical; positive movement control differs',records=collisions),indent=2)+'\n')
if __name__=='__main__':main()
