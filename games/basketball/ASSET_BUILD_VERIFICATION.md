# Pinned basketball public assets in verified application builds

Failure inventory written before implementation:

- Cold build succeeds with missing/unpublished assets, stale different-version files, wrong source ROM/hash/MIME/length, redirected or failed downloads.
- Warm cache trusted without verifying every file; corrupt/missing/unlisted files leak into Vite; partial downloads become visible as a completed cache.
- Package override bypasses the committed pin, silently falls back to network after package failure, or follows symlinks/outside-checkout paths.
- Manifest URL/traversal/duplicate path escapes the intended bucket or public namespace; cache paths escape the worktree or overwrite source files.
- Source manifests/ROM/music WAV masters/private provenance become public; ordinary cached failures reuse another content identity.
- Production's global CDN base is replaced for unrelated games; explicit mirrors/local harness URLs are lost; an empty platform base unexpectedly enables CDN.
- Prepare hook introduces Python/install/extra-service requirements into Docker; tag rebuilds media instead of reusing the already verified main binary.
- Nondeterministic pin/generated report, unchecked binary local fallback, and source/pin changes reusing old assets.

Approved integration API: Node-only `prepareBasketballAssets(root, { packageDirectory? })` returns a verified generated public directory, for the existing public collector. This worker owns build helpers, compact pin, pure base resolver, scripts/tests and this report. Root owns Vite/workflow/UI lifecycle integration; audio adapter changes remain with its owner. No upload is performed here. Existing session authorization covers final release after required acceptance.

## Current pin after music76 and atlas acceptance

The current compact pin is82,718 bytes for448 objects, contentSHA256 `2d05c1115cfc05aa3f87ae6ad7a49bad9bec7357ec0dda3917480739a573d68d`, fileSHA256 `b190cbadf868b853a67e8fcab2a7c904204af142da9629900ec0d2574f5249c8`. Its offline package is `.local/basketball-asset-package/final-76-a`; the second package `final-76-b` is byte-identical. The current67-page postresult atlas and exact `music-076-intro-loop.mp3` delivery/timing metadata are included. The initial447-object records below are retained as history.

Node's filename test failed first for the actual76 name, then the allowlist gained an exact music76 exception; ordinary76.mp3/76-loop.mp3 and arbitrary other intro-loop names remain forbidden. All8 Node build/base cases and8 packaging cases pass. Real offline preparation, warm reuse, public collection and scoped dist verification passed with448 files; no upload occurred. Next build environment: `BASKETBALL_ASSET_PACKAGE=.local/basketball-asset-package/final-76-a`.

Current evidence: `.local/basketball-asset-package/final-result.json`, `final-76-node-tests.log`, `final-76-build-acceptance.log` and `final-76-local-verify.json`; the reusable collector result index `.local/basketball-asset-build-acceptance/result.json` now describes this new pin.

## Pinned source contract

`games/basketball/assets/release-manifest.json` contains only schema/game, original ROM SHA-256, content SHA-256, this game's immutable CDN base and ordered `{urlPath,sha256,bytes,mime}` public entries. The initial pin is82,545 bytes for447 objects; none of the254,366,026 media bytes is added to Git. The original package identity is recomputed by restoring each entry's mechanical `output=public+urlPath` field and using the packager's canonical JSON. A changed path, hash, MIME, byte length or prefix cannot retain the old identity.

Initial, now historical, content identity: `82b77475390f10555d1bcd7de67447eb0d21ddc6d7e69116ab7f14859e1bb367`. Pin file SHA-256: `aa85dd4fdfdae9c4aa26b503b91f5e23a753a45c563980d593dfbc0af566675a`. That initial snapshot is the verified candidate1 package with music59..66 and75. After music76 or other accepted public outputs change, package and repin before publishing the new content namespace.

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/pin-assets.py --package .local/basketball-asset-package/candidate-1 --output games/basketball/assets/release-manifest.json
```

The pin writer verifies the entire existing package first. It writes only a compact manifest inside the checkout; no media, original executable, source export tree or private provenance is committed by it.

## Build/cache lifecycle

`games/basketball/build/assets.mjs` exports:

- `prepareBasketballAssets(root, {packageDirectory?})`: synchronous hook for Vite's existing generated-public collector. Node runs the same module as a subprocess only when materialization is needed. No Python, external npm dependency, new executable or installation is needed in Docker's frontend stage.
- `materializeBasketballAssets(root, {packageDirectory?, fetchImpl?})`: asynchronous materialization; `fetchImpl` is an injected test transport, not a production URL override.
- `verifyBasketballAssets(root)`: checks the current pin's cache stamp and every public byte.
- `verifyBasketballPublicFiles(root, publicRoot, {exact?})`: verifies all pinned files. Application-dist mode allows other games and the separately verified basketball Wasm runtime, but rejects extra basketball media.
- `loadBasketballAssetPin(root)`: validates the committed source/content/URL/MIME/path contract.

The only generated cache path is `.local/basketball-asset-build/<contentSHA256>/public/`, with `build.json` beside `public`, never inside it. Files are validated on every reuse. Downloads go into a new sibling staging directory, use the pinned HTTPS CDN prefix only, reject redirects, and check response MIME, exact decoded length and SHA-256. At most four files download concurrently. Node fetch can expose decompressed bodies while preserving compressed Content-Length headers; the streamed decoded size/hash is authoritative in that case. A failed download or incomplete staging directory is never returned as a usable public root. A different pin never falls back to an older verified content directory.

`packageDirectory`, or the explicit `BASKETBALL_ASSET_PACKAGE` environment setting on the synchronous hook, is for offline local acceptance. The candidate's source identity, full public entry list and every public file must match the committed pin. A bad explicit candidate fails even when a valid cache exists; it does not silently use CDN or change the pinned source. Paths and files reject symlinks and escapes outside the checkout, including dangling symlinks.

The hook deliberately leaves `prepareBasketballRuntime` unchanged. Root integration can supply two generated roots to the existing collector:

```js
preparePublicAssets(root, [
  prepareBasketballRuntime(root),
  prepareBasketballAssets(root),
]);
```

The resulting public media paths remain `basketball/v1/graphics/...`, `basketball/v1/selection/...`, and `basketball/v1/audio/...`. They can enter Vite's dist and the existing Go embed/application build alongside Wasm. This worker's real collector acceptance verified all447 local fallback files, not just metadata or directory existence. Full production Vite/embedded-binary integration remains root's subsequent step.

## Per-game base and UI ownership

The pure `games/basketball/web/asset-base.ts` resolver exports `resolveBasketballAssetsBase(platformBase)`:

- Empty/root-only platform base retains local fallback.
- The known default-port HTTP(S) `cdn.ichenj.com` production host selects this game's pinned CDN namespace.
- Explicit local, mirror, custom-port and harness bases remain intact, apart from trailing slash normalization.

It never writes to global platform configuration. Board/selection/whole-graphics-bundle fallback integration belongs to root; audio adapter wiring belongs to its worker. Those files were not edited by this task. The game remains off by default and no registry entry changed.

## CI and publication ordering

No workflow or cache step was edited. The proposed optional frontend-job cache contains **only `.local/basketball-asset-build/`**, keyed by the release-manifest hash, after Node setup and before the frontend build. It must not cache the shared `.local/basketball` research/export tree. Correctness does not depend on an Actions cache hit: a cold checkout downloads the exact public pin and fails closed when files are unavailable, mismatched or unpublished. Every restored cached object is hashed again.

Final media publication proceeds under the existing session authorization after required acceptance, then main can run its real cold-cache check and produce the verified binary with all local fallbacks. The tag workflow still publishes that previously verified application artifact; it does not rebuild or redownload assets. No OSS upload or real CDN fetch occurred in this task, so a remote cold-cache success is not claimed before publication.

## Verification evidence

Tests were written before implementation. Initial missing-module/pin failures are retained in `.local/basketball-asset-package/build-red.log` and `base-red.log`. Additional failing cases were retained before their fixes: unlisted private basketball files in application-dist mode (`build-private-red.log`), and compressed transfer headers/custom-port mirrors (`build-transfer-red.log`). All seven final test cases pass.

```sh
node --experimental-strip-types --test scripts/basketball/asset-build.test.mjs scripts/basketball/asset-base.test.mjs
node scripts/basketball/accept_asset_build.mjs .local/basketball-asset-package/candidate-1
node games/basketball/build/verify-assets.mjs .local/basketball-asset-build-acceptance/collector-workspace/.local/web-public
```

The real offline synchronous hook completed from a cold generated cache, then warm reuse preserved file timestamps. The acceptance script used the existing `preparePublicAssets` in an isolated collector workspace, verified every pinned output hash, confirmed no package/provenance files or research extensions were copied, and preserved an independent public sentinel. It did not overwrite the active `.local/web-public` or `web/dist` during root's ongoing work.

Result index: `.local/basketball-asset-build-acceptance/result.json`; final tests and collector logs: `.local/basketball-asset-package/build-after.log` and `build-acceptance.log`. Source/game boundary checks also passed. CDN failure/success paths use injected transport responses; actual TLS/CDN availability and remote main/Actions runs remain unclaimed here.
