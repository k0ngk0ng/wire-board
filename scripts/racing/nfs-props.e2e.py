#!/usr/bin/env python3
"""Source-to-GLB destructible-prop E2E; all original immutable rules stay intact.

Failure cases before implementation: billboard attribute-zero false positives,
special rigid bodies admitted without the whitelist, day/night index drift,
changed route hashes or server rules, wrong expanded edge indices, static-mesh
duplication/geometry loss, lost texture pixels, ambient autoplay of destruction,
track-origin subtraction from relative CAN offsets, wrong animation time units,
missing one-shot end state, non-reproducible files, and numeric rules published
as client assets. --check-current captures the old shipped-asset failure.
"""
import hashlib,importlib.util,json,math
from pathlib import Path
import shutil,struct,subprocess,sys,unittest
from speedtools.track_data import TrackData
from speedtools.types import Vector3d

ROOT=Path(__file__).resolve().parents[2];GAME=ROOT/'.local/racing-nfs-reference/additional/game-root'
WORK=ROOT/'.local/racing-nfs-reference/props-e2e';PUBLIC=ROOT/'games/racing/assets/public/racing/v3'
CHECK='--check-current'in sys.argv
OUT=PUBLIC if CHECK else WORK/'output'
COUNTS={'France':29,'Coastal':83,'UK':108,'Park':45}
spec=importlib.util.spec_from_file_location('numeric_rules',ROOT/'scripts/racing/nfs-rules.py');rules=importlib.util.module_from_spec(spec);spec.loader.exec_module(rules)
def glb(path):
 b=path.read_bytes();n=struct.unpack_from('<I',b,12)[0];return json.loads(b[20:20+n]),b[28+n:]
def array(doc,blob,index):
 a=doc['accessors'][index];v=doc['bufferViews'][a['bufferView']];width={'SCALAR':1,'VEC3':3,'VEC4':4}[a['type']];fmt={5126:'f',5123:'H',5125:'I'}[a['componentType']];start=v.get('byteOffset',0)+a.get('byteOffset',0)
 return list(struct.iter_unpack('<'+fmt*width,blob[start:start+a['count']*width*struct.calcsize(fmt)]))
def source_records(data):
 out={};number=0;objects=list(data.objects)
 for s,segment in enumerate(data.frd.frd.segment_data):
  for c,chunk in enumerate(segment.object_chunks):
   for j,header in enumerate(chunk.objects):
    obj=objects[number]
    if int(header.type)==4:
     attribute=segment.object_attributes[header.attribute_index]
     if int(attribute.collision_type)==2:
      a=Vector3d.from_frd_int3(attribute.location);p=header.location
      if math.dist([a.x,a.y,a.z],[p.x,p.y,p.z])>.01:raise AssertionError('unmatched physical attribute')
      out[(s,c,j)]=(number,header,obj,attribute)
    number+=1
 return out

class NativeProps(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  if not CHECK:
   if WORK.exists():shutil.rmtree(WORK)
   WORK.mkdir(parents=True)
   command=[sys.executable,'-B',str(ROOT/'scripts/racing/nfs-convert.py'),'--game-root',str(GAME),'--out',str(OUT),'--cars','P911','FM50','LDIA','MCF1','CCOR','--tracks',*COUNTS]
   result=subprocess.run(command,cwd=ROOT,text=True,capture_output=True);(WORK/'conversion.log').write_text(result.stdout+result.stderr)
   if result.returncode:raise AssertionError(result.stderr)
  else:WORK.mkdir(parents=True,exist_ok=True)

 def test_exact_normal_whitelist_stable_identity_and_day_night_source_mapping(self):
  for name,count in COUNTS.items():
   track=name.lower();catalog=json.loads((OUT/f'props/{track}.json').read_text())
   self.assertEqual(catalog['unit'],'metres');self.assertEqual(len(catalog['props']),count)
   native=[source_records(TrackData(GAME/'Data/Tracks'/name,GAME,night=n))for n in [False,True]]
   self.assertEqual(set(native[0]),set(native[1]));self.assertEqual(len(native[0]),count)
   for i,(key,record)in enumerate(native[0].items()):
    p=catalog['props'][i];self.assertEqual(p['index'],i);self.assertEqual(p['id'],f'{track}:s{key[0]}:c{key[1]}:o{key[2]}');self.assertEqual(p['collisionType'],2)
    self.assertEqual(p['source']['dayObject'],record[0]);self.assertEqual(p['source']['nightObject'],native[1][key][0])
    self.assertEqual(p['source']['segment'],key[0]);self.assertEqual(p['source']['chunk'],key[1]);self.assertEqual(p['source']['localObject'],key[2])

 def test_original_route_and_rules_unchanged_and_expanded_edges_are_exact(self):
  for name in COUNTS:
   track=name.lower();catalog=json.loads((OUT/f'props/{track}.json').read_text());route=json.loads((OUT/f'tracks/{track}.route.json').read_text())
   self.assertEqual((OUT/f'tracks/{track}.route.json').read_bytes(),(PUBLIC/f'tracks/{track}.route.json').read_bytes())
   self.assertEqual(catalog['geometry'],route['geometryHash'])
   frozen_path=ROOT/f"games/racing/server/data/rules/{route['geometryHash']}.json";frozen_bytes=frozen_path.read_bytes();frozen=json.loads(frozen_bytes)
   self.assertEqual(catalog['ruleSha256'],hashlib.sha256(frozen_bytes).hexdigest());self.assertEqual(rules.convert(route),frozen)
   numeric=json.loads((OUT/f'rules/{track}.props.json').read_text());self.assertEqual(numeric['unit'],'decimetres');self.assertEqual(numeric['catalogHash'],catalog['catalogHash'])
   cursor=0;edge_map={}
   for ordinal,b in enumerate(route['barriers']):
    count=1 if b['kind']=='wall'else (0 if b.get('collisionType')==120 else len(rules.hull(b.get('vertices',[]))))
    edge_map[ordinal]=list(range(cursor,cursor+count));cursor+=count
   self.assertEqual(cursor,len(frozen['barriers']))
   all_edges=[]
   for i,p in enumerate(catalog['props']):
    barrier=route['barriers'][p['routeBarrier']];self.assertEqual(barrier['object'],p['source']['dayObject']);self.assertEqual(barrier['collisionType'],2)
    self.assertEqual(p['ruleEdges'],edge_map[p['routeBarrier']]);self.assertGreater(len(p['ruleEdges']),0)
    self.assertEqual(numeric['props'][i]['ruleEdges'],p['ruleEdges']);all_edges.extend(p['ruleEdges'])
    for edge in p['ruleEdges']:self.assertEqual(frozen['barriers'][edge]['kind'],'object')
   self.assertEqual(len(all_edges),len(set(all_edges)))

 def test_individual_nodes_keep_original_geometry_pixels_and_one_shot_relative_can_clips(self):
  for name in COUNTS:
   track=name.lower();catalog=json.loads((OUT/f'props/{track}.json').read_text())
   for night in [False,True]:
    suffix='-night'if night else '';doc,blob=glb(OUT/f'tracks/{track}{suffix}.glb');old,_=glb(PUBLIC/f'tracks/{track}{suffix}.glb')
    self.assertEqual(sum(doc['accessors'][p['indices']]['count']//3 for m in doc['meshes']for p in m['primitives']),sum(old['accessors'][p['indices']]['count']//3 for m in old['meshes']for p in m['primitives']))
    self.assertEqual(sorted((tuple(x['extras']['sourceDimensions']),x['extras']['pixelSha256'])for x in doc['images']),sorted((tuple(x['extras']['sourceDimensions']),x['extras']['pixelSha256'])for x in old['images']))
    native=source_records(TrackData(GAME/'Data/Tracks'/name,GAME,night=night));nodes={n.get('name'):i for i,n in enumerate(doc['nodes'])};clips={c['name']:c for c in doc['animations']}
    self.assertEqual(doc['extras']['propsCatalogHash'],catalog['catalogHash'])
    for prop in catalog['props']:
     key=tuple(prop['source'][k]for k in ['segment','chunk','localObject']);_,_,obj,_=native[key];node=doc['nodes'][nodes[prop['node']]];motion=doc['nodes'][nodes[prop['motionNode']]]
     self.assertEqual(node['extras']['propIndex'],prop['index']);self.assertEqual(node['extras']['propId'],prop['id']);self.assertEqual(motion.get('translation',[0,0,0]),[0,0,0]);self.assertEqual(motion.get('rotation',[0,0,0,1]),[0,0,0,1])
     mesh=doc['nodes'][nodes[f"prop_{prop['index']}_mesh"]];actual=[]
     for primitive in doc['meshes'][mesh['mesh']]['primitives']:actual.extend(array(doc,blob,primitive['attributes']['POSITION']))
     source_positions={(-v.location.x,v.location.y,v.location.z)for v in obj.mesh.vertices}
     self.assertTrue(set(actual)<=source_positions)
     for speed,action in zip(['low','high'],obj.actions):
      clip=clips[prop['clips'][speed]];self.assertEqual(clip['extras']['trigger'],'collision');self.assertFalse(clip['extras']['loop']);self.assertTrue(clip['extras']['clampWhenFinished'])
      samples={channel['target']['path']:clip['samplers'][channel['sampler']]for channel in clip['channels']}
      for channel in clip['channels']:self.assertEqual(channel['target']['node'],nodes[prop['motionNode']])
      times=[v[0]for v in array(doc,blob,samples['translation']['input'])];self.assertEqual(times,[i*action.animation.delay/64 for i in range(action.animation.length)])
      locations=array(doc,blob,samples['translation']['output'])
      for a,b in zip(locations,action.animation.locations):
       for got,expected in zip(a,[-b.x,b.y,b.z]):self.assertAlmostEqual(got,expected,places=5)
      rotations=array(doc,blob,samples['rotation']['output'])
      for got,raw in zip(rotations,action.animation.quaternions):
       expected=[-raw.x,raw.y,raw.z,raw.w];length=math.sqrt(sum(x*x for x in expected));self.assertAlmostEqual(abs(sum(a*b/length for a,b in zip(got,expected))),1,places=5)

 def test_other_assets_and_old_server_rules_are_byte_preserved_and_numeric_catalog_is_not_client_payload(self):
  if CHECK:self.skipTest('old published baseline contains no conversion output')
  for path in (OUT/'cars').glob('*.glb'):self.assertEqual(path.read_bytes(),(PUBLIC/'cars'/path.name).read_bytes())
  for path in (OUT/'tracks').iterdir():
   if path.suffix!='.glb':self.assertEqual(path.read_bytes(),(PUBLIC/'tracks'/path.name).read_bytes())
  manifest=json.loads((OUT/'manifest.json').read_text());self.assertFalse(any(a['path'].startswith('rules/')for a in manifest['assets']))
  for name in COUNTS:self.assertTrue(any(a['path']==f'props/{name.lower()}.json'for a in manifest['assets']))

if __name__=='__main__':
 result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(NativeProps))
 (WORK/('initial-result.json'if CHECK else'result.json')).write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,'failures':[str(t)for t,_ in result.failures],'errors':[str(t)for t,_ in result.errors]},indent=2)+'\n')
 sys.exit(0 if result.wasSuccessful()else 1)
