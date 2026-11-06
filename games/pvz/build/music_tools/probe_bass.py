"""Offline BASS/MO3 probe; opens no output audio device."""
import ctypes as C
import hashlib
import json
from pathlib import Path
import struct
import wave

ROOT = Path.cwd() / ".local/pvz-music"
LIBRARY = ROOT / "tools/bass24-osx/libbass.dylib"
SOURCES = ROOT.parent / "pvz-research/extracted/sounds"
FREQ = 44100


class ChannelInfo(C.Structure):
    _fields_ = [(name, C.c_uint32) for name in ("freq", "chans", "flags", "ctype", "origres", "plugin", "sample")] + [("filename", C.c_char_p)]


def bind(library, name, restype, argtypes):
    fn = getattr(library, name)
    fn.restype = restype
    fn.argtypes = argtypes
    return fn


def main():
    lib = C.CDLL(str(LIBRARY))
    uint, uint64, pointer, boolean = C.c_uint32, C.c_uint64, C.c_void_p, C.c_int
    version = bind(lib, "BASS_GetVersion", uint, [])
    error = bind(lib, "BASS_ErrorGetCode", C.c_int, [])
    initialize = bind(lib, "BASS_Init", boolean, [C.c_int, uint, uint, pointer, pointer])
    free = bind(lib, "BASS_Free", boolean, [])
    load = bind(lib, "BASS_MusicLoad", uint, [uint, pointer, uint64, uint, uint, uint])
    music_free = bind(lib, "BASS_MusicFree", boolean, [uint])
    get_info = bind(lib, "BASS_ChannelGetInfo", boolean, [uint, C.POINTER(ChannelInfo)])
    tags = bind(lib, "BASS_ChannelGetTags", pointer, [uint, uint])
    get_length = bind(lib, "BASS_ChannelGetLength", uint64, [uint, uint])
    get_attribute = bind(lib, "BASS_ChannelGetAttribute", boolean, [uint, uint, C.POINTER(C.c_float)])
    get_data = bind(lib, "BASS_ChannelGetData", uint, [uint, pointer, uint])
    set_position = bind(lib, "BASS_ChannelSetPosition", boolean, [uint, uint64, uint])
    get_position = bind(lib, "BASS_ChannelGetPosition", uint64, [uint, uint])
    assert initialize(0, FREQ, 0, None, None), ("init", error())
    report = {"library": str(LIBRARY), "librarySHA256": hashlib.sha256(LIBRARY.read_bytes()).hexdigest(),
              "bassVersionHex": hex(version()), "device": 0, "decodeOnly": True, "sources": []}
    try:
        for filename in ("mainmusic.mo3", "mainmusic_hihats.mo3"):
            path = SOURCES / filename
            source_bytes = path.read_bytes()
            pcm = []
            records = []
            for repeat in range(2):
                encoded_path = str(path).encode()
                # PRESCAN provides durations/seeking; DECODE never plays output.
                handle = load(0, C.cast(C.c_char_p(encoded_path), pointer), 0, 0, 0x200000 | 0x20000, FREQ)
                assert handle, (filename, "load", error())
                try:
                    info = ChannelInfo()
                    assert get_info(handle, C.byref(info)), error()
                    orders = get_length(handle, 1)
                    assert 0 < orders < 2048, orders
                    tracks = 0
                    while tracks < 256:
                        value = C.c_float()
                        if not get_attribute(handle, 0x200 + tracks, C.byref(value)):
                            break
                        tracks += 1
                    assert 0 < tracks <= 256
                    title_pointer = tags(handle, 0x10000)
                    title = C.string_at(title_pointer).decode("latin1") if title_pointer else ""
                    order_pointer = tags(handle, 0x10002)
                    order_list = list(C.string_at(order_pointer, orders)) if order_pointer else []
                    assert set_position(handle, 0, 1), error()
                    needed = 3 * FREQ * info.chans * 2
                    buffer = C.create_string_buffer(needed)
                    read = get_data(handle, buffer, needed)
                    assert read != 0xffffffff and read == needed, (read, needed, error())
                    data = buffer.raw[:read]
                    assert any(data), filename
                    pcm.append(data)
                    peak = max(abs(value[0]) for value in struct.iter_unpack("<h", data))
                    records.append({"repeat": repeat, "metadata": {name: getattr(info, name) for name in ("freq", "chans", "flags", "ctype", "origres")},
                                    "title": title, "orderCount": orders, "trackerChannels": tracks, "orderList": order_list,
                                    "bytes": read, "peakPCM16": peak, "sha256": hashlib.sha256(data).hexdigest(),
                                    "endOrderRowPacked": get_position(handle, 1)})
                finally:
                    assert music_free(handle), error()
            assert pcm[0] == pcm[1], (filename, "non-deterministic decoding")
            wave_path = ROOT / "probe" / (Path(filename).stem + "-3s.wav")
            with wave.open(str(wave_path), "wb") as output:
                output.setnchannels(records[0]["metadata"]["chans"])
                output.setsampwidth(2)
                output.setframerate(FREQ)
                output.writeframes(pcm[0])
            assert path.read_bytes() == source_bytes
            report["sources"].append({"file": filename, "bytes": len(source_bytes), "sha256": hashlib.sha256(source_bytes).hexdigest(), "runs": records})
        report["status"] = "pass"
        (ROOT / "probe" / "result.json").write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps(report, indent=2))
    finally:
        free()


if __name__ == "__main__":
    main()
