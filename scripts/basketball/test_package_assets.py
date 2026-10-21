"""Run with python -B; temporary files stay in this worktree."""
import importlib.util,json,hashlib,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
def load(name):
 spec=importlib.util.spec_from_file_location(name,ROOT/'scripts/basketball'/f'{name.replace("_","-")}.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m
pack=load('package_assets')
class PackagingTests(unittest.TestCase):
 def setUp(self):
  parent=ROOT/'.local/asset-package-tests';parent.mkdir(parents=True,exist_ok=True)
  self.tmp=tempfile.TemporaryDirectory(dir=parent);self.root=Path(self.tmp.name)
 def tearDown(self):self.tmp.cleanup()
 def fixture(self):
  r=self.root;png=bytes.fromhex('89504e470d0a1a0a')+b'test-image'
  def put(path,data):
   p=r/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data);return hashlib.sha256(data).hexdigest()
  def doc(path,d):put(path,(json.dumps(d)+'\n').encode())
  def manifest(base,name='manifest.json'):
   digest=put(base+'/tile.png',png)
   doc(base+'/'+name,{'source':{'rom_sha256':pack.ROM_SHA,'privateSource':'secret location'},'accepted_for_release':False,'assets':{'tile.png':{'sha256':digest}},'verification':{'log':'research.log'}})
  for group in ['court','raster','results']:manifest('.local/basketball/'+group)
  for group in ['static-atlas','static-objects']:manifest('.local/basketball/'+group,'atlas.json')
  manifest('.local/postresult-assets');manifest('.local/basketball/selection-assets')
  courts=[]
  for i in range(8):
   base=f'.local/basketball/courts/{i}';manifest(base)
   courts.append({'court':i,'manifest':f'{i}/manifest.json','sha256':hashlib.sha256((r/base/'manifest.json').read_bytes()).hexdigest()})
  doc('.local/basketball/courts/manifest.json',{'source':{'rom_sha256':pack.ROM_SHA},'courts':courts})
  wav=b'RIFF'+(8).to_bytes(4,'little')+b'WAVE';digest=put('.local/basketball/audio/export/sound-003.wav',wav)
  doc('games/basketball/assets/audio-catalog.json',{'source_rom_sha256':pack.ROM_SHA,'effects':[{'id':3,'wav':'sound-003.wav'}]})
  doc('.local/basketball/audio/export/manifest.json',{'rom_sha256':pack.ROM_SHA,'sounds':[{'id':3,'wav':'sound-003.wav','sha256':{'sound-003.wav':digest}}]})
  mp3=b'ID3test-delivery';digest=put('.local/basketball/audio/export/music-075.mp3',mp3)
  doc('games/basketball/assets/audio-playback.json',{'accepted_for_release':False,'effects':{},'music':{'75':{'file':'music-075.mp3','mime_type':'audio/mpeg','distribution_sha256':digest,'encoding_manifest':'encoding.json','source_manifest':'music-source.json','wav':'music-075.wav','loop':False}}})
  doc('.local/basketball/audio/export/encoding.json',{'tracks':[{'id':75,'file':'music-075.mp3','sha256':digest}]})
  doc('.local/basketball/audio/export/music-source.json',{'rom_sha256':pack.ROM_SHA})
  for filename in ['basketball.nes','sound-003.nsf','music-075.wav','research.html','driver-ram.bin','capture.log']:put('.local/basketball/audio/export/'+filename,b'private')
 def fixture_music76(self):
  self.fixture();base=self.root/'.local/basketball/audio/export'
  mp3=b'ID3verified-compound-delivery';master=b'private-source-bound-master'
  (base/'music-076-intro-loop.mp3').write_bytes(mp3);(base/'music-076-intro-loop.wav').write_bytes(master)
  timing={'sample_rate':44100,'loop_start_sample':5465255,'loop_end_sample':32515748,'loop_samples':27050493,'decoded_samples':32515748}
  delivery=dict(timing,id=76,status='offline-asset-verified',file='music-076-intro-loop.mp3',sha256=hashlib.sha256(mp3).hexdigest(),master='music-076-intro-loop.wav',master_sha256=hashlib.sha256(master).hexdigest(),source={'settings':{'romSHA256':pack.ROM_SHA}})
  (base/'music76-delivery-manifest.json').write_text(json.dumps(delivery))
  p=self.root/'games/basketball/assets/audio-playback.json';playback=json.loads(p.read_text())
  playback['music']['76']=dict(timing,file=delivery['file'],wav=delivery['master'],sha256=delivery['master_sha256'],distribution_sha256=delivery['sha256'],mime_type='audio/mpeg',loop=True,source_manifest='music76-delivery-manifest.json',encoding_manifest='music76-delivery-manifest.json')
  p.write_text(json.dumps(playback));return p,base/'music76-delivery-manifest.json',timing
 def test_music76_compound_metadata_is_public_and_bound_to_source(self):
  _,_,timing=self.fixture_music76();pack.build_package(self.root,self.root/'compound')
  public=self.root/'compound/public/basketball/v1/audio';manifest=json.loads((public/'audio-playback.json').read_text())
  entry=manifest['music']['76'];self.assertEqual({key:entry[key] for key in timing},timing)
  self.assertEqual(entry['file'],'music-076-intro-loop.mp3');self.assertTrue(entry['loop'])
  self.assertEqual(set(entry),set(timing)|{'file','mime_type','distribution_sha256','loop'})
  self.assertFalse((public/'music-076-intro-loop.wav').exists());self.assertFalse((public/'music76-delivery-manifest.json').exists())
 def test_music76_rejects_malformed_or_unbound_timing(self):
  cases={'missing':lambda e:e.pop('loop_start_sample'),'bool':lambda e:e.update(loop_start_sample=True),
         'fraction':lambda e:e.update(loop_samples=27050493.5),'rate':lambda e:e.update(sample_rate=48000),
         'negative':lambda e:e.update(loop_start_sample=-1),'zero_prefix':lambda e:e.update(loop_start_sample=0),
         'end':lambda e:e.update(loop_end_sample=e['decoded_samples']-1),
         'period':lambda e:e.update(loop_samples=e['loop_samples']-1),
         'decoded':lambda e:e.update(decoded_samples=e['decoded_samples']+1),
         'finite':lambda e:e.update(loop=False),'wrong_file':lambda e:e.update(file='music-076-loop.mp3'),
         'master_hash':lambda e:e.update(sha256='0'*64)}
  for name,corrupt in cases.items():
   with self.subTest(case=name):
    path,_,_=self.fixture_music76();doc=json.loads(path.read_text());corrupt(doc['music']['76']);path.write_text(json.dumps(doc))
    with self.assertRaises(ValueError):pack.build_package(self.root,self.root/('bad76-'+name))
  for name in ['mismatched_source','missing_source_identity','wrong_source_master','unverified_source']:
   with self.subTest(case=name):
    _,path,_=self.fixture_music76();doc=json.loads(path.read_text())
    if name=='mismatched_source':doc['loop_start_sample']+=1;doc['loop_samples']-=1
    elif name=='missing_source_identity':doc.pop('source')
    elif name=='wrong_source_master':doc['master_sha256']='0'*64
    else:doc['status']='rejected-approved-anchor'
    path.write_text(json.dumps(doc))
    with self.assertRaises(ValueError):pack.build_package(self.root,self.root/('bad76-'+name))
 def test_reproducible_allowlist_and_private_provenance(self):
  self.fixture();a=pack.build_package(self.root,self.root/'out-a');b=pack.build_package(self.root,self.root/'out-b')
  self.assertEqual(a,b);self.assertEqual((self.root/'out-a/asset-package.tar.gz').read_bytes(),(self.root/'out-b/asset-package.tar.gz').read_bytes())
  public=[e['urlPath'] for e in a['entries']];self.assertIn('/basketball/v1/audio/music-075.mp3',public)
  self.assertFalse(any(x.endswith(('.nes','.nsf','.html','.bin','.log')) or 'music-075.wav' in x for x in public))
  doc=json.loads((self.root/'out-a/public/basketball/v1/graphics/courts/manifest.json').read_text())
  for row in doc['courts']:self.assertEqual(row['sha256'],hashlib.sha256((self.root/'out-a/public/basketball/v1/graphics/courts'/row['manifest']).read_bytes()).hexdigest())
  self.assertNotIn('privateSource',(self.root/'out-a/public/basketball/v1/graphics/court/manifest.json').read_text())
  self.assertTrue((self.root/'out-a/provenance.json').is_file());pack.verify_package(self.root/'out-a',self.root)
 def test_invalid_manifest_digest_or_rom(self):
  for corrupt in ['sha','rom']:
   with self.subTest(corrupt=corrupt):
    self.fixture();p=self.root/'.local/basketball/court/manifest.json';d=json.loads(p.read_text())
    if corrupt=='sha':d['assets']['tile.png']['sha256']='0'*64
    else:d['source']['rom_sha256']='0'*64
    p.write_text(json.dumps(d))
    with self.assertRaises(ValueError):pack.build_package(self.root,self.root/('out-'+corrupt))
 def test_bad_paths_and_nested_symlinks(self):
  for value in ['../secret','/etc/passwd','https://bad/x','a\\b','a/../b','a//b','%2e%2e/x','x?y','x#z']:
   with self.subTest(value=value),self.assertRaises(ValueError):pack.safe_relative(value)
  self.fixture();p=self.root/'.local/basketball/court/tile.png';p.unlink();p.symlink_to(self.root/'.local/basketball/raster/tile.png')
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'out')
  p.unlink();self.fixture();directory=self.root/'.local/basketball/court';saved=directory.with_name('court-saved');directory.rename(saved);directory.symlink_to(saved,target_is_directory=True)
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'out-dir')
 def test_output_boundary_existing_and_tamper(self):
  self.fixture()
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root.parent/'outside')
  (self.root/'link').symlink_to(self.root/'.local',target_is_directory=True)
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'link/out')
  pack.build_package(self.root,self.root/'out')
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'out')
  (self.root/'out/public/basketball/v1/audio/music-075.mp3').write_bytes(b'changed')
  with self.assertRaises(ValueError):pack.verify_package(self.root/'out',self.root)
 def test_unhashed_or_forbidden_manifest_reference(self):
  self.fixture();p=self.root/'.local/basketball/court/manifest.json';d=json.loads(p.read_text());d['extra']={'file':'secret.nes','sha256':'0'*64};p.write_text(json.dumps(d))
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'out')
  self.fixture();p=self.root/'games/basketball/assets/audio-playback.json';d=json.loads(p.read_text());d['music']['77']=d['music'].pop('75');p.write_text(json.dumps(d))
  with self.assertRaises(ValueError):pack.build_package(self.root,self.root/'wrong-track')
 def test_publish_plan_never_connects_and_collision_preflight(self):
  publish=load('publish_assets');self.fixture();pack.build_package(self.root,self.root/'out')
  plan=publish.publication_plan(self.root/'out',self.root);self.assertEqual(plan['writesPerformed'],0)
  class Transport:
   def __init__(self):self.puts=[]
   def get(self,path,cdn=False):
    if path.endswith('music-075.mp3'):return b'collision','audio/mpeg'
    return None,None
   def put(self,path,data,mime):self.puts.append(path)
  client=Transport()
  with self.assertRaises(ValueError):publish.publish_verified(self.root/'out',client,self.root)
  self.assertEqual(client.puts,[])
  class Store:
   def __init__(self):self.objects={};self.readbacks=[];self.corrupt_cdn=False
   def get(self,path,cdn=False):
    self.readbacks.append((path,cdn));data,mime=self.objects.get(path,(None,None))
    return (b'bad-cdn',mime) if cdn and self.corrupt_cdn else (data,mime)
   def put(self,path,data,mime):
    if path in self.objects:raise AssertionError('immutable overwrite')
    self.objects[path]=(data,mime)
  client=Store();result=publish.publish_verified(self.root/'out',client,self.root)
  self.assertEqual(result['writesPerformed'],plan['objects']);self.assertEqual(len([r for r in client.readbacks if r[1]]),plan['objects'])
  self.assertEqual(publish.publish_verified(self.root/'out',client,self.root)['writesPerformed'],0)
  client.corrupt_cdn=True
  with self.assertRaises(ValueError):publish.publish_verified(self.root/'out',client,self.root)
if __name__=='__main__':unittest.main()
