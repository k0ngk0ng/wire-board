"""Measure the same Night module's original active-drum candidate orders."""
import json
from pathlib import Path
from scan_layers import checked_scan
from probe_bass import FREQ


def main():
    records = {}
    for order in (76, 77):
        row = checked_scan(order, "mainmusic.mo3")
        row.update(enabledChannels=list(range(30)), sourceRole="night-active-drums",
                   candidateOriginalTrigger="Music::UpdateMusicBurst 0x45b260, scheme2, even order76 / odd order77; caller to bind original-executable Evidence")
        records[str(order)] = row
        print(order, "loop", row["loopOrder"], "intro", row["introPCMFrames"], "first", row["firstCyclePCMFrames"], "loopFrames", row["loopPCMFrames"])
    (Path.cwd() / ".local/pvz-music/probe/night-active-loops.json").write_text(json.dumps({"sampleRate": FREQ, "channels": 2, "sampleFormat": "PCM16LE", "records": records}, indent=2) + "\n")


if __name__ == "__main__":
    main()
