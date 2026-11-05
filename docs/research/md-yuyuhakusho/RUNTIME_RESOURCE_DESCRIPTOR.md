# Runtime resource identity descriptor

`games/yuyuhakusho/data/runtime-resource-descriptor.json` is a 3,737-byte plain
JSON identity document. Its exact SHA-256 is
`7977b2e32a824923f3037373ba318d70c72d58d447be5953fdb7677ce9c59b8c`.
It adds no client resource body to the server and does not change the frozen
renderstate algorithm, State ownership, computational payload or Identity.

Schema 1 contains only `schema` and `resources`. Every record has exactly:

```json
{
  "id": "rules",
  "logical_path": "/yuyuhakusho/data/chu-world-rules.json.gz",
  "raw_json_sha256": "...",
  "raw_json_bytes": 2103508,
  "gzip_sha256": "...",
  "gzip_bytes": 103902
}
```

The thirteen IDs are `rules`, `sprites`, `hud`, `render_meta`, `arena-default`,
`arena-2`, `arena-4`, `arena-6`, `arena-8`, `arena-a`, `arena-c`, `arena-e`, and
`arena-a-aux`. Arena 0 selects `arena-default`; A additionally selects its aux.
The descriptor is separate from the four common renderstate Identity hashes,
so selected background/aux assets have explicit expected byte identities too.

The root data loader may explicitly embed this small third metadata file with
the two computational payloads. Its Go decoder/API belongs to the root loader;
the descriptor does not add a renderstate decoding or ownership path. A network
checkpoint can bind the exact descriptor SHA. Client paths are logical keys to
resolve through the versioned asset manifest/base, not mutable CDN hostnames.

Both hashes describe actual bytes: gzip SHA/size describe the stored compressed
file; raw JSON SHA/size describe its exact decompressed bytes. Twelve current
resources change when JSON is parsed and re-stringified; `arena-default` already
has exactly that compact form without a final newline. A client must preserve
the verified source bytes for Go rather than trim or normalize all resources.
Serving `.json.gz` as an opaque gzip resource avoids transparent HTTP decoding
when the client intends to verify both stored-gzip and decompressed hashes.

Independently, checkpoint sprite `Present` words are uint64 and actual saves
contain values above JavaScript's safe-integer limit. Snapshot recovery must use
an opaque exact JSON string/bytes or an explicit lossless bitmap wire codec.
This descriptor cannot make a parsed-and-re-stringified checkpoint lossless.

The failure checklist, real missing-file red and four contract tests preceded
the exporter. The first exact-byte test wrongly assumed all thirteen resources
would change on re-stringification; its failure and frozen original test are
retained. The correction checks unchanged exact bytes for the already compact
source and changed bytes for the other twelve. No asset or exporter behavior
was altered to satisfy that assertion.

Reproduction:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/yuyuhakusho/export_runtime_resource_descriptor.py
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/yuyuhakusho/test_runtime_resource_descriptor.py
PYTHONDONTWRITEBYTECODE=1 python3 -B scripts/yuyuhakusho/export_runtime_resource_descriptor.py --check
```

The check-only form performs no writes. Its manifest and
`runtime-resource-descriptor-verification.json` retain exact output/source/test
bindings. Wasm binaries and unfinished audio assets are not among these thirteen
resources; their runtime/version and audio acceptance remain separate work.
