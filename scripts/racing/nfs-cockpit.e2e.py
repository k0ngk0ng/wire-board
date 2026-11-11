#!/usr/bin/env python3
"""Original-resource E2E for the five real NFS4 interior meshes and camera eyes.

Failure cases before implementation: substitute or empty dashboard geometry,
lost exterior triangles, duplicated ground offsets, a generic camera eye,
wrong car/dash texture mapping, changed source pixels, untraceable VIV members,
and a non-reproducible export. All outputs stay in the repository.
"""
import hashlib
import importlib.util
from io import BytesIO
import json
from pathlib import Path
import shutil
import struct
import sys
import unittest
from PIL import Image
from speedtools.car_data import CarData
from speedtools.utils import create_pil_image

ROOT=Path(__file__).resolve().parents[2]
GAME=ROOT/'.local/racing-nfs-reference/additional/game-root'
WORK=ROOT/'.local/racing-nfs-reference/cockpit-e2e'
CARS={'P911':1127,'FM50':995,'LDIA':1077,'MCF1':907,'CCOR':1044}
spec=importlib.util.spec_from_file_location('nfs_convert',ROOT/'scripts/racing/nfs-convert.py')
converter=importlib.util.module_from_spec(spec)
spec.loader.exec_module(converter)

def glb(path):
 data=path.read_bytes();length=struct.unpack_from('<I',data,12)[0]
 doc=json.loads(data[20:20+length]);size=struct.unpack_from('<I',data,20+length)[0]
 return doc,data[28+length:28+length+size]

def triangles(doc,index):
 node=doc['nodes'][index]
 return (sum(doc['accessors'][p['indices']]['count']//3 for p in doc['meshes'][node['mesh']]['primitives']) if 'mesh'in node else 0)+sum(triangles(doc,n)for n in node.get('children',[]))

class NativeCockpit(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  if WORK.exists():shutil.rmtree(WORK)
  (WORK/'output/cars').mkdir(parents=True)
  cls.metadata={}
  for ident in CARS:cls.metadata[ident],_=converter.car_export(GAME,ident,WORK/'output')

 def test_original_exterior_and_real_cockpit_have_separate_exact_triangle_counts(self):
  for ident,exterior in CARS.items():
   with self.subTest(car=ident):
    source=CarData(GAME/'Data/Cars'/ident,GAME);parts=list(source.interior)
    native=sum(len(polygon.face)-2 for part in parts for polygon in part.mesh.polygons)
    doc,_=glb(WORK/f'output/cars/{ident.lower()}.glb')
    nodes={node.get('name'):i for i,node in enumerate(doc['nodes'])}
    self.assertIn('exterior',nodes)
    self.assertIn('cockpit',nodes)
    self.assertEqual(triangles(doc,nodes['exterior']),exterior)
    self.assertEqual(triangles(doc,nodes['cockpit']),native)
    self.assertGreater(native,1000)
    self.assertEqual({doc['nodes'][i]['name']for i in doc['nodes'][nodes['cockpit']]['children']},{p.name for p in parts})

 def test_camera_is_the_original_dash_fce_eye_with_one_ground_translation(self):
  for ident in CARS:
   with self.subTest(car=ident):
    source=CarData(GAME/'Data/Cars'/ident,GAME);location=source.interior_camera.location
    doc,_=glb(WORK/f'output/cars/{ident.lower()}.glb')
    expected=[-location.x,location.y+doc['extras']['groundOffset'],location.z]
    self.assertEqual(doc['extras']['cockpitCamera'],expected)
    vehicle=next(n for n in doc['nodes']if n.get('name')=='vehicle')
    self.assertEqual(vehicle['translation'],[0,doc['extras']['groundOffset'],0])

 def test_cockpit_mesh_vertices_and_texture_pixels_are_original(self):
  for ident in CARS:
   with self.subTest(car=ident):
    source=CarData(GAME/'Data/Cars'/ident,GAME)
    doc,blob=glb(WORK/f'output/cars/{ident.lower()}.glb')
    root=next(n for n in doc['nodes']if n.get('name')=='cockpit')
    images={}
    for i,image in enumerate(doc['images']):
     view=doc['bufferViews'][image['bufferView']]
     pixels=Image.open(BytesIO(blob[view['byteOffset']:view['byteOffset']+view['byteLength']])).convert('RGBA')
     images[i]=(pixels.size,hashlib.sha256(pixels.tobytes()).hexdigest())
    dash=create_pil_image(next(iter(source.interior_materials)).image).convert('RGBA')
    expected=(dash.size,hashlib.sha256(dash.tobytes()).hexdigest())
    self.assertIn(expected,images.values())
    originals={part.name:part for part in source.interior}
    for index in root['children']:
     node=doc['nodes'][index];part=originals[node['name']]
     self.assertEqual(node['translation'],[-part.location.x,part.location.y,part.location.z])
     native_positions={(-v.location.x,v.location.y,v.location.z)for v in part.mesh.vertices}
     for primitive in doc['meshes'][node['mesh']]['primitives']:
      material=doc['materials'][primitive['material']]
      texture_info=material['pbrMetallicRoughness'].get('baseColorTexture')
      if texture_info:
       texture=doc['textures'][texture_info['index']]
       source_image=texture.get('source',texture.get('extensions',{}).get('EXT_texture_webp',{}).get('source'))
       self.assertEqual(images[source_image],expected)
      else:
       self.assertEqual(material.get('alphaMode'),'BLEND')
       self.assertTrue(any(p.transparent for p in part.mesh.polygons))
       self.assertEqual(material['extras']['sourceTexture'],'dash00.tga')
      accessor=doc['accessors'][primitive['attributes']['POSITION']];view=doc['bufferViews'][accessor['bufferView']]
      start=view.get('byteOffset',0)+accessor.get('byteOffset',0)
      for vertex in struct.iter_unpack('<fff',blob[start:start+accessor['count']*12]):self.assertIn(vertex,native_positions)

 def test_dash_source_members_are_hashed_and_repeated_exports_match(self):
  (WORK/'repeat/cars').mkdir(parents=True)
  for ident in CARS:
   with self.subTest(car=ident):
    source=CarData(GAME/'Data/Cars'/ident,GAME);viv=(GAME/'Data/Cars'/ident/'car.viv').read_bytes()
    doc,_=glb(WORK/f'output/cars/{ident.lower()}.glb')
    expected=[]
    for name in ['dash.fce','dash00.tga']:
     entry=next(e for e in source.viv.viv.entries if e.name.lower()==name)
     data=viv[entry.offset:entry.offset+entry.length]
     expected.append({'member':name,'bytes':entry.length,'sha256':hashlib.sha256(data).hexdigest()})
    self.assertEqual(doc['extras']['cockpitSource'],expected)
    converter.car_export(GAME,ident,WORK/'repeat')
    self.assertEqual((WORK/f'output/cars/{ident.lower()}.glb').read_bytes(),(WORK/f'repeat/cars/{ident.lower()}.glb').read_bytes())

if __name__=='__main__':
 result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(NativeCockpit))
 (WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,'failures':[str(t)for t,_ in result.failures],'errors':[str(t)for t,_ in result.errors]},indent=2)+'\n')
 sys.exit(0 if result.wasSuccessful()else 1)
