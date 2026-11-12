# SPDX-License-Identifier: GPL-3.0-or-later
"""Offline, source-bound prop semantics; never imported by the game runtime."""
import hashlib,importlib.util,json,math,struct
from pathlib import Path
from speedtools.types import Vector3d
from speedtools.utils import get_path_case_insensitive

def encode(value):return json.dumps(value,ensure_ascii=False,separators=(',',':'),allow_nan=False).encode()
def sha(data):return hashlib.sha256(data).hexdigest()
def file_record(path):
 data=path.read_bytes();return {'path':path.name,'bytes':len(data),'sha256':sha(data)}
def raw_positions(obj):return [[v.location.x,v.location.y,v.location.z]for v in obj.mesh.vertices]
def shape_hash(obj):return sha(encode({'vertices':raw_positions(obj),'faces':[list(p.face)for p in obj.mesh.polygons]}))
def unique_offset(data,value):
 offset=data.find(value)
 if offset<0 or data.find(value,offset+1)>=0:raise ValueError('source record byte span is absent or ambiguous')
 return offset

def object_records(data):
 """Keep the exact FRD segment/chunk/local identities and parser object order."""
 objects=list(data.objects);out=[]
 chunks=[(s,c,segment,chunk)for s,segment in enumerate(data.frd.frd.segment_data)for c,chunk in enumerate(segment.object_chunks)]
 chunks.extend((-1,g,None,global_chunk.chunk)for g,global_chunk in enumerate(data.frd.frd.global_objects))
 for s,c,segment,chunk in chunks:
  for j,header in enumerate(chunk.objects):
   obj=objects[len(out)];attribute=None
   if segment is not None and header.attribute_index<len(segment.object_attributes):attribute=segment.object_attributes[header.attribute_index]
   out.append({'object':len(out),'segment':s,'chunk':c,'localObject':j,'header':header,'attribute':attribute,'value':obj})
 if len(out)!=len(objects):raise ValueError('source object ordering mismatch')
 return out

def verified_props(data):
 out={}
 for record in object_records(data):
  h,a=record['header'],record['attribute']
  if int(h.type)!=4 or a is None or int(a.collision_type)!=2:continue
  loc=Vector3d.from_frd_int3(a.location)
  error=math.dist([loc.x,loc.y,loc.z],[h.location.x,h.location.y,h.location.z])
  if error>.01:raise ValueError('normal destructible attribute is not colocated')
  key=(record['segment'],record['chunk'],record['localObject']);record['attributeError']=error;out[key]=record
 return out

def source_span(record,raw):
 h,a=record['header'],record['attribute'];p=h.location
 header=struct.pack('<II',int(h.type),h.attribute_index)+h.unknown+struct.pack('<fffI',p.x,p.y,p.z,h.specific_data_size)+h.unused1+struct.pack('<I',h.num_vertices)+h.unused2+struct.pack('<I',h.num_polygons)+h.unused3
 attr=struct.pack('<iii',a.location.x,a.location.y,a.location.z)+a.unknown1+struct.pack('<H',a.identifier)+a.unknown2+bytes([int(a.collision_type)])
 ho,ao=unique_offset(raw,header),unique_offset(raw,attr)
 if raw[ao+19]!=2:raise ValueError('raw collision byte did not match the whitelist')
 return {'headerOffset':ho,'attributeOffset':ao,'collisionByteOffset':ao+19,'collisionByte':raw[ao+19],
         'attributeIndex':h.attribute_index,'attributeIdentifier':a.identifier,'attributeMatchError':record['attributeError']}

def make_catalog(ident,directory,day,night,route):
 spec=importlib.util.spec_from_file_location('nfs_props_numeric',Path(__file__).with_name('nfs-rules.py'));rules=importlib.util.module_from_spec(spec);spec.loader.exec_module(rules)
 first,second=verified_props(day),verified_props(night)
 if first.keys()!=second.keys():raise ValueError('day/night destructible source identities differ')
 day_path=get_path_case_insensitive(directory,directory/'Tr.frd');night_path=get_path_case_insensitive(directory,directory/'TrN.frd')
 day_bytes,night_bytes=day_path.read_bytes(),night_path.read_bytes()
 frozen=rules.convert(route);frozen_bytes=encode(frozen)+b'\n'
 edge_map={};cursor=0
 for ordinal,b in enumerate(route['barriers']):
  count=1 if b['kind']=='wall'else (0 if b.get('collisionType')==120 else len(rules.hull(b.get('vertices',[]))))
  edge_map[ordinal]=list(range(cursor,cursor+count));cursor+=count
 if cursor!=len(frozen['barriers']):raise ValueError('expanded rule edge order changed')
 barriers={b['object']:i for i,b in enumerate(route['barriers'])if b['kind']=='object'}
 catalog={'version':1,'unit':'metres','track':ident,'geometry':route['geometryHash'],'ruleSha256':sha(frozen_bytes),
          'origin':route['origin'],'sourceTransform':[-1,1,1],
          'sources':{'dayFRD':file_record(day_path),'nightFRD':file_record(night_path),
                     'lowCAN':file_record(get_path_case_insensitive(directory,directory/'TR02.CAN')),
                     'highCAN':file_record(get_path_case_insensitive(directory,directory/'TR03.CAN'))},
          'animations':{},'props':[]}
 actions=dict(day.can)
 for action,can in day.can:
  speed='low'if action.name=='DESTROY_LOW_SPEED'else'high';animation=can.animation
  catalog['animations'][speed]={'suffix':f'destroy_{speed}_speed','frames':animation.length,'delayTicks':animation.delay,
                              'tickRate':64,'duration':(animation.length-1)*animation.delay/64,
                              'coordinates':'object-local-relative','trigger':'collision','loop':False,'clampWhenFinished':True}
 audit={'track':ident,'sourceFiles':catalog['sources'],'confirmed':[],'unconfirmedBillboardAssociations':[],
        'specialPhysics':[],'limitations':['Collision type names and 64 Hz action timing are community format interpretations, not observed Windows runtime behavior.',
        'Billboard attribute-index zero associations are not accepted as destructible identities.',
        'Original impact speed thresholds, impact-direction redirection and post-animation physical motion are not established.']}
 for index,(key,record)in enumerate(first.items()):
  other=second[key];obj=record['value'];h=record['header'];nh=other['header']
  if (h.location.x,h.location.y,h.location.z)!=(nh.location.x,nh.location.y,nh.location.z)or shape_hash(obj)!=shape_hash(other['value']):raise ValueError('day/night prop location or geometry differs')
  identifier=f'{ident}:s{key[0]}:c{key[1]}:o{key[2]}';pivot=[-h.location.x-route['origin'][0],h.location.y-route['origin'][1],h.location.z-route['origin'][2]]
  points=[[-v.location.x+pivot[0],v.location.y+pivot[1],v.location.z+pivot[2]]for v in obj.mesh.vertices]
  ordinal=barriers[record['object']];edges=edge_map[ordinal]
  if not edges or route['barriers'][ordinal]['collisionType']!=2:raise ValueError('confirmed object has no matching original collision edges')
  prop={'index':index,'id':identifier,'collisionType':2,'source':{'segment':key[0],'chunk':key[1],'localObject':key[2],
        'dayObject':record['object'],'nightObject':other['object'],'dayAttribute':h.attribute_index,'nightAttribute':nh.attribute_index},
        'node':f'prop_{index}','motionNode':f'prop_{index}_motion','clips':{'low':f'prop_{index}_destroy_low_speed','high':f'prop_{index}_destroy_high_speed'},
        'routeBarrier':ordinal,'ruleEdges':edges,'position':pivot,
        'bounds':{'min':[min(v[k]for v in points)for k in range(3)],'max':[max(v[k]for v in points)for k in range(3)]},
        'textures':sorted({day.get_polygon_material(p).name for p in obj.mesh.polygons})}
  catalog['props'].append(prop)
  audit['confirmed'].append({'index':index,'id':identifier,'day':source_span(record,day_bytes),'night':source_span(other,night_bytes),
                             'shapeHash':shape_hash(obj),'currentNode':'static_objects','routeBarrier':ordinal,'ruleEdges':edges,
                             'vertices':h.num_vertices,'polygons':h.num_polygons,'textures':prop['textures']})
 for record in object_records(day):
  h,a,obj=record['header'],record['attribute'],record['value']
  if int(h.type)==2 and a is not None and int(a.collision_type)!=0:
   p=Vector3d.from_frd_int3(a.location);error=math.dist([p.x,p.y,p.z],[h.location.x,h.location.y,h.location.z])
   audit['unconfirmedBillboardAssociations'].append({'object':record['object'],'segment':record['segment'],'chunk':record['chunk'],
      'localObject':record['localObject'],'attributeIndex':h.attribute_index,'inheritedCollisionType':int(a.collision_type),'attributeMatchError':error})
  if obj.physics is not None:
   p=obj.physics.dimension;audit['specialPhysics'].append({'object':record['object'],'chunk':record['chunk'],'localObject':record['localObject'],
       'sourceType':int(h.type),'mass':obj.physics.mass,'dimensions':[p.x,p.y,p.z]})
 # The canonical client JSON without this field is the semantic identity. The
 # decimetre numeric copy references it and has its own separate byte digest.
 catalog['catalogHash']=sha(encode(catalog))
 numeric={'version':1,'unit':'decimetres','track':ident,'geometry':catalog['geometry'],'ruleSha256':catalog['ruleSha256'],
          'catalogHash':catalog['catalogHash'],'props':[{'index':p['index'],'id':p['id'],'routeBarrier':p['routeBarrier'],'ruleEdges':p['ruleEdges'],
          'position':[rules.number(n,10)for n in p['position']],
          'bounds':{key:[rules.number(n,10)for n in p['bounds'][key]]for key in ['min','max']}}for p in catalog['props']]}
 audit['catalogHash']=catalog['catalogHash'];audit['counts']={'confirmed':len(catalog['props']),
   'billboardInheritedType2':sum(x['inheritedCollisionType']==2 for x in audit['unconfirmedBillboardAssociations']),'specialPhysics':len(audit['specialPhysics'])}
 return catalog,numeric,audit
