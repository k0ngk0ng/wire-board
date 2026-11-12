#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Offline glTF adapter for the separately installed speedtools 0.38 parser.

This build tool and its GPL dependency never run in the browser or server.
It reads original game data; emitted resources contain no parser source code.
"""

from __future__ import annotations

import argparse
import copy
from collections import Counter, defaultdict
from dataclasses import asdict, replace
from io import BytesIO
import hashlib
import json
import math
from pathlib import Path
import shutil
import struct
import sys

from speedtools.car_data import CarData
from speedtools.carp_data import CarpData
from speedtools.track_data import TrackData
from speedtools.utils import create_pil_image, get_path_case_insensitive, make_horizon_texture
from nfs_props import make_catalog


TRACK_NAMES = {'france': '法国乡间', 'coastal': '滨海公路', 'uk': '凯尔特遗迹', 'park': '奇迪亚克森林'}
SOURCE_TRANSFORM = [-1, 1, 1]


def digest(data):
    return hashlib.sha256(data).hexdigest()


def encoded(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False).encode()


def vec(value):
    """Explicit named access avoids speedtools' Blender-oriented tuple order."""
    return (-float(value.x), float(value.y), float(value.z))


def shifted(value, origin):
    return tuple(a-b for a, b in zip(vec(value), origin))


def rounded(values):
    return [round(float(x), 6) for x in values]


def unit(values):
    magnitude = math.sqrt(sum(x*x for x in values))
    if magnitude < 1e-12:
        return (0.0, 1.0, 0.0)
    return tuple(x/magnitude for x in values)


def linear(channel):
    value = channel / 255
    return value/12.92 if value <= .04045 else ((value+.055)/1.055)**2.4


def bounds(points):
    return {'min': rounded(min(p[k] for p in points) for k in range(3)),
            'max': rounded(max(p[k] for p in points) for k in range(3))}


def object_location(obj):
    if obj.location is not None:
        return obj.location
    for action in obj.actions:
        if action.animation.locations:
            return action.animation.locations[0]
    return None


def object_point(obj, point):
    x, y, z = point.x, point.y, point.z
    if obj.transform:
        m = obj.transform
        x, y, z = (m.x.x*x+m.z.x*z+m.y.x*y,
                   m.x.y*x+m.z.y*z+m.y.y*y,
                   m.x.z*x+m.z.z*z+m.y.z*y)
    location = object_location(obj)
    if location:
        x, y, z = x+location.x, y+location.y, z+location.z
    return (-float(x), float(y), float(z))


class GLB:
    def __init__(self, name, extras=None, unlit=False):
        self.data = bytearray()
        self.chunks = []
        self.webp_chunks = {}
        self.doc = {'asset': {'version': '2.0', 'generator': 'wire-board nfs-convert 1'},
                    'scene': 0, 'scenes': [{'name': name, 'nodes': []}], 'nodes': [],
                    'meshes': [], 'materials': [], 'textures': [], 'images': [],
                    'samplers': [{'magFilter': 9729, 'minFilter': 9987, 'wrapS': 10497, 'wrapT': 10497}],
                    'accessors': [], 'bufferViews': [], 'buffers': [], 'extras': extras or {}}
        self.unlit = unlit
        self.images = {}
        self.materials = {}
        self.image_alpha = {}
        self.texture_dimensions = []
        self.triangles = 0
        self.vertices = 0
        if unlit:
            self.doc['extensionsUsed'] = ['KHR_materials_unlit']

    def view(self, value, target=None):
        while len(self.data) % 4:
            self.data.append(0)
        offset = len(self.data)
        self.data.extend(value)
        self.chunks.append(bytes(value))
        item = {'buffer': 0, 'byteOffset': offset, 'byteLength': len(value)}
        if target:
            item['target'] = target
        self.doc['bufferViews'].append(item)
        return len(self.doc['bufferViews'])-1

    def accessor(self, values, width, component=5126, normalized=False, indices=False):
        code = {5126: 'f', 5125: 'I', 5123: 'H', 5121: 'B'}[component]
        flat = values if width == 1 else [x for row in values for x in row]
        view = self.view(struct.pack('<'+str(len(flat))+code, *flat), 34963 if indices else 34962)
        item = {'bufferView': view, 'componentType': component, 'count': len(values),
                'type': {1: 'SCALAR', 2: 'VEC2', 3: 'VEC3', 4: 'VEC4'}[width]}
        if normalized:
            item['normalized'] = True
        if width == 3 and not normalized and not indices:
            item['min'] = [min(row[k] for row in values) for k in range(3)]
            item['max'] = [max(row[k] for row in values) for k in range(3)]
        self.doc['accessors'].append(item)
        return len(self.doc['accessors'])-1

    def image(self, resource):
        pil = create_pil_image(resource.image).convert('RGBA')
        pixels = pil.tobytes()
        sha = digest(pixels + struct.pack('<II', *pil.size))
        if sha in self.images:
            return self.images[sha]
        output = BytesIO()
        pil.save(output, format='PNG', compress_level=9)
        webp = BytesIO()
        pil.save(webp, format='WEBP', lossless=True, exact=True, method=6)
        webp_bytes = webp.getvalue()
        index = len(self.doc['images'])
        self.doc['images'].append({'name': resource.name, 'mimeType': 'image/png',
                                  'bufferView': self.view(output.getvalue()),
                                  'extras': {'sourceDimensions': list(pil.size), 'pixelSha256': digest(pixels)}})
        image_view = self.doc['images'][-1]['bufferView']
        # Hidden RGB in alpha=0 texels is part of the source-pixel contract.
        from PIL import Image
        decoded_webp = Image.open(BytesIO(webp_bytes)).convert('RGBA')
        if len(webp_bytes) < len(output.getvalue()) and decoded_webp.tobytes() == pixels:
            self.webp_chunks[image_view] = (index, webp_bytes)
        self.doc['textures'].append({'source': index, 'sampler': 0})
        alpha = pil.getchannel('A').getextrema()
        self.image_alpha[index] = alpha
        self.images[sha] = index
        self.texture_dimensions.append({'name': resource.name, 'width': pil.width, 'height': pil.height,
                                        'pixelsSha256': digest(pixels)})
        return index

    def material(self, polygon, resource, car=False, texture_animation=None):
        image = self.image(resource)
        blend = getattr(resource.blend_mode, 'name', None)
        key = (image, polygon.backface_culling, polygon.transparent, polygon.highly_reflective,
               polygon.emissive, blend, car, repr(texture_animation))
        if key in self.materials:
            return self.materials[key]
        item = {'name': resource.name, 'doubleSided': not polygon.backface_culling,
                'pbrMetallicRoughness': {'baseColorTexture': {'index': image},
                                       'metallicFactor': .5 if polygon.highly_reflective else (0.12 if car else 0),
                                       'roughnessFactor': .25 if car else 1},
                'extras': {'sourceTexture': resource.name, 'originalBlend': blend,
                           'carPaintMaskInAlpha': car, 'billboard': polygon.billboard}}
        if self.unlit:
            item['extensions'] = {'KHR_materials_unlit': {}}
        if texture_animation:
            item['extras']['textureAnimation'] = texture_animation
        if polygon.transparent:
            item['pbrMetallicRoughness'] = {'baseColorFactor': [.32, .43, .48, .28],
                                          'metallicFactor': .25, 'roughnessFactor': .1}
            item['alphaMode'] = 'BLEND'
            item['doubleSided'] = True
        elif self.image_alpha[image][0] < 255:
            item['alphaMode'] = 'BLEND' if blend == 'ALPHA' and not car else 'MASK'
            if item['alphaMode'] == 'MASK':
                item['alphaCutoff'] = .5
        if polygon.emissive:
            item['emissiveFactor'] = [1, 1, 1]
        index = len(self.doc['materials'])
        self.doc['materials'].append(item)
        self.materials[key] = index
        return index

    def mesh(self, name, mesh_specs, extras=None, translation=None, car=False, root=True):
        groups = {}
        for mesh, material_map, position_map, normal_map in mesh_specs:
            for polygon in mesh.polygons:
                if len(polygon.face) < 3:
                    continue
                resource = material_map(polygon)
                animation = polygon.animation
                texture_animation = None
                if animation and hasattr(animation, 'count'):
                    frames = [self.image(material_map(replace(polygon, material=polygon.material+i))) for i in range(animation.count)]
                    texture_animation = {'kind': 'flipbook', 'textures': frames, 'interval': animation.ticks/64}
                elif animation and hasattr(animation, 'speed'):
                    texture_animation = {'kind': 'uv-scroll', 'speed': animation.speed}
                material = self.material(polygon, resource, car=car, texture_animation=texture_animation)
                group = groups.setdefault(material, {'vertices': [], 'normals': [], 'uv': [], 'colors': [],
                                                     'indices': [], 'mapping': {}})
                face = []
                for index, uv in zip(polygon.face, polygon.uv):
                    vertex = mesh.vertices[index]
                    location = position_map(vertex.location)
                    normal = normal_map(vertex.normal) if vertex.normal else None
                    color = tuple(round(linear(c)*65535) for c in vertex.color[:3]) + (round(vertex.color.alpha/255*65535),) if vertex.color else None
                    # speedtools exposes Blender V coordinates; glTF's V is top-down.
                    texcoord = (float(uv.u), 1-float(uv.v))
                    # Keep distinct native vertex identities even where an old
                    # FCE intentionally contains coincident vertices/faces.
                    key = (index, location, normal, texcoord, color)
                    target = group['mapping'].get(key)
                    if target is None:
                        target = len(group['vertices'])
                        group['mapping'][key] = target
                        group['vertices'].append(location)
                        group['normals'].append(normal)
                        group['uv'].append(texcoord)
                        group['colors'].append(color)
                    face.append(target)
                for index in range(1, len(face)-1):
                    triangle = (face[0], face[index], face[index+1])
                    if len(set(triangle)) == 3:
                        group['indices'].extend(triangle)
        primitives = []
        for material, group in groups.items():
            if not group['indices']:
                continue
            attributes = {'POSITION': self.accessor(group['vertices'], 3)}
            if 'baseColorTexture' in self.doc['materials'][material]['pbrMetallicRoughness']:
                attributes['TEXCOORD_0'] = self.accessor(group['uv'], 2)
            if all(n is not None for n in group['normals']):
                attributes['NORMAL'] = self.accessor(group['normals'], 3)
            if all(c is not None for c in group['colors']):
                attributes['COLOR_0'] = self.accessor(group['colors'], 4, 5123, normalized=True)
            component = 5123 if len(group['vertices']) <= 65535 else 5125
            primitives.append({'attributes': attributes,
                               'indices': self.accessor(group['indices'], 1, component, indices=True),
                               'material': material})
            self.triangles += len(group['indices'])//3
            self.vertices += len(group['vertices'])
        if not primitives:
            return None
        mesh_index = len(self.doc['meshes'])
        self.doc['meshes'].append({'name': name, 'primitives': primitives})
        node = {'name': name, 'mesh': mesh_index, 'extras': extras or {}}
        if translation is not None:
            node['translation'] = list(translation)
        index = len(self.doc['nodes'])
        self.doc['nodes'].append(node)
        if root:
            self.doc['scenes'][0]['nodes'].append(index)
        return index

    def animation(self, node, animation, origin):
        frames = min(len(animation.locations), len(animation.quaternions))
        if frames < 2:
            return
        times = [index*max(1, animation.delay)/64 for index in range(frames)]
        locations = [shifted(p, origin) for p in animation.locations[:frames]]
        rotations = []
        for q in animation.quaternions[:frames]:
            # Frames store inverse native orientations. Invert, then conjugate
            # by the X reflection so glTF rotation stays in the same RH basis.
            values = (-q.x, q.y, q.z, q.w)
            magnitude = math.sqrt(sum(value*value for value in values))
            rotation = tuple(value/magnitude for value in values) if magnitude > 1e-12 else (0,0,0,1)
            if rotations and sum(a*b for a,b in zip(rotation, rotations[-1])) < 0:
                rotation = tuple(-value for value in rotation)
            rotations.append(rotation)
        time_accessor = self.accessor(times, 1)
        self.doc['accessors'][time_accessor]['min'] = [times[0]]
        self.doc['accessors'][time_accessor]['max'] = [times[-1]]
        translation = self.accessor(locations, 3)
        rotation = self.accessor(rotations, 4)
        for accessor in (time_accessor, translation, rotation):
            self.doc['bufferViews'][self.doc['accessors'][accessor]['bufferView']].pop('target', None)
        self.doc['nodes'][node]['translation'] = list(locations[0])
        self.doc['nodes'][node]['rotation'] = list(rotations[0])
        self.doc.setdefault('animations', []).append({
            'name': self.doc['nodes'][node]['name']+'_loop',
            'samplers': [{'input':time_accessor,'output':translation,'interpolation':'LINEAR'},
                         {'input':time_accessor,'output':rotation,'interpolation':'LINEAR'}],
            'channels': [{'sampler':0,'target':{'node':node,'path':'translation'}},
                         {'sampler':1,'target':{'node':node,'path':'rotation'}}],
            'extras': {'fps':64, 'sourceFrameDelay':animation.delay, 'sourceFrames':frames}})

    def destruction_animation(self, node, animation, name):
        frames = len(animation.locations)
        if frames != len(animation.quaternions) or frames < 2:
            raise ValueError('incomplete original destruction animation')
        times = [index*animation.delay/64 for index in range(frames)]
        locations = [vec(p) for p in animation.locations]
        rotations = []
        for q in animation.quaternions:
            value = (-q.x, q.y, q.z, q.w)
            length = math.sqrt(sum(v*v for v in value))
            if length < 1e-12:
                raise ValueError('invalid original destruction quaternion')
            value = tuple(v/length for v in value)
            if rotations and sum(a*b for a,b in zip(value,rotations[-1])) < 0:
                value = tuple(-v for v in value)
            rotations.append(value)
        timeline = self.accessor(times,1)
        self.doc['accessors'][timeline].update(min=[times[0]],max=[times[-1]])
        translation = self.accessor(locations,3)
        rotation = self.accessor(rotations,4)
        for accessor in (timeline,translation,rotation):
            self.doc['bufferViews'][self.doc['accessors'][accessor]['bufferView']].pop('target',None)
        self.doc.setdefault('animations',[]).append({'name':name,
            'samplers':[{'input':timeline,'output':translation,'interpolation':'LINEAR'},
                        {'input':timeline,'output':rotation,'interpolation':'LINEAR'}],
            'channels':[{'sampler':0,'target':{'node':node,'path':'translation'}},
                        {'sampler':1,'target':{'node':node,'path':'rotation'}}],
            'extras':{'trigger':'collision','loop':False,'clampWhenFinished':True,'coordinates':'object-local-relative',
                      'fps':64,'sourceFrameDelay':animation.delay,'sourceFrames':frames}})

    def packed(self, doc, replacements=None):
        blob = bytearray()
        for index, original in enumerate(self.chunks):
            chunk = replacements.get(index, original) if replacements else original
            while len(blob) % 4:
                blob.append(0)
            doc['bufferViews'][index]['byteOffset'] = len(blob)
            doc['bufferViews'][index]['byteLength'] = len(chunk)
            blob.extend(chunk)
        doc['buffers'] = [{'byteLength': len(blob)}]
        payload = encoded(doc)
        payload += b' ' * (-len(payload) % 4)
        blob = bytes(blob)
        blob += b'\0' * (-len(blob) % 4)
        total = 12+8+len(payload)+8+len(blob)
        return struct.pack('<4sII', b'glTF', 2, total)+struct.pack('<II', len(payload), 0x4E4F534A)+payload+struct.pack('<II', len(blob), 0x004E4942)+blob

    def write(self, path):
        png = self.packed(copy.deepcopy(self.doc))
        chosen = png
        webp_count = 0
        if self.webp_chunks:
            doc = copy.deepcopy(self.doc)
            doc.setdefault('extensionsUsed', []).append('EXT_texture_webp')
            doc.setdefault('extensionsRequired', []).append('EXT_texture_webp')
            for _, (image, _) in self.webp_chunks.items():
                doc['images'][image]['mimeType'] = 'image/webp'
                texture = doc['textures'][image]
                texture.pop('source')
                texture['extensions'] = {'EXT_texture_webp': {'source': image}}
            candidate = self.packed(doc, {view: value[1] for view, value in self.webp_chunks.items()})
            if len(candidate) < len(png):
                chosen = candidate
                webp_count = len(self.webp_chunks)
        path.write_bytes(chosen)
        return {'triangles': self.triangles, 'vertices': self.vertices,
                'materials': len(self.doc['materials']), 'textures': self.texture_dimensions,
                'bytes': len(chosen), 'sha256': digest(chosen),
                'losslessEncoding': {'pngOnlyBytes': len(png), 'chosenBytes': len(chosen),
                                     'webpImages': webp_count, 'savedBytes': len(png)-len(chosen)}}


def car_export(root, ident, output):
    directory = get_path_case_insensitive(root, root/'Data/Cars'/ident)
    car = CarData(directory, root)
    parts = list(car.body)
    points = [(p.location.x+v.location.x, p.location.y+v.location.y, p.location.z+v.location.z)
              for p in parts for v in p.mesh.vertices]
    ground = -min(p[1] for p in points)
    builder = GLB(ident, {'vehicle': ident, 'sourceTransform': SOURCE_TRANSFORM,
                         'groundOffset': ground, 'forward': [0, 0, 1]})
    indices = []
    for part in parts:
        index = builder.mesh(part.name, [(part.mesh, car.material_map, vec, vec)],
                             extras={'wheel': part.name.endswith('_whl')},
                             translation=vec(part.location), car=True, root=False)
        if index is not None:
            indices.append(index)
    exterior_triangles = builder.triangles
    exterior = len(builder.doc['nodes'])
    builder.doc['nodes'].append({'name': 'exterior', 'children': indices})
    interior_parts = list(car.interior)
    camera = car.interior_camera
    if not interior_parts or camera is None:
        raise ValueError(f'{ident}: original cockpit mesh/camera is absent')
    cockpit_indices = []
    for part in interior_parts:
        index = builder.mesh(part.name, [(part.mesh, car.material_map, vec, vec)],
                             extras={'cockpit': True, 'originalPart': part.name},
                             translation=vec(part.location), car=True, root=False)
        if index is not None:
            cockpit_indices.append(index)
    cockpit = len(builder.doc['nodes'])
    builder.doc['nodes'].append({'name': 'cockpit', 'extras': {'originalInterior': True}, 'children': cockpit_indices})
    cockpit_triangles = builder.triangles - exterior_triangles
    cockpit_camera = list(vec(camera.location))
    cockpit_camera[1] += ground
    builder.doc['nodes'].append({'name': 'vehicle', 'translation': [0, ground, 0], 'children': [exterior, cockpit]})
    builder.doc['scenes'][0]['nodes'] = [len(builder.doc['nodes'])-1]
    viv_path = get_path_case_insensitive(directory, directory/'car.viv')
    viv_bytes = viv_path.read_bytes()
    cockpit_source = []
    for member in ['dash.fce', 'dash00.tga']:
        entry = next(e for e in car.viv.viv.entries if e.name.lower() == member)
        data = viv_bytes[entry.offset:entry.offset+entry.length]
        cockpit_source.append({'member': member, 'bytes': len(data), 'sha256': digest(data)})
    builder.doc['extras'].update({'cockpitCamera': cockpit_camera, 'cockpitSource': cockpit_source,
                                 'exteriorTriangles': exterior_triangles, 'cockpitTriangles': cockpit_triangles})
    variants = {}
    for entry in car.viv.viv.entries:
        if entry.name.lower() not in ('carp.txt', 'carp1.txt', 'carp2.txt', 'carp3.txt'):
            continue
        data = viv_bytes[entry.offset:entry.offset+entry.length]
        variant = 'base' if entry.name.lower() == 'carp.txt' else 'stage'+entry.name[4]
        variants[variant] = {'parameters': CarpData.to_dict(data.decode('ascii')),
                             'source': {'member': entry.name, 'offset': entry.offset, 'bytes': entry.length,
                                        'sha256': digest(data)}}
    wheels = {}
    for part in parts:
        if part.name.endswith('_whl'):
            radius = max(math.hypot(v.location.y, v.location.z) for v in part.mesh.vertices)
            wheels[part.name] = {'position': rounded(vec(part.location)), 'radius': round(radius, 6)}
    metadata = {'name': car.fedata.name if car.fedata else ident,
                'manufacturer': car.fedata.manufacturer if car.fedata else '',
                'dimensions': [car.dimensions.x, car.dimensions.y, car.dimensions.z],
                'groundOffset': ground, 'defaultVariant': 'base', 'variants': variants,
                'wheels': wheels, 'colors': [asdict(c) for c in car.colors],
                'cockpitCamera': cockpit_camera, 'cockpitSource': cockpit_source,
                'sourceSha256': digest(viv_bytes), 'forward': [0, 0, 1],
                'model': f'cars/{ident.lower()}.glb'}
    stats = builder.write(output/f'cars/{ident.lower()}.glb')
    stats.update({'exteriorTriangles': exterior_triangles, 'cockpitTriangles': cockpit_triangles})
    return metadata, stats


def load_track(root, track, night=False):
    directory = get_path_case_insensitive(root, root/'Data/Tracks'/track)
    data = TrackData(directory, root, night=night)
    return data, list(data.track_segments), list(data.objects)


def track_positions(segments, objects):
    for segment in segments:
        for vertex in segment.mesh.vertices:
            yield vec(vertex.location)
    for obj in objects:
        for vertex in obj.mesh.vertices:
            yield object_point(obj, vertex.location)


def environment(data, origin, sky):
    fog = data.fog
    return {'night': data.night, 'ambient': list(data.ambient_color.rgb), 'sky': sky,
            'horizon': asdict(data.horizon),
            'fog': {'color': list(fog.color.rgb), 'density': fog.density, 'horizon': fog.horizon,
                    'regions': [asdict(x) for x in fog.regions]},
            'lights': [{'position': rounded(shifted(light.location, origin)), 'color': list(light.color.rgb),
                        'size': light.flare_size, 'blinkMs': light.blink_interval_ms} for light in data.lights]}


def route_export(ident, data, segments, objects, origin, world_bounds):
    points = []
    barriers = []
    surfaces = []
    unresolved = Counter()
    for segment_index, segment in enumerate(segments):
        effect_counts = Counter()
        for collision in segment.collision_meshes[:-1]:
            effect_counts[int(collision.collision_effect)] += len(collision.polygons)
            surfaces.append({'segment': segment_index, 'surface': int(collision.collision_effect),
                             'vertices': [rounded(shifted(v.location, origin)) for v in collision.vertices],
                             'faces': [list(p.face) for p in collision.polygons]})
        main_surface = next((kind for kind, _ in effect_counts.most_common() if kind in (1, 2, 4, 7)), 1)
        for waypoint in segment.waypoints:
            x, y, z = shifted(waypoint.location, origin)
            nx, ny, nz = unit(vec(waypoint.orientation.y))
            fx, fy, fz = unit(vec(waypoint.orientation.z))
            rx, ry, rz = unit(vec(waypoint.orientation.x))
            points.append({'x': round(x, 6), 'y': round(y, 6), 'z': round(z, 6),
                           'leftWidth': round(waypoint.left_lane_width*max(1, waypoint.num_left_lanes), 6),
                           'rightWidth': round(waypoint.right_lane_width*max(1, waypoint.num_right_lanes), 6),
                           'nx': round(nx, 6), 'ny': round(ny, 6), 'nz': round(nz, 6),
                           'fx': round(fx, 6), 'fy': round(fy, 6), 'fz': round(fz, 6),
                           'rx': round(rx, 6), 'ry': round(ry, 6), 'rz': round(rz, 6),
                           'leftWall': round(waypoint.left_wall, 6), 'rightWall': round(waypoint.right_wall, 6),
                           'surface': main_surface, 'segment': segment_index})
        wall = segment.collision_meshes[-1]
        for polygon in wall.polygons:
            vertices = [shifted(wall.vertices[index].location, origin) for index in polygon.face]
            barriers.append({'kind': 'wall', 'a': rounded(vertices[0]), 'b': rounded(vertices[1]),
                             'height': round(max(p[1] for p in vertices)-min(p[1] for p in vertices), 6),
                             'segment': segment_index})
    for index, obj in enumerate(objects):
        collision_type = int(obj.collision_type)
        if collision_type == 0:
            continue
        vertices = [tuple(a-b for a, b in zip(object_point(obj, v.location), origin)) for v in obj.mesh.vertices]
        if collision_type not in (1, 2):
            unresolved[f'objectCollisionType{collision_type}'] += 1
        barriers.append({'kind': 'object', 'object': index, 'collisionType': collision_type,
                         'bounds': bounds(vertices), 'vertices': [rounded(p) for p in vertices],
                         'faces': [list(p.face) for p in obj.mesh.polygons]})
    length = sum(math.dist((a['x'], a['y'], a['z']), (b['x'], b['y'], b['z']))
                 for a, b in zip(points, points[1:]+points[:1]))
    first = points[0]
    route = {'version': 1, 'id': ident, 'name': TRACK_NAMES.get(ident, ident),
             'origin': rounded(origin), 'sourceTransform': SOURCE_TRANSFORM,
             'bounds': world_bounds, 'length': round(length, 6), 'points': points,
             'barriers': barriers,
             'start': {'index': 0, 'position': [first['x'], first['y'], first['z']],
                       'forward': [first['fx'], first['fy'], first['fz']],
                       'source': 'first native VROAD record; exact original grid-slot offsets unverified'},
             'unresolved': dict(unresolved)}
    return route, {'version': 1, 'id': ident, 'origin': rounded(origin),
                   'sourceTransform': SOURCE_TRANSFORM, 'surfaces': surfaces}


def track_export(root, track, output):
    ident = track.lower()
    variants = [(False,)+load_track(root, track), (True,)+load_track(root, track, night=True)]
    all_positions = [point for _, _, segments, objects in variants for point in track_positions(segments, objects)]
    raw_bounds = bounds(all_positions)
    origin = (raw_bounds['min'][0]-30, 0, raw_bounds['min'][2]-30)
    world = [tuple(a-b for a, b in zip(point, origin)) for point in all_positions]
    world_bounds = bounds(world)
    route, collision = route_export(ident, variants[0][1], variants[0][2], variants[0][3], origin, world_bounds)
    geometry_hash = digest(encoded({'origin': route['origin'], 'points': route['points'], 'barriers': route['barriers']}))
    route['geometryHash'] = geometry_hash
    route['geometryVersion'] = f'nfs4-v1-{geometry_hash[:16]}'
    directory = get_path_case_insensitive(root, root/'Data/Tracks'/track)
    route['sourceFiles'] = [{'name': name, 'bytes': len(data), 'sha256': digest(data)}
                            for name in ('Tr.frd', 'heights.sim')
                            for data in [get_path_case_insensitive(directory, directory/name).read_bytes()]]
    frozen_route = Path(__file__).resolve().parents[2]/'games/racing/assets/public/racing/v3/tracks'/f'{ident}.route.json'
    if frozen_route.exists() and frozen_route.read_bytes() != encoded(route)+b'\n':
        raise ValueError(f'{ident}: prop semantics cannot change the immutable original route')
    catalog,numeric,audit = make_catalog(ident,directory,variants[0][1],variants[1][1],route)
    for folder,value in [('props',catalog),('rules',numeric),('audit',audit)]:
        (output/folder).mkdir(exist_ok=True)
        suffix = '.json' if folder == 'props' else '.props.json'
        (output/folder/(ident+suffix)).write_bytes(encoded(value)+b'\n')
    (output/f'tracks/{ident}.route.json').write_bytes(encoded(route)+b'\n')
    (output/f'tracks/{ident}.collision.json').write_bytes(encoded(collision)+b'\n')
    stats = {}
    for night, data, segments, objects in variants:
        name = ident+('-night' if night else '')
        sky_name = f'{name}-sky.png'
        sky_resources = list(data.sky_images)
        if sky_resources:
            make_horizon_texture(sky_resources).save(output/'tracks'/sky_name, format='PNG', compress_level=9)
        else:
            sky_name = None
        env = environment(data, origin, sky_name)
        builder = GLB(name, {'origin': rounded(origin), 'sourceTransform': SOURCE_TRANSFORM,
                             'environment': env, 'track': ident, 'night': night,
                             'geometryVersion': route['geometryVersion'],'propsCatalogHash':catalog['catalogHash']}, unlit=True)
        # Consecutive road blocks are separate culling groups; original high-LOD
        # surfaces, decals and vertex lighting are all retained.
        for index, segment in enumerate(segments):
            builder.mesh(f'segment_{index}', [(segment.mesh, data.get_polygon_material,
                                               lambda p: shifted(p, origin), vec)],
                         extras={'segment': index})
        animated = 0
        billboard = 0
        object_specs = []
        prop_objects = {p['source']['nightObject' if night else 'dayObject']:p for p in catalog['props']}
        for index, obj in enumerate(objects):
            if index in prop_objects:
                prop=prop_objects[index]
                mesh=builder.mesh(f"prop_{prop['index']}_mesh",[(obj.mesh,data.get_polygon_material,vec,vec)],root=False)
                if mesh is None:
                    raise ValueError('confirmed prop has no original geometry')
                motion=len(builder.doc['nodes'])
                builder.doc['nodes'].append({'name':prop['motionNode'],'children':[mesh],'translation':[0,0,0],'rotation':[0,0,0,1],
                    'extras':{'propMotion':True,'propIndex':prop['index']}})
                node=len(builder.doc['nodes'])
                builder.doc['nodes'].append({'name':prop['node'],'translation':list(prop['position']),'children':[motion],
                    'extras':{'propIndex':prop['index'],'propId':prop['id'],'object':index,'collisionType':2,
                              'destructible':True,'segment':prop['source']['segment']}})
                builder.doc['scenes'][0]['nodes'].append(node)
                for action in obj.actions:
                    speed='low' if action.action.name=='DESTROY_LOW_SPEED' else 'high'
                    builder.destruction_animation(motion,action.animation,prop['clips'][speed])
                continue
            is_billboard = any(p.billboard for p in obj.mesh.polygons)
            is_animated = any(str(a.action) == 'Action.DEFAULT_LOOP' for a in obj.actions)
            mapping = lambda p, o=obj: tuple(a-b for a, b in zip(object_point(o, p), origin))
            if is_billboard or is_animated:
                location = object_location(obj)
                center = shifted(location, origin) if location else (0, 0, 0)
                local_mapping = vec if is_animated else lambda p, m=mapping, c=center: tuple(a-b for a, b in zip(m(p), c))
                node = builder.mesh(f'object_{index}', [(obj.mesh, data.get_polygon_material, local_mapping, vec)],
                             extras={'object': index, 'billboard': is_billboard, 'animated': is_animated,
                                     'collisionType': int(obj.collision_type)}, translation=center)
                if is_animated and node is not None:
                    for action in obj.actions:
                        if str(action.action) == 'Action.DEFAULT_LOOP':
                            builder.animation(node, action.animation, origin)
                animated += int(is_animated)
                billboard += int(is_billboard)
            else:
                object_specs.append((obj.mesh, data.get_polygon_material, mapping, vec))
        builder.mesh('static_objects', object_specs, extras={'staticObjects': len(object_specs)})
        item = builder.write(output/f'tracks/{name}.glb')
        item.update({'segments': len(segments), 'objects': len(objects),
                     'destructibleObjects':len(catalog['props']),
                     'billboardObjects': billboard, 'animatedObjects': animated,
                     'sourcePolygons': sum(len(s.mesh.polygons) for s in segments)+sum(len(o.mesh.polygons) for o in objects),
                     'unresolved': ['destruction actions are one-shot collision clips; original speed threshold/direction mapping unverified',
                                    'billboard nodes require camera-facing update',
                                    'additive and texture-animation metadata require renderer application']})
        stats[name] = item
    return stats


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--game-root', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--cars', nargs='+', default=['P911', 'FM50'])
    parser.add_argument('--tracks', nargs='+', default=['France'])
    args = parser.parse_args()
    root = args.game_root.resolve()
    output = args.out.resolve()
    repository = Path(__file__).resolve().parents[2]
    if not output.is_relative_to(repository):
        parser.error('output must stay inside this repository')
    if output.exists():
        parser.error('output must be a new directory; existing data is never overwritten')
    output.parent.mkdir(parents=True, exist_ok=True)
    staging = output.with_name(output.name+'.building')
    if staging.exists():
        parser.error('staging path already exists')
    staging.mkdir()
    (staging/'cars').mkdir()
    (staging/'tracks').mkdir()
    try:
        cars = {}
        vehicle_stats = {}
        for ident in args.cars:
            cars[ident], vehicle_stats[ident] = car_export(root, ident, staging)
        (staging/'cars.json').write_bytes(encoded({'version': 1, 'cars': cars})+b'\n')
        track_stats = {}
        for track in args.tracks:
            track_stats.update(track_export(root, track, staging))
        assets = []
        for path in sorted(p for p in staging.rglob('*') if p.is_file() and p.relative_to(staging).parts[0] not in ('rules','audit')):
            data = path.read_bytes()
            assets.append({'path': path.relative_to(staging).as_posix(), 'bytes': len(data), 'sha256': digest(data),
                           'mime': {'glb': 'model/gltf-binary', 'json': 'application/json', 'png': 'image/png'}[path.suffix[1:]]})
        manifest = {'version': 3, 'generator': 'wire-board nfs-convert 1', 'parser': 'speedtools 0.38.0',
                    'units': 'metres', 'axis': 'Y-up', 'sourceTransform': SOURCE_TRANSFORM,
                    'source': 'original NFS4 PC game resources; EA intellectual property',
                    'assets': assets, 'vehicles': vehicle_stats, 'tracks': track_stats,
                    'totalBytes': sum(item['bytes'] for item in assets)}
        (staging/'manifest.json').write_bytes(encoded(manifest)+b'\n')
        staging.rename(output)
        print(json.dumps({'output': str(output.relative_to(repository)), 'assets': len(assets),
                          'bytes': manifest['totalBytes'], 'cars': list(cars), 'tracks': list(track_stats)}, ensure_ascii=False))
    except BaseException:
        shutil.rmtree(staging)
        raise


if __name__ == '__main__':
    main()
