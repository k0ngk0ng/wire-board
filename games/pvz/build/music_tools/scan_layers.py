"""Revalidate native flags and scan all requested main/drum/hihat role timings."""
import hashlib
import json
from pathlib import Path
from probe_bass import LIBRARY, SOURCES, FREQ
from scan_themes import THEMES, FLAGS, scan


KEYS = ("status", "loopOrder", "loopRow", "introPCMFrames", "firstCyclePCMFrames", "loopPCMFrames", "orderChangeTimeline")


def checked_scan(start, filename):
    runs = [scan(start, block, filename) for block in (65536, 1024)]
    for key in KEYS:
        assert runs[0].get(key) == runs[1].get(key), (filename, start, key, runs)
    assert runs[0]["status"] == "observed", (filename, start, runs)
    out = runs[0] | {"verificationBlockFrames": runs[1]["blockFrames"], "verifiedExactBoundaryAcrossBlockSizes": True}
    out.update(introSeconds=out["introPCMFrames"] / FREQ, firstCycleSeconds=out["firstCyclePCMFrames"] / FREQ,
               loopSeconds=out["loopPCMFrames"] / FREQ)
    return out


def main():
    previous = json.loads((Path.cwd() / ".local/pvz-music/probe/theme-loops.json").read_text())
    themes = {}
    for name, start in THEMES.items():
        main = checked_scan(start, "mainmusic.mo3")
        for key in KEYS:
            assert main.get(key) == previous["themes"][name].get(key), (name, key, "native flags changed prior boundary")
        themes[name] = {"main": main}
        print(name, "main", main["introPCMFrames"], main["firstCyclePCMFrames"], flush=True)
    # Main and drums use the same decoder/file/order for these four scenes.
    for name in ("day", "pool", "fog", "roof"):
        themes[name]["drums"] = themes[name]["main"] | {"reusedTimingFrom": name + ".main"}
        hats = checked_scan(THEMES[name], "mainmusic_hihats.mo3")
        themes[name]["hihat"] = hats
        print(name, "hihat", hats["introPCMFrames"], hats["firstCyclePCMFrames"], flush=True)
    themes["night"]["drums"] = checked_scan(92, "mainmusic.mo3")
    night = themes["night"]["drums"]
    print("night", "drums", night["loopOrder"], night["introPCMFrames"], night["firstCyclePCMFrames"], flush=True)
    # These role masks are supplied by the parent's original-executable REA;
    # they are declared here solely to bind timing records to intended stems.
    masks = {
        "day": {"main": list(range(24)), "drums": [24, 25, 26], "hihat": [27]},
        "night": {"main": list(range(30)), "drums": list(range(30))},
        "pool": {"main": list(range(18)), "drums": list(range(18, 29)), "hihat": list(range(18, 25)) + [29]},
        "fog": {"main": list(range(16)), "drums": list(range(16, 23)), "hihat": [23]},
        "roof": {"main": list(range(18)), "drums": [18, 19, 20], "hihat": [21]},
    }
    for name, roles in themes.items():
        for role, record in roles.items():
            record["enabledChannels"] = masks.get(name, {}).get(role, list(range(30)))
            record["initialVolume"] = 1 if role == "main" or name == "night" and role == "drums" else 0
    output = {"status": "pass", "sampleRate": FREQ, "channels": 2, "sampleFormat": "PCM16LE", "bytesPerFrame": 4,
              "originalMusicFlags": 0x8204, "nativeBASSflags": FLAGS, "nativeBASSflagsHex": hex(FLAGS),
              "bassVersion": "2.4.18.3", "librarySHA256": hashlib.sha256(LIBRARY.read_bytes()).hexdigest(),
              "sources": {name: {"file": str(SOURCES / name), "sha256": hashlib.sha256((SOURCES / name).read_bytes()).hexdigest()}
                          for name in ("mainmusic.mo3", "mainmusic_hihats.mo3")},
              "boundaryMethod": "MUSICPOS|MIXTIME wildcard callback + BASS_POS_BYTE|DECODE; exact positions identical for 65536 and 1024-frame requests; prior main boundaries also revalidated",
              "themes": themes}
    (Path.cwd() / ".local/pvz-music/probe/theme-loops-v2.json").write_text(json.dumps(output, indent=2) + "\n")


if __name__ == "__main__":
    main()
