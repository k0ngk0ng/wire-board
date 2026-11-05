# Runtime resource descriptor: contract before export

- Retain the already frozen renderstate payload, flow algorithm and Identity.
  This is a separate identity-only descriptor, not another runtime/data owner.
- Include exactly rules, sprites, HUD, render metadata, all eight selected arena
  payloads and the arena-A auxiliary payload. Reject missing/duplicate IDs and
  paths in the consuming loader; no file wildcard may add unrelated client data.
- Every record has only id, logical_path, raw_json_sha256, raw_json_bytes,
  gzip_sha256 and gzip_bytes. No pixel/audio/JSON body, tile array, base64,
  arbitrary URL, timestamp or local absolute path enters this server metadata.
- Hash exact stored gzip bytes and exact decompressed JSON bytes including the
  final newline. Client object parse/stringify or one-sided trimming must not
  silently create another asset identity.
- Paths are immutable logical game keys below /yuyuhakusho/data/. They have no
  traversal, hostname, query, fragment or duplicate resource alias. The client
  resolves them through the versioned asset manifest/base, not a mutable URL.
- Export must be deterministic and a check-only invocation must not write.
  Freeze a real missing-descriptor contract failure before implementing export.
- Server code may embed this small descriptor in addition to the two explicitly
  named computational payloads; it must not embed any referenced client body.
