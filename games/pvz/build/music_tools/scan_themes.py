"""Locate native first backward order jumps with sample-exact MIXTIME positions."""
import ctypes as C
import hashlib
import json
from pathlib import Path
from probe_bass import bind, LIBRARY, SOURCES, FREQ

THEMES = {"day": 0, "night": 0x30, "pool": 0x5e, "fog": 0x7d, "roof": 0xb8,
          "menu": 0x98, "boss": 0x9e, "chooseSeeds": 0x7a}
FLAGS = 0x200000 | 0x20000 | 0x8000 | 0x200 | 4


def scan(start_order, block_frames, filename="mainmusic.mo3"):
    lib = C.CDLL(str(LIBRARY))
    U, Q, P, B = C.c_uint32, C.c_uint64, C.c_void_p, C.c_int
    init = bind(lib, "BASS_Init", B, [C.c_int, U, U, P, P])
    error = bind(lib, "BASS_ErrorGetCode", C.c_int, [])
    free = bind(lib, "BASS_Free", B, [])
    load = bind(lib, "BASS_MusicLoad", U, [U, P, Q, U, U, U])
    music_free = bind(lib, "BASS_MusicFree", B, [U])
    position = bind(lib, "BASS_ChannelGetPosition", Q, [U, U])
    set_position = bind(lib, "BASS_ChannelSetPosition", B, [U, Q, U])
    get_data = bind(lib, "BASS_ChannelGetData", U, [U, P, U])
    CALLBACK = C.CFUNCTYPE(None, U, U, U, P)
    set_sync = bind(lib, "BASS_ChannelSetSync", U, [U, U, Q, CALLBACK, P])
    assert init(0, FREQ, 0, None, None), error()
    path = str(SOURCES / filename).encode()
    handle = load(0, C.cast(C.c_char_p(path), P), 0, 0, FLAGS, FREQ)
    assert handle, error()
    state = {"last": None, "loop": None, "first": {}, "timeline": [], "callbackErrors": [], "backwardJumps": []}

    @CALLBACK
    def callback(sync, channel, data, user):
        try:
            order, row = data & 0xffff, data >> 16
            byte = position(channel, 0x10000000)
            current = {"order": order, "row": row, "decodedBytePosition": byte, "decodedPCMFrame": byte // 4}
            previous = state["last"]
            backward = previous is not None and (order, row) < (previous["order"], previous["row"])
            if state["loop"] is None and backward:
                origin = state["first"].get((order, row))
                state["backwardJumps"].append(current | {"targetPreviouslyObserved": origin is not None})
                if origin is not None:
                    state["loop"] = current
                    state["loopFirstOccurrence"] = origin
            if previous is None or order != previous["order"] or backward:
                state["timeline"].append(current)
            state["first"].setdefault((order, row), current)
            state["last"] = current
        except BaseException as exc:
            state["callbackErrors"].append(repr(exc))

    decoded = 0
    try:
        assert set_sync(handle, 10 | 0x40000000, 0xffffffff, callback, None), error()
        assert set_position(handle, start_order, 1), error()
        buffer = C.create_string_buffer(block_frames * 4)
        limit = 400 * FREQ * 4
        while decoded < limit and state["loop"] is None:
            read = get_data(handle, buffer, min(len(buffer), limit - decoded))
            assert read not in (0, 0xffffffff), (read, error(), state)
            decoded += read
        assert not state["callbackErrors"], state["callbackErrors"]
        first = state["timeline"][0]
        assert first["order"] == start_order and first["row"] == 0, first
        assert all(event["decodedBytePosition"] % 4 == 0 for event in state["timeline"])
        result = {"startOrder": start_order, "startRow": 0, "blockFrames": block_frames,
                  "nativeBASSflags": FLAGS, "nativeBASSflagsHex": hex(FLAGS), "decodeLimitSeconds": 400,
                  "sourceFile": filename, "sourceSHA256": hashlib.sha256((SOURCES / filename).read_bytes()).hexdigest(),
                  "decodedPCMFramesIncludingFinalRead": decoded // 4, "orderChangeTimeline": state["timeline"], "backwardJumps": state["backwardJumps"]}
        if state["loop"] is not None and state.get("loopFirstOccurrence") is not None:
            loop, origin = state["loop"], state["loopFirstOccurrence"]
            assert loop["decodedBytePosition"] > origin["decodedBytePosition"] >= first["decodedBytePosition"], (first, origin, loop)
            result.update(status="observed", loopOrder=loop["order"], loopRow=loop["row"],
                          introPCMFrames=(origin["decodedBytePosition"] - first["decodedBytePosition"]) // 4,
                          firstCyclePCMFrames=(loop["decodedBytePosition"] - first["decodedBytePosition"]) // 4,
                          loopPCMFrames=(loop["decodedBytePosition"] - origin["decodedBytePosition"]) // 4)
        else:
            result.update(status="unknown", reason="No backward order jump to an observed order/row within 400 seconds")
        return result
    finally:
        music_free(handle)
        free()


def main():
    rows = {}
    for name, order in THEMES.items():
        runs = [scan(order, block) for block in (65536, 1024)]
        for key in ("status", "loopOrder", "loopRow", "introPCMFrames", "firstCyclePCMFrames", "loopPCMFrames", "orderChangeTimeline"):
            assert runs[0].get(key) == runs[1].get(key), (name, key, runs)
        rows[name] = runs[0] | {"verificationBlockFrames": runs[1]["blockFrames"], "verifiedExactBoundaryAcrossBlockSizes": True}
        print(name, {key: rows[name].get(key) for key in ("status", "loopOrder", "loopRow", "introPCMFrames", "firstCyclePCMFrames", "loopPCMFrames")}, flush=True)
    output = {"sampleRate": FREQ, "channels": 2, "sampleFormat": "PCM16LE", "bytesPerFrame": 4,
              "bassVersion": "2.4.18.3", "librarySHA256": hashlib.sha256(LIBRARY.read_bytes()).hexdigest(),
              "source": str(SOURCES / "mainmusic.mo3"), "sourceSHA256": hashlib.sha256((SOURCES / "mainmusic.mo3").read_bytes()).hexdigest(),
              "boundaryMethod": "MUSICPOS|MIXTIME callback, wildcard order+row, BASS_POS_BYTE|BASS_POS_DECODE; verified at 65536/1024 PCM-frame reads",
              "themes": rows}
    (Path.cwd() / ".local/pvz-music/probe/theme-loops.json").write_text(json.dumps(output, indent=2) + "\n")


if __name__ == "__main__":
    main()
