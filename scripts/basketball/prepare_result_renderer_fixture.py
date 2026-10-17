"""Original result draw states plus semantic restored court-column history."""
from pathlib import Path
import json
ROOT=Path(__file__).resolve().parents[2]
def prepare(palette_variant=False,court_six=False):
 source='.local/basketball/rea/court-six/result-capture.json' if court_six else '.local/basketball/palette-timing/result-capture.json' if palette_variant else '.local/basketball/result-art/capture.json'
 prefix='result-court-six' if court_six else 'result-palette' if palette_variant else 'result'
 out=ROOT/'.local/basketball/renderer/browser';capture=json.loads((ROOT/source).read_text());directory=ROOT/'.local/nes-basket/runs'/capture.get('run','result-art-observed');hud=json.loads((out/'cases.json').read_text())['cases'][0]['snapshot']['hud'];cases=[]
 for item in capture['frames']:
  source=json.loads((directory/f"ppu-{item['emulator_frame']:05d}-semantic.json").read_text());history=[]
  for commit in source['courtCommits']:
   history=[c for c in history if c['column']%32!=commit['column']%32];history.append(commit)
  snapshot={**source['state'],'court':capture.get('court',1),'paletteID':capture.get('paletteID',2),'camera':source['camera'],'reverseSprites':source['reverseSprites'],'ballCHR':source['ballCHR'],'result':source['result'],'hud':hud}
  cases.append({'frame':item['frame'],'courtHistory':history,'snapshot':snapshot,'reference':f'/{prefix}-reference/{item["frame"]:05d}.png'})
 (out/(prefix+'-cases.json')).write_text(json.dumps({'scope':'original result-renderer inputs, not Go full-match execution','cases':cases},separators=(',',':'))+'\n')
if __name__=='__main__':prepare()
