"""Compact existing original PCM decisions for repeatable independent tests."""
from pathlib import Path
import argparse,gzip,json
from reference import ROOT,sha

def load(path):return json.loads(gzip.decompress(path.read_bytes()))
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private',type=Path,required=True);p.add_argument('--study',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
 private=a.private.resolve();study=a.study.resolve();out=a.out.resolve();assert all(x.is_relative_to(ROOT)for x in [private,study,out]);out.mkdir(parents=True,exist_ok=True)
 manifest=json.loads((private/'pcm-arbitration-private-manifest.json').read_text());source=private/'pcm-arbitration-private.json.gz';raw=gzip.decompress(source.read_bytes());assert sha(raw)==manifest['fixture_json_sha256']and sha(source.read_bytes())==manifest['fixture_gzip_sha256']
 payload=json.loads(raw)
 target=out/'pcm-arbitration-private.json.gz';target.write_bytes(source.read_bytes());m=dict(manifest);m.update(fixture_path=str(target.relative_to(ROOT)),original_fixture_path=str(source.relative_to(ROOT)));(out/'pcm-arbitration-private-manifest.json').write_text(json.dumps(m,indent=2)+'\n')
 summary=json.loads((study/'capture.json').read_text());assert summary['pass'];calls=[];sources=[]
 for case in summary['cases']:
  path=study/(case['name']+'-observed.json.gz');body=load(path);sources.append({'path':str(path.relative_to(ROOT)),'sha256':sha(path.read_bytes())})
  for index,item in enumerate(body['dispatches']):
   if not item['samples']:continue
   assert len(item['samples'])==1
   sample=item['samples'][0];accept=[r for r in item['branches']if r['pc']in('0x37dd8','0x37f1e','0x37fce')];assert len(accept)<=1
   destination={'0x37dd8':1,'0x37f1e':2,'0x37fce':3}[accept[0]['pc']]if accept else 0
   view={key:sample[key]for key in('mode_flags','slot_flags','slot_busy','slot_choice_bits','duck_mode')};view['slot_busy']=view['slot_busy']or[0,0]
   row={'name':case['name']+'-'+str(index),'code':item['code'],'command':{'Flags':sample['flags'],'RoutingOrPan':sample['routing']},'view':{'ModeFlags':view['mode_flags'],'SlotFlags':view['slot_flags'],'SlotBusy':view['slot_busy'],'ChoiceBits':view['slot_choice_bits'],'DuckMode':view['duck_mode']},'expected':{'Destination':destination,'ChoiceBits':item['after']['slot_choice_bits'],'DuckMode':item['after']['duck_mode'],'StartDual':destination in(2,3)and sample['mode_flags']&1==0},'source_ordinal':item['ordinal'],'source_pc':sample['pc'],'branch_pcs':[x['pc']for x in item['branches']]}
   calls.append(row)
 raw=(json.dumps({'schema':1,'calls':calls},separators=(',',':'))+'\n').encode();target=out/'pcm-arbitration-sampled.json.gz';target.write_bytes(gzip.compress(raw,mtime=0))
 report={'pass':True,'scope':'Observed real queue dispatch decisions in declared private pair/control streams; not natural gameplay reachability','records':len(calls),'frames_match_original_core':summary['all_original_observed_frames_equal'],'identity':summary['identity'],'fixture_path':str(target.relative_to(ROOT)),'fixture_json_sha256':sha(raw),'fixture_gzip_sha256':sha(target.read_bytes()),'producer_sha256':sha(Path(__file__).read_bytes()),'sources':sources,'limits':['StartDual in this sampled fixture is derived from observed accepted branch plus the recorded pre-branch mode; private boundary fixture separately observes exact37f48/37ff8 PCs']}
 (out/'pcm-arbitration-sampled-manifest.json').write_text(json.dumps(report,indent=2)+'\n');print('private',len(payload['calls']),'sampled',len(calls),flush=True)
if __name__=='__main__':main()
