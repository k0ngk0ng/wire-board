import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
build = (ROOT / '.github/workflows/build.yml').read_text()
docker = (ROOT / 'Dockerfile').read_text()
release = (ROOT / '.github/workflows/publish-verified.yml').read_text()
prepare = 'python3 -B games/pal/build/prepare-server.py --out dist/package/pal'
assert build.count(prepare) == 1 and build.index(prepare) < build.index('python3 -B games/contra/build/native.py --runtime')
assert 'cp games/pal/native/upstream/LICENSE dist/package/docs/LICENSE.SDLPAL' in build
assert 'python3 -B games/pal/build/prepare-server.py --out .local/pal/container-runtime/pal' in docker
assert 'COPY --from=backend /src/.local/pal/container-runtime/pal /usr/local/bin/pal' in docker
assert 'COPY --from=backend /src/games/pal/native/upstream/LICENSE /usr/local/share/licenses/wire-board/LICENSE.SDLPAL' in docker
assert 'tar -xzf wire-board-linux-amd64.tar.gz -C image ./wire-board ./pal ./docs/LICENSE.SDLPAL ./docs/THIRD_PARTY.md' in release
assert "'COPY pal /usr/local/bin/pal'" in release
assert "'COPY docs/LICENSE.SDLPAL /usr/local/share/licenses/wire-board/LICENSE.SDLPAL'" in release
assert 'prepare-server.py' not in release and 'publish-server.py' not in release
runtime = 'FROM alpine:' + docker.split('FROM alpine:', 1)[1]
for source, target in {
    'COPY --from=backend /wire-board /usr/local/bin/wire-board': 'COPY wire-board /usr/local/bin/wire-board',
    'COPY --from=backend /src/.local/pal/container-runtime/pal /usr/local/bin/pal': 'COPY pal /usr/local/bin/pal',
    'COPY --from=backend /src/games/pal/native/upstream/LICENSE /usr/local/share/licenses/wire-board/LICENSE.SDLPAL': 'COPY docs/LICENSE.SDLPAL /usr/local/share/licenses/wire-board/LICENSE.SDLPAL',
    'COPY --from=backend /src/docs/THIRD_PARTY.md /usr/local/share/licenses/wire-board/THIRD_PARTY.md': 'COPY docs/THIRD_PARTY.md /usr/local/share/licenses/wire-board/THIRD_PARTY.md',
}.items():
    assert runtime.count(source) == 1
    runtime = runtime.replace(source, target)
assert '--from=backend' not in runtime and 'COPY pal /usr/local/bin/pal' in runtime
report = {'passed': True, 'scope': 'source build/release artifact contracts and exact runtime Dockerfile transformation; not actual image filesystem or container startup', 'serverRulesPreparedBeforeBinary': True, 'tagReusesPreparedRules': True, 'licenseIncluded': True, 'runtimeCopyCount': runtime.count('COPY ')}
(ROOT / '.local/pal/verification/release-package-contract.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
