import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import sys

from PIL import Image
from speedtools.fsh_data import FshData
from speedtools.utils import create_pil_image

ROOT = Path(__file__).resolve().parents[3]
PUBLIC = ROOT / 'games/racing/assets/public/racing/v3'
DATA = ROOT / 'games/racing/server/data'
WEB = ROOT / 'games/racing/web'
NAMES = {'BMW5': '宝马 M5', 'JXKR': '捷豹 XKR', 'MCLK': '奔驰 CLK GTR'}
TOOL_LICENSE = 'GPL-3.0-or-later'


def read_json(path):
    return json.loads(path.read_text())


def encoded(value):
    return (json.dumps(value, ensure_ascii=False, separators=(',', ':'), allow_nan=False) + '\n').encode()


def sha(data):
    return hashlib.sha256(data).hexdigest()


def write(path, data, immutable=False):
    if not path.resolve().is_relative_to(ROOT):
        raise ValueError('Output must remain in the repository')
    if path.exists():
        if path.read_bytes() == data:
            return
        if immutable:
            raise ValueError('Refusing to change immutable resource: ' + str(path))
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def load_tool(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts/racing' / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def normalized_vehicle(ident, car):
    p = car['variants']['base']['parameters']
    count = p['automatic_number_of_gears'] - 2
    if not 1 <= count <= 6 or len(p['torque_curve']) != 21:
        raise ValueError('Unsupported original drivetrain: ' + ident)
    rear = p['tire_specs_rear']
    result = {'id': ident.lower(), 'mass': p['mass'], 'wheelbase': p['wheel_base'],
              'tireRadius': rear[0] * rear[1] / 100000 + rear[2] * .0254 / 2,
              'maxSpeed': p['top_speed_cap'], 'idleRPM': p['engine_minimum_rpm'],
              'redlineRPM': p['engine_redline_rpm'], 'finalDrive': p['automatic_final_gear'],
              'maxBraking': p['maximum_braking_deceleration'], 'frontGripBias': p['front_grip_bias'],
              'frontBrakeBias': p['front_bias_brake_ratio'], 'frontDriveRatio': p['front_drive_ratio'],
              'turnInRate': p['turn_in_ramp'] / 4, 'turnOutRate': p['turn_out_ramp'] / 4,
              'turningRadius': p['turning_circle_radius'], 'lateralGrip': p['lateral_acceleration_grip_multiplier'],
              'gasOffFactor': p['gas_off_factor'], 'shiftDelay': p['gear_shift_delay'] / 32,
              'gearCount': count, 'reverseVelocityToRPM': abs(p['automatic_velocity_to_rpm_ratio'][0]),
              'reverseEfficiency': p['automatic_gear_efficiency'][0], 'torqueCurve': p['torque_curve']}
    for target, source in [('gearRatios', 'automatic_gear_ratios'), ('gearEfficiency', 'automatic_gear_efficiency'), ('velocityToRPM', 'automatic_velocity_to_rpm_ratio')]:
        values = p[source][2:2 + count]
        result[target] = values + [0] * (6 - len(values))
    return result


def export_source(args):
    from speedtools.parsers import FceParser, VivParser
    from speedtools.fce_data import FceData
    from speedtools.types import Image as NativeImage, Resource
    sys.path.insert(0, str(ROOT / 'scripts/racing'))
    converter = load_tool('racing_content_converter', 'nfs-convert.py')
    converter.TRACK_NAMES['germany'] = '德国乡间公路'
    sys.argv = ['nfs-convert.py', '--game-root', str(args.game_root), '--out', str(args.converted), '--cars', *NAMES, '--tracks', 'Germany']
    converter.main()
    audio = load_tool('racing_content_audio', 'nfs-audio.py')
    sys.argv = ['nfs-audio.py', '--game-root', str(args.game_root), '--out', str(args.audio), '--cars', *NAMES]
    audio.main()
    source = args.source / 'DATA/FEART/TRACK/TRKGO.VIV'
    data = source.read_bytes()
    entries = {item.name: item for item in VivParser.from_file(source).entries}
    item = entries['tr.tga']
    texture = Resource(name='tr.tga', image=NativeImage(data=data[item.offset:item.offset + item.length]))
    reports = []
    for ident, member in [('france', 'fran.fce'), ('coastal', 'cost.fce'), ('uk', 'uk.fce'), ('park', 'park.fce'), ('germany', 'germ.fce')]:
        item = entries[member]
        raw = data[item.offset:item.offset + item.length]
        parts = list(FceData(FceParser.from_bytes(raw)).parts)
        builder = converter.GLB(ident, {'source': 'original NFS4 menu track model', 'sourceMember': member}, unlit=False)
        for part in parts:
            builder.mesh(part.name, [(part.mesh, lambda polygon: texture, converter.vec, converter.vec)], translation=converter.vec(part.location), car=True)
        result = builder.write(args.miniatures / (ident + '.glb'))
        reports.append({'id': ident, 'member': member, 'sourceSHA256': sha(raw), 'parts': len(parts), 'triangles': result['triangles']})
    write(args.miniatures / 'sources.json', json.dumps(reports, indent=2).encode() + b'\n')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--converted', type=Path, required=True)
    parser.add_argument('--audio', type=Path, required=True)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--acquisition', type=Path, required=True)
    parser.add_argument('--track-previews', type=Path)
    parser.add_argument('--export-only', action='store_true')
    parser.add_argument('--game-root', type=Path)
    parser.add_argument('--miniatures', type=Path, default=ROOT / '.local/racing-content/miniature')
    args = parser.parse_args()
    for output in [args.converted, args.audio, args.miniatures]:
        if output.resolve() == ROOT or not output.resolve().is_relative_to(ROOT):
            raise ValueError('Output must remain in a dedicated repository directory')
    acquisition = read_json(args.acquisition)
    for item in acquisition['files']:
        data = (args.source / item['path']).read_bytes()
        if len(data) != item['bytes'] or sha(data) != item['sha256']:
            raise ValueError('Original source changed: ' + item['path'])
    if args.export_only:
        if args.game_root is None:
            raise ValueError('Original game root is required for export')
        args.miniatures.mkdir(parents=True, exist_ok=True)
        export_source(args)
        return
    if args.track_previews is None:
        raise ValueError('Rendered original track previews are required')
    converted = read_json(args.converted / 'manifest.json')
    audio = read_json(args.audio / 'audio-manifest.json')
    new_cars = read_json(args.converted / 'cars.json')['cars']
    if set(new_cars) != set(NAMES):
        raise ValueError('Expected the three selected original vehicles')
    for ident in ['france', 'coastal', 'uk', 'park', 'germany']:
        Image.open(args.track_previews / (ident + '.png')).verify()
    sys.path.insert(0, str(ROOT / 'scripts/racing'))
    rules = load_tool('racing_content_rules', 'nfs-rules.py')
    route_bytes = (args.converted / 'tracks/germany.route.json').read_bytes()
    route = json.loads(route_bytes)
    road = rules.convert(route)
    rule_bytes = encoded(road)
    prop_bytes = (args.converted / 'rules/germany.props.json').read_bytes()
    props = json.loads(prop_bytes)
    if props['geometry'] != road['geometry'] or props['ruleSha256'] != sha(rule_bytes):
        raise ValueError('New prop catalog differs from the exact route rules')
    for pack, folder in [(converted, args.converted), (audio, args.audio)]:
        for asset in pack['assets']:
            if asset['path'] in ['cars.json', 'audio-manifest.json']:
                continue
            data = (folder / asset['path']).read_bytes()
            if len(data) != asset['bytes'] or sha(data) != asset['sha256']:
                raise ValueError('Converted resource changed: ' + asset['path'])
            write(PUBLIC / asset['path'], data, immutable=True)
    cars = read_json(PUBLIC / 'cars.json')
    cars['cars'].update(new_cars)
    write(PUBLIC / 'cars.json', encoded(cars))
    engine_manifest = read_json(PUBLIC / 'audio-manifest.json')
    engine_manifest['models'].update(audio['models'])
    engine_assets = {item['path']: item for item in engine_manifest['assets']}
    engine_assets.update({item['path']: item for item in audio['assets']})
    engine_manifest['assets'] = [engine_assets[key] for key in sorted(engine_assets)]
    engine_manifest['totalBytes'] = sum(item['bytes'] for item in engine_manifest['assets'])
    write(PUBLIC / 'audio-manifest.json', encoded(engine_manifest))
    extra = {'version': 1, 'vehicles': {}, 'bodies': {}}
    definitions = read_json(WEB / 'classic-vehicles.json')
    definitions = [item for item in definitions if item['id'] not in {key.lower() for key in NAMES}]
    bodies = read_json(DATA / 'car-bodies.json')
    for ident, car in new_cars.items():
        key = ident.lower()
        params = normalized_vehicle(ident, car)
        extra['vehicles'][key] = params
        body = dict(zip(['width', 'height', 'length'], car['dimensions']))
        extra['bodies'][key] = body
        bodies[key] = body
        colors = car['colors']
        definitions.append({'id': key, 'name': NAMES[ident], 'sourceName': car['name'], 'dimensions': car['dimensions'], 'colors': colors,
                            'maxSpeedKmh': round(params['maxSpeed'] * 3.6), 'mass': params['mass'], 'groundOffset': car['groundOffset']})
    write(DATA / 'extra-vehicles.json', encoded(extra))
    write(WEB / 'classic-extra-vehicles.json', encoded(extra))
    write(DATA / 'car-bodies.json', encoded(bodies))
    write(WEB / 'classic-vehicles.json', encoded(definitions))
    write(DATA / 'rules' / (road['geometry'] + '.json'), rule_bytes, immutable=True)
    index = [item for item in read_json(DATA / 'index.json') if item['id'] != 'germany']
    index.append({'id': 'germany', 'geometry': road['geometry'], 'ruleSha256': sha(rule_bytes), 'bytes': len(rule_bytes),
                  'sourceSha256': sha(route_bytes), 'points': len(road['points']), 'barriers': len(road['barriers'])})
    write(DATA / 'index.json', encoded(index))
    write(DATA / 'props' / (props['catalogHash'] + '.json'), prop_bytes, immutable=True)
    prop_index = [item for item in read_json(DATA / 'prop-index.json') if item['geometry'] != road['geometry']]
    prop_index.append({'geometry': road['geometry'], 'catalog': props['catalogHash'], 'sha256': sha(prop_bytes), 'current': True, 'legacy': True})
    write(DATA / 'prop-index.json', encoded(prop_index))
    preview_sources = []
    for ident in [item['id'].upper() for item in definitions]:
        source = args.source / f'DATA/FEART/VIDWALL/{ident}.QFS'
        resource = next(value for value in FshData.from_file(source).resources if value.name == 'shp1')
        image = create_pil_image(resource.image).convert('RGBA')
        target = PUBLIC / f'previews/cars/{ident.lower()}.webp'
        target.parent.mkdir(parents=True, exist_ok=True)
        image.save(target, 'WEBP', lossless=True, exact=True, method=6)
        if Image.open(target).convert('RGBA').tobytes() != image.tobytes():
            raise ValueError('Lossless preview changed original menu pixels')
        preview_sources.append({'id': ident.lower(), 'kind': 'car', 'source': source.relative_to(args.source).as_posix(),
                                'sourceSHA256': sha(source.read_bytes()), 'resource': resource.name, 'pixelsSHA256': sha(image.tobytes()), 'width': image.width, 'height': image.height})
    for ident in ['france', 'coastal', 'uk', 'park', 'germany']:
        source = args.track_previews / (ident + '.png')
        image = Image.open(source).convert('RGBA')
        target = PUBLIC / f'previews/tracks/{ident}.webp'
        target.parent.mkdir(parents=True, exist_ok=True)
        image.save(target, 'WEBP', lossless=True, exact=True, method=6)
        if Image.open(target).convert('RGBA').tobytes() != image.tobytes():
            raise ValueError('Lossless track preview changed rendered pixels')
        preview_sources.append({'id': ident, 'kind': 'track', 'sourceSHA256': sha(source.read_bytes()), 'pixelsSHA256': sha(image.tobytes()), 'width': image.width, 'height': image.height})
    manifest = read_json(PUBLIC / 'manifest.json')
    manifest['vehicles'].update(converted['vehicles'])
    manifest['tracks'].update(converted['tracks'])
    manifest['engineAudio'].update(audio['models'])
    manifest['propCatalogs']['germany'] = {'path': 'props/germany.json', 'catalogHash': props['catalogHash'], 'count': len(props['props'])}
    mime = {'.json': 'application/json', '.glb': 'model/gltf-binary', '.flac': 'audio/flac', '.webp': 'image/webp', '.png': 'image/png'}
    assets = []
    for file in sorted(PUBLIC.rglob('*')):
        if not file.is_file() or file == PUBLIC / 'manifest.json':
            continue
        data = file.read_bytes()
        assets.append({'path': file.relative_to(PUBLIC).as_posix(), 'bytes': len(data), 'sha256': sha(data), 'mime': mime[file.suffix]})
    manifest['assets'] = assets
    manifest['totalBytes'] = sum(item['bytes'] for item in assets)
    write(PUBLIC / 'manifest.json', encoded(manifest))
    terrain = load_tool('racing_content_terrain', 'nfs-terrain.py')
    sys.argv = ['nfs-terrain.py', '--out', str(DATA), '--client-out', str(WEB)]
    terrain.main()
    source_directory = ROOT / 'games/racing/assets/source/nfs4'
    write(source_directory / 'content-inputs.json', encoded(acquisition))
    write(source_directory / 'content-previews.json', encoded({'source': 'original NFS4 vehicle menu art and named original menu track models', 'images': preview_sources}))
    print(json.dumps({'passed': True, 'newVehicles': list(NAMES), 'newTrack': 'germany', 'previews': len(preview_sources), 'assets': len(assets), 'bytes': manifest['totalBytes']}))


if __name__ == '__main__':
    main()
