from concurrent.futures import ThreadPoolExecutor
import gzip
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import threading
import unittest
from urllib.parse import urlsplit
from urllib.request import Request, urlopen
import zlib

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'scripts'))
import asset_reuse as reuse
import cdn_delivery as cdn

WORK = ROOT / '.local/pokepath/verification/native-delivery'
WORK.mkdir(parents=True, exist_ok=True)
SOURCE = Path(os.environ.get('WIRE_BOARD_POKEPATH_DELIVERY_WEB', str(ROOT / '.local/pokepath/native-web'))).resolve()
OLD = 'https://cdn.ichenj.com/bga/web/' + 'a' * 40 + '/'
NEW = 'https://cdn.ichenj.com/bga/web/' + 'b' * 40 + '/'
THIRD = 'https://cdn.ichenj.com/bga/web/' + 'c' * 40 + '/'
PREFIX = 'pokepath/v1/'
RESULTS = []


def png_variant(data):
    if data[:8] != b'\x89PNG\r\n\x1a\n' or data[-8:-4] != b'IEND':
        raise ValueError('Expected a canonical PNG fixture')
    body = b'wire-board-native-delivery\0second-version'
    chunk = b'tEXt' + body
    encoded = struct.pack('>I', len(body)) + chunk + struct.pack('>I', zlib.crc32(chunk))
    return data[:-12] + encoded + data[-12:]


def canonical_records(dist, base):
    manifest_path = dist / 'game-load-manifest.json'
    manifest = json.loads(manifest_path.read_bytes())
    manifest['assetBase'] = base
    manifest['entry'] = base + manifest['entry'].lstrip('/')
    for group in [*manifest['games'].values(), manifest.get('home', {}).get('files', [])]:
        for record in group:
            record['url'] = base + record['url'].lstrip('/')
    for name in ('assetMap', 'assetObjects', 'assetPlan', 'entryPath', 'mediaBase'):
        manifest.pop(name, None)
    manifest_path.write_text(json.dumps(manifest, separators=(',', ':')) + '\n')
    index = dist / 'index.html'
    source = index.read_text().replace('src="/assets/', 'src="' + base + 'assets/').replace('href="/assets/', 'href="' + base + 'assets/')
    index.write_text(source)


class NativeDeliveryE2E(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not SOURCE.is_relative_to(ROOT) or SOURCE.is_symlink():
            raise RuntimeError('Native publication input must remain inside this workspace')
        if not (SOURCE / 'pokepath/v1/native/engine.wasm').is_file():
            raise RuntimeError('Build the independent native web artifact before running delivery acceptance')
        cls.directory = tempfile.TemporaryDirectory(dir=WORK)
        cls.root = Path(cls.directory.name)
        environment = dict(os.environ)
        for key, value in [('GOCACHE', '.local/go-build'), ('GOMODCACHE', '.local/go-mod'), ('GOTMPDIR', '.local/tmp'), ('TMPDIR', '.local/tmp'), ('TEST_TELEMETRY_DIR', '.local/go-telemetry')]:
            environment[key] = str(ROOT / value)
            Path(environment[key]).mkdir(parents=True, exist_ok=True)
        environment['GOTOOLCHAIN'] = 'local'
        cls.environment = environment
        runtime = json.loads(subprocess.check_output(['node', 'games/pokepath/build/browser-runtime.mjs'], cwd=ROOT, env=environment, text=True))
        cls.runtime_public = Path(runtime['publicRoot'])
        cls.runtime_identity = runtime
        cls.revision = json.loads((ROOT / 'games/pokepath/data/rules-revision.json').read_bytes())
        required_revision = os.environ.get('WIRE_BOARD_POKEPATH_DELIVERY_REVISION', cls.revision['rulesRevision'])
        required_facts = os.environ.get('WIRE_BOARD_POKEPATH_DELIVERY_FACTS', cls.revision['factsSHA256'])
        if cls.revision['rulesRevision'] != required_revision or cls.revision['factsSHA256'] != required_facts:
            raise RuntimeError('The current source does not match the requested final rules and facts identities')
        source_manifest = json.loads((SOURCE / PREFIX / 'native/manifest.json').read_bytes())
        source_revision = json.loads((SOURCE / PREFIX / 'rules-revision.json').read_bytes())
        if source_manifest['sourceSHA256'] != runtime['sourceSHA256'] or source_manifest['buildSHA256'] != runtime['buildSHA256']:
            raise RuntimeError('The native web artifact does not contain the frozen browser runtime; rebuild it before final delivery acceptance')
        if source_revision['rulesRevision'] != cls.revision['rulesRevision'] or source_revision['factsSHA256'] != cls.revision['factsSHA256']:
            raise RuntimeError('The native web artifact does not contain the frozen engine and facts revisions')
        cls.template = cls.root / 'template'
        shutil.copytree(SOURCE, cls.template)
        runtime_manifest = json.loads((cls.template / PREFIX / 'native/manifest.json').read_bytes())
        resources = (cls.template / PREFIX / 'resources.json').read_bytes()
        if runtime_manifest['catalog']['manifest']['sha256'] != cdn.sha(resources):
            raise RuntimeError('Publication inputs changed during fixture capture; retry with a stable asset inventory')
        cls.objects = {}
        cls.aliases = {}
        cls.requests = []
        store = cls.objects
        aliases = cls.aliases
        requests = cls.requests

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                path = urlsplit(self.path).path
                requests.append({'path': path, 'range': self.headers.get('Range')})
                if path.startswith('/_media/'):
                    record = aliases.get('/' + path.removeprefix('/_media/'))
                    if record is None:
                        self.send_error(404)
                        return
                    self.send_response(307)
                    self.send_header('Location', cls.origin + urlsplit(record['url']).path)
                    self.end_headers()
                    return
                value = store.get(path)
                if value is None:
                    self.send_error(404)
                    return
                body, headers = value
                range_value = self.headers.get('Range')
                match = re.fullmatch(r'bytes=(\d+)-(\d+)', range_value or '')
                if match:
                    start, end = map(int, match.groups())
                    if start > end or end >= len(body) or headers.get('Content-Encoding'):
                        self.send_error(416)
                        return
                    self.send_response(206)
                    self.send_header('Content-Range', f'bytes {start}-{end}/{len(body)}')
                    body = body[start:end + 1]
                else:
                    self.send_response(200)
                for key, item in headers.items():
                    self.send_header(key, item)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                return

        cls.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        cls.origin = 'http://127.0.0.1:' + str(cls.server.server_port)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=5)
        cls.directory.cleanup()

    def copy(self, name, base):
        destination = self.root / name
        shutil.copytree(self.template, destination)
        canonical_records(destination, base)
        return destination

    def publish(self, dist, baseline):
        summary = reuse.prepare(dist, baseline)
        delivery = json.loads((dist / 'cdn-assets-manifest.json').read_bytes())
        rows = cdn.inventory(dist, delivery['assetBase'])
        uploaded = []
        for row in rows:
            key = urlsplit(row['url']).path
            body = (dist / row['path']).read_bytes()
            if row['encoding']:
                body = gzip.compress(body, mtime=0)
            headers = {'Content-Type': row['mime'], 'Access-Control-Allow-Origin': '*'}
            if row['encoding']:
                headers['Content-Encoding'] = row['encoding']
            if key in self.objects:
                self.assertEqual(self.objects[key], (body, headers), 'immutable object changed')
            else:
                self.objects[key] = (body, headers)
                uploaded.append(row['url'])
        self.aliases.clear()
        self.aliases.update(delivery['files'])
        self.assertEqual(len(uploaded), summary['newObjects'])
        return summary, delivery, rows, uploaded

    def read(self, row, logical=None):
        address = self.origin + ('/_media/' + logical if logical is not None else urlsplit(row['url']).path)
        with urlopen(address, timeout=10) as response:
            body = response.read()
            headers = dict(response.headers)
        cdn.validate_response(row, body, headers)
        return cdn.decoded(body, headers)

    def bootstrap(self, delivery):
        start = len(self.requests)
        files = delivery['files']
        root = '/pokepath/v1/'
        manifest = json.loads(self.read(files[root + 'native/manifest.json'], PREFIX + 'native/manifest.json'))
        self.assertEqual(manifest['engine'], 'wire-board-pokepath-go')
        self.assertEqual(manifest['abi'], 'wire-board-pokepath-go-v1')
        dependencies = {item['module']: item for item in manifest.get('dependencies', [])}
        self.assertEqual(set(dependencies), {'golang.org/x/text'})
        self.assertEqual(dependencies['golang.org/x/text']['version'], 'v0.34.0')
        self.assertEqual(dependencies['golang.org/x/text']['license'], 'BSD-3-Clause')
        for item in manifest['sourceFiles']:
            dependency = dependencies.get(item.get('dependency'))
            own = item['path'] in ('go.mod', 'go.sum') or item['path'].startswith(('games/pokepath/engine/', 'games/pokepath/cmd/browser/'))
            self.assertTrue(own or dependency and item['path'].startswith(dependency['sourcePrefix']))
            self.assertNotIn('upstream', item['path'])
            self.assertNotIn('reference', item['path'])
        metadata = self.read(files[root + 'resources.json'], PREFIX + 'resources.json')
        expected = manifest['catalog']['manifest']
        self.assertEqual(len(metadata), expected['bytes'])
        self.assertEqual(cdn.sha(metadata), expected['sha256'])
        resource_index = json.loads(metadata)
        index = {item['path']: item for item in resource_index['files']}
        aliases = resource_index.get('aliases', {})
        for item in manifest['files']:
            logical = PREFIX + 'native/' + item['path']
            body = self.read(files['/' + logical], logical)
            self.assertEqual(len(body), item['bytes'])
            self.assertEqual(cdn.sha(body), item['sha256'])
            if item['path'] == 'engine.wasm':
                self.assertEqual(body[:8], b'\x00asm\x01\x00\x00\x00')
        core_bytes = self.read(files[root + 'catalog.json'], PREFIX + 'catalog.json')
        route_bytes = self.read(files[root + 'routes/0.json'], PREFIX + 'routes/0.json')
        for path, body in [('catalog.json', core_bytes), ('routes/0.json', route_bytes)]:
            self.assertEqual(len(body), index[path]['bytes'])
            self.assertEqual(cdn.sha(body), index[path]['sha256'])
        core = json.loads(core_bytes)
        route = json.loads(route_bytes)
        revision = json.loads(self.read(files[root + 'rules-revision.json'], PREFIX + 'rules-revision.json'))
        self.assertEqual(revision['rulesRevision'], self.revision['rulesRevision'])
        self.assertEqual(revision['factsSHA256'], self.revision['factsSHA256'])
        self.assertEqual(core['source']['sourceSha256'], self.revision['factsSHA256'])
        self.assertEqual(len(core['tutorials']), 29)
        self.assertEqual(len(core['themes']), 4)
        self.assertEqual(route['id'], 0)
        image = core['species']['charmander']['sprite']['image']
        paths = [route['background'], image, 'src/assets/fonts/NGC.ttf', 'src/assets/sounds/ui/open.wav']
        for path in paths:
            path = path.removeprefix('./')
            path = aliases.get(path, path)
            record = files[root + path]
            body = self.read(record, PREFIX + path)
            self.assertEqual(len(body), index[path]['bytes'])
            self.assertEqual(cdn.sha(body), index[path]['sha256'])
        requests = self.requests[start:]
        logical_routes = [item['path'] for item in requests if item['path'].startswith('/_media/pokepath/v1/routes/')]
        self.assertEqual(logical_routes, ['/_media/pokepath/v1/routes/0.json'])
        return {'requests': requests, 'sources': sorted({urlsplit(files[item['path'].replace('/_media', '')]['url']).path.split('/')[3] for item in requests if item['path'].startswith('/_media/')})}

    def changed_media(self, dist, rename=False):
        root = dist / PREFIX
        logical = 'src/assets/images/icons/heart.png'
        index_path = root / 'resources.json'
        index = json.loads(index_path.read_bytes())
        item = next(item for item in index['files'] if item['path'] == logical)
        image = root / logical
        if rename:
            new_path = 'src/assets/images/icons/heart-renamed.png'
            image.rename(root / new_path)
            item['path'] = new_path
        else:
            image.write_bytes(png_variant(image.read_bytes()))
            item.update(bytes=image.stat().st_size, sha256=cdn.sha(image.read_bytes()))
        index_path.write_text(json.dumps(index, separators=(',', ':')) + '\n')
        manifest_path = root / 'native/manifest.json'
        manifest = json.loads(manifest_path.read_bytes())
        manifest['catalog']['manifest'].update(bytes=index_path.stat().st_size, sha256=cdn.sha(index_path.read_bytes()))
        manifest['buildSHA256'] = cdn.sha(json.dumps({'fixture': 'resource-publication', 'originalBuild': manifest['buildSHA256'], 'catalog': manifest['catalog']}, sort_keys=True).encode())
        manifest_path.write_text(json.dumps(manifest, indent=2) + '\n')
        return logical

    def compile_package(self, dist, report, payloads):
        output = self.root / 'package'
        output.mkdir()
        replacement = {str(path): '' for path in (ROOT / 'web/dist').rglob('*') if path.is_file()}
        for path in dist.rglob('*'):
            if path.is_file():
                replacement[str(ROOT / 'web/dist' / path.relative_to(dist))] = str(path)
        overlay = output / 'overlay.json'
        overlay.write_text(json.dumps({'Replace': replacement}))
        environment = dict(self.environment, CGO_ENABLED='0', GOOS='linux', GOARCH='amd64', GOFLAGS='', GOMAXPROCS='2')
        listing = subprocess.check_output(['go', 'list', '-overlay', str(overlay), '-deps', '-json', '.'], cwd=ROOT, env=environment, text=True)
        decoder = json.JSONDecoder()
        position = 0
        inputs = {}
        embedded = {}
        while position < len(listing):
            package, consumed = decoder.raw_decode(listing[position:])
            position += consumed
            while position < len(listing) and listing[position].isspace():
                position += 1
            if package.get('Standard'):
                continue
            directory = Path(package['Dir'])
            for name in package.get('EmbedFiles', []):
                logical = directory / name
                actual = Path(replacement.get(str(logical), str(logical)))
                if directory == ROOT and name.startswith('web/dist/'):
                    embedded[name] = str(actual.relative_to(ROOT))
            for name in [*package.get('GoFiles', []), *package.get('SFiles', []), *package.get('EmbedFiles', [])]:
                logical = directory / name
                actual = Path(replacement.get(str(logical), str(logical)))
                if not actual.is_relative_to(ROOT):
                    continue
                inputs[actual.relative_to(ROOT).as_posix()] = cdn.sha(actual.read_bytes())
        self.assertEqual(sorted(embedded), ['web/dist/' + name for name in sorted(cdn.SERVER_METADATA) if (dist / name).is_file()])
        binary = output / 'wire-board'
        subprocess.run(['go', 'build', '-overlay', str(overlay), '-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-o', str(binary), '.'], cwd=ROOT, env=environment, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        body = binary.read_bytes()
        for name, payload in payloads.items():
            self.assertNotIn(payload, body, f'compiled server contains {name}')
        receipt = {'sha256': cdn.sha(body), 'bytes': len(body), 'inputs': [{'path': name, 'sha256': value} for name, value in sorted(inputs.items())], 'logicalEmbedMapping': embedded, 'command': 'go build -overlay <repo-local-overlay> -trimpath -buildvcs=false -ldflags=-s -w -o <fixture-binary> .'}
        binary.with_suffix('.build.json').write_text(json.dumps(receipt, indent=2) + '\n')
        result = cdn.verify_server_package(ROOT, dist, binary, report)
        result['logicalEmbedMapping'] = embedded
        result['publicPayloadProbesAbsent'] = list(payloads)
        (WORK / 'server-package.json').write_text(json.dumps(result, indent=2) + '\n')
        (WORK / 'server-compiler-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
        return result

    def test_actual_native_artifacts_two_versions_alias_loading_dedup_and_server_strip(self):
        first = self.copy('first', OLD)
        published = [path.relative_to(first).as_posix() for path in (first / PREFIX).rglob('*') if path.is_file()]
        programs = [path for path in published if path.endswith(('.js', '.css', '.html'))]
        self.assertEqual(programs, ['pokepath/v1/native/wasm_exec.js'])
        first_plan, baseline, first_rows, first_uploads = self.publish(first, {})
        first_boot = self.bootstrap(baseline)
        second = self.copy('second', NEW)
        changed = self.changed_media(second)
        second_plan, current, rows, uploads = self.publish(second, baseline)
        second_boot = self.bootstrap(current)
        self.assertEqual(second_plan['newObjects'], 3)
        self.assertNotEqual(current['files']['/' + PREFIX + changed]['url'], baseline['files']['/' + PREFIX + changed]['url'])
        aliases = json.loads((second / PREFIX / 'resources.json').read_bytes()).get('aliases', {})
        audio_path = aliases.get('src/assets/sounds/ui/open.wav', 'src/assets/sounds/ui/open.wav')
        for path in ['native/engine.wasm', 'native/wasm_exec.js', 'catalog.json', 'routes/0.json', audio_path, 'src/assets/sounds/music/route1.mp3', 'src/assets/fonts/NGC.ttf']:
            self.assertEqual(current['files']['/' + PREFIX + path]['url'], baseline['files']['/' + PREFIX + path]['url'])
        self.assertEqual(set(second_boot['sources']), {'a' * 40, 'b' * 40})
        repeat = self.copy('repeat', THIRD)
        repeat_plan, repeated, repeat_rows, repeat_uploads = self.publish(repeat, baseline)
        self.assertEqual(repeat_plan['newObjects'], 0)
        self.assertEqual(repeat_plan['uploadBytes'], 0)
        self.assertFalse(repeat_uploads)
        renamed = self.copy('renamed', THIRD)
        self.changed_media(renamed, rename=True)
        rename_plan, renamed_delivery, rename_rows, rename_uploads = self.publish(renamed, baseline)
        self.assertEqual(renamed_delivery['files']['/' + PREFIX + 'src/assets/images/icons/heart-renamed.png']['url'], baseline['files']['/' + PREFIX + changed]['url'])
        self.assertEqual(rename_plan['newObjects'], 2)
        self.aliases.clear()
        self.aliases.update(current['files'])
        current_by_path = {row['path']: row for row in rows}
        selected = ['native/engine.wasm', 'native/wasm_exec.js', 'resources.json', 'catalog.json', 'routes/46.json', 'src/assets/images/icons/heart.png', 'src/assets/fonts/NGC.ttf', 'src/assets/icon.ico']
        for path in selected:
            self.read(current_by_path[PREFIX + path], PREFIX + path)
        audio_row = current_by_path[PREFIX + audio_path]
        expected = (second / audio_row['path']).read_bytes()[:64]
        with urlopen(Request(self.origin + urlsplit(audio_row['url']).path, headers={'Range': 'bytes=0-63'}), timeout=10) as response:
            cdn.validate_range_response(audio_row, expected, response.read(), dict(response.headers), response.status)
        with ThreadPoolExecutor(max_workers=8) as pool:
            list(pool.map(self.read, rows))
        report = {'passed': True, 'verificationLocation': 'loopback-fixture', 'simulated': True, 'files': rows, 'indexSHA256': cdn.sha((second / 'index.html').read_bytes()), 'manifestSHA256': cdn.sha((second / 'game-load-manifest.json').read_bytes()), 'cdnManifestSHA256': cdn.sha((second / 'cdn-assets-manifest.json').read_bytes())}
        self.assertEqual(cdn.inventory(second, NEW), rows)
        if (second / 'contra-delivery.json').is_file():
            report['contraDeliverySHA256'] = cdn.sha((second / 'contra-delivery.json').read_bytes())
        (WORK / 'current-report.json').write_text(json.dumps(report, indent=2) + '\n')
        probes = {name: (second / PREFIX / name).read_bytes() for name in ['native/engine.wasm', 'native/wasm_exec.js', 'src/assets/images/maps/1-1.png', 'src/assets/fonts/NGC.ttf', audio_path]}
        cdn.trim(second, report)
        remaining = sorted(path.relative_to(second).as_posix() for path in second.rglob('*') if path.is_file())
        self.assertEqual(remaining, sorted({'cdn-assets-manifest.json', 'game-load-manifest.json', 'index.html'} | ({'contra-delivery.json'} if 'contraDeliverySHA256' in report else set())))
        packaged = self.compile_package(second, report, probes)
        RESULTS.append({'name': 'actual-native-two-version-delivery', 'passed': True, 'source': str(SOURCE.relative_to(ROOT)), 'sourceIndexSHA256': cdn.sha((SOURCE / 'index.html').read_bytes()), 'runtimeSourceSHA256': self.runtime_identity['sourceSHA256'], 'runtimeBuildSHA256': self.runtime_identity['buildSHA256'], 'rulesRevision': self.revision['rulesRevision'], 'factsSHA256': self.revision['factsSHA256'], 'tutorials': 29, 'themes': 4, 'first': first_plan, 'changedMedia': second_plan, 'unchangedBuild': repeat_plan, 'renamedMedia': rename_plan, 'newURLs': uploads, 'renamedNewURLs': rename_uploads, 'firstBoot': first_boot, 'secondBoot': second_boot, 'publishedFiles': len(first_rows), 'nativeFiles': len(published), 'programPayloads': programs, 'httpVerifiedSamples': selected, 'httpVerifiedFiles': len(rows), 'audioRangeVerified': True, 'remainingServerFiles': remaining, 'compiledServerPackage': packaged})
        (WORK / 'delivery-first.json').write_text(json.dumps(baseline, indent=2) + '\n')
        (WORK / 'delivery-second.json').write_text(json.dumps(current, indent=2) + '\n')

    def test_new_client_mime_formats_are_precise_and_response_failures_are_rejected(self):
        dist = self.root / 'formats'
        (dist / 'assets').mkdir(parents=True)
        (dist / 'index.html').write_text('entry')
        (dist / 'assets/app.js').write_text('export const ready=true;')
        for path, data in [('game/icon.ico', b'\x00\x00\x01\x00native-icon'), ('game/font.otf', b'OTTO-native-font')]:
            file = dist / path
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_bytes(data)
        rows = {row['path']: row for row in cdn.inventory(dist, NEW)}
        self.assertEqual(rows['game/icon.ico']['mime'], 'image/x-icon')
        self.assertEqual(rows['game/font.otf']['mime'], 'font/otf')
        for row in rows.values():
            body = (dist / row['path']).read_bytes()
            encoded = gzip.compress(body, mtime=0) if row['encoding'] else body
            headers = {'Content-Type': row['mime'], 'Access-Control-Allow-Origin': '*'}
            if row['encoding']:
                headers['Content-Encoding'] = 'gzip'
            cdn.validate_response(row, encoded, headers)
            for invalid in [dict(headers, **{'Content-Type': 'application/octet-stream'}), {key: value for key, value in headers.items() if key != 'Access-Control-Allow-Origin'}]:
                with self.assertRaises(ValueError):
                    cdn.validate_response(row, encoded, invalid)
            with self.assertRaises(ValueError):
                cdn.validate_response(row, encoded + b'changed', headers)
        RESULTS.append({'name': 'client-mime-and-negative-response', 'passed': True, 'formats': {'ico': rows['game/icon.ico']['mime'], 'otf': rows['game/font.otf']['mime']}})


if __name__ == '__main__':
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(NativeDeliveryE2E))
    (WORK / 'result.json').write_text(json.dumps({'command': 'python3 -B games/pokepath/tests/native-delivery-e2e.py', 'passed': result.wasSuccessful(), 'tests': result.testsRun, 'failures': len(result.failures), 'errors': len(result.errors), 'scenarios': RESULTS}, indent=2) + '\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
