import argparse
import binascii
import hashlib
import json
import re
import struct
import sys
import zlib
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[2]
DEFAULT_WAD = ROOT / ".local/hotlinemiami/research/original/gog-linux-2.0.0.4/selected/HotlineMiami_GL.wad"
EXPECTED_SHA256 = "a9499384a2a30a969f1db67e9f9e6d767a48a031272cdef987f349a8d7095e40"
FIXUP_MACHINE = None


class PhyreFixupMachine:
    def __init__(self):
        sys.path.insert(0, str(ROOT / ".local/hotlinemiami/tools/python"))
        from elftools.elf.elffile import ELFFile
        from unicorn import Uc, UC_ARCH_X86, UC_MODE_32
        from unicorn.x86_const import UC_X86_REG_ESP, UC_X86_REG_EAX, UC_X86_REG_EIP
        binary = DEFAULT_WAD.parent / "Hotline"
        if hashlib.sha256(binary.read_bytes()).hexdigest() != "e20f21bfac04ed3fc2c1a2edfb3b2b729141efa7667d1dcaafc557676280e52b":
            raise ValueError("Phyre oracle binary digest mismatch")
        self.machine = Uc(UC_ARCH_X86, UC_MODE_32)
        self.machine.mem_map(0x08000000, 0x02000000)
        self.machine.mem_map(0x10000000, 0x400000)
        self.stack_register = UC_X86_REG_ESP
        self.result_register = UC_X86_REG_EAX
        self.instruction_register = UC_X86_REG_EIP
        with binary.open("rb") as source:
            elf = ELFFile(source)
            for segment in elf.iter_segments():
                if segment["p_type"] == "PT_LOAD":
                    self.machine.mem_write(segment["p_vaddr"], segment.data())

    def decode(self, data, headers_start, headers):
        count = struct.unpack_from("<I", data, 24)[0]
        size = struct.unpack_from("<I", data, 20)[0]
        object_start = headers_start + len(headers) * 36
        object_end = object_start + sum(header[2] for header in headers)
        stream_start = object_end + struct.unpack_from("<I", data, 52)[0] + 12 * struct.unpack_from("<I", data, 48)[0] + struct.unpack_from("<I", data, 28)[0] + struct.unpack_from("<I", data, 36)[0]
        if count > 100000 or size > 1048576 or stream_start + size > len(data):
            raise ValueError("Phyre fixup bounds invalid")
        machine = self.machine
        machine.mem_write(0x10000000, struct.pack("<II", count, 0x10001000))
        machine.mem_write(0x10000100, struct.pack("<I", 0x10200000))
        machine.mem_write(0x10000200, struct.pack("<II", len(headers), 0x10000300))
        machine.mem_write(0x10000300, data[headers_start:headers_start + len(headers) * 36])
        machine.mem_write(0x10200000, data[stream_start:stream_start + size])
        machine.mem_write(0x103FF000, struct.pack("<5I", 0x103FE000, 0x10000000, 0x10000100, 0x10000200, 0))
        machine.reg_write(self.stack_register, 0x103FF000)
        machine.emu_start(0x08948446, 0x103FE000, count=10000000)
        consumed = struct.unpack("<I", machine.mem_read(0x10000100, 4))[0] - 0x10200000
        if machine.reg_read(self.result_register) != 0 or machine.reg_read(self.instruction_register) != 0x103FE000 or consumed != size:
            raise ValueError("Phyre oracle fixup mismatch")
        result = bytes(machine.mem_read(0x10001000, count * 16))
        return [struct.unpack_from("<4I", result, index * 16) for index in range(count)]


def validate_output(path):
    resolved = Path(path).resolve()
    allowed = ROOT / ".local/hotlinemiami"
    if not resolved.is_relative_to(allowed.resolve()) or resolved == allowed:
        raise ValueError("output must be inside .local/hotlinemiami")
    return resolved


def read_wad(data):
    if len(data) < 8:
        raise ValueError("truncated WAD header")
    payload_start, count = struct.unpack_from("<II", data)
    if payload_start < 8 or payload_start > len(data) or count > 10000:
        raise ValueError("invalid WAD header")
    cursor = 8
    result = {}
    for _ in range(count):
        if cursor + 4 > payload_start:
            raise ValueError("truncated WAD index")
        length = struct.unpack_from("<I", data, cursor)[0]
        cursor += 4
        if length < 1 or length > 4096 or cursor + length + 8 > payload_start:
            raise ValueError("invalid WAD entry")
        name = data[cursor:cursor + length].decode("utf-8")
        cursor += length
        path = PurePosixPath(name)
        if path.is_absolute() or ".." in path.parts or "\\" in name or "\0" in name or name in result:
            raise ValueError("unsafe WAD entry")
        size, offset = struct.unpack_from("<II", data, cursor)
        cursor += 8
        start = payload_start + offset
        if start + size > len(data):
            raise ValueError("WAD entry exceeds source")
        result[name] = data[start:start + size]
    if cursor != payload_start:
        raise ValueError("WAD index size mismatch")
    return result


def decode_texture(data):
    global FIXUP_MACHINE
    if len(data) < 84 or data[:4] != b"RYHP":
        raise ValueError("invalid Phyre header")
    format_at = data.find(b"RGBA8\0")
    if format_at < 40:
        raise ValueError("unsupported Phyre texture format")
    width, height = struct.unpack_from("<II", data, format_at - 40)
    byte_count = struct.unpack_from("<I", data, 80)[0]
    if width < 1 or height < 1 or width > 8192 or height > 8192 or width * height * 4 != byte_count or byte_count > len(data) - format_at - 6:
        raise ValueError("invalid Phyre texture bounds")
    frames = []
    headers_start = struct.unpack_from("<I", data, 4)[0] + struct.unpack_from("<I", data, 8)[0]
    header_count = struct.unpack_from("<I", data, 16)[0]
    if header_count > 64 or headers_start + header_count * 36 > len(data) - byte_count:
        raise ValueError("invalid Phyre instance headers")
    headers = [struct.unpack_from("<9I", data, headers_start + index * 36) for index in range(header_count)]
    if FIXUP_MACHINE is None:
        FIXUP_MACHINE = PhyreFixupMachine()
    fixups = FIXUP_MACHINE.decode(data, headers_start, headers)
    records_at = headers_start + header_count * 36
    fixup_offset = 0
    for header in headers:
        group_fixups = fixups[fixup_offset:fixup_offset + header[6]]
        fixup_offset += header[6]
        if header[3] != header[1] * 24:
            records_at += header[2]
            continue
        for field, index, _, string_offset in group_fixups:
            if field != 0x80000004 or index >= header[1]:
                continue
            name_at = records_at + header[3] + string_offset
            name_end = data.index(b"\0", name_at)
            name = data[name_at:name_end].decode("utf-8")
            if not name.endswith(".png"):
                continue
            values = struct.unpack_from("<4f", data, records_at + index * 24 + 8)
            coordinates = []
            for value, extent in zip(values, [width, height, width, height]):
                numerator, denominator = value.as_integer_ratio()
                coordinates.append((numerator * extent + denominator // 2) // denominator)
            x, y, frame_width, frame_height = coordinates
            if min(x, y) < 0 or min(frame_width, frame_height) < 1 or x + frame_width > width or y + frame_height > height:
                raise ValueError("Phyre frame exceeds texture")
            frames.append({"name": name.split("/")[-1].removesuffix(".png"), "x": x, "y": height - y - frame_height, "w": frame_width, "h": frame_height})
        records_at += header[2]
    frames.sort(key=lambda frame: (frame["name"].rsplit("_", 1)[0], frame_number(frame["name"])))
    raw = data[-byte_count:]
    stride = width * 4
    rgba = b"".join(raw[row * stride:(row + 1) * stride] for row in range(height - 1, -1, -1))
    return {"width": width, "height": height, "rgba": rgba, "frames": frames}


def frame_number(name):
    suffix = name.rsplit("_", 1)[-1]
    return int(suffix) if suffix.isdigit() else 0


def sprite_identity(name):
    return name.rsplit("_", 1)[0] if name.rsplit("_", 1)[-1].isdigit() else name


def add_sprite_metadata(manifest):
    data = (DEFAULT_WAD.parent.parent / "metadata/hotline_sprites.bin").read_bytes()
    lookup_count = struct.unpack_from("<I", data)[0]
    offset = 4 + lookup_count * 4
    count = struct.unpack_from("<I", data, offset)[0]
    offset += 4
    if offset + count * 148 != len(data):
        raise ValueError("native sprite records mismatch")
    for index in range(count):
        start = offset + index * 148
        fields = struct.unpack_from("<14i", data, start)
        name = data[start + 56:start + 148].split(b"\0")[0].decode("utf-8").split("/")[-1]
        sprite = manifest["sprites"].get(name)
        if not sprite:
            continue
        sprite.update({"width": fields[1], "height": fields[2], "originX": fields[3], "originY": fields[4], "sourceId": fields[0]})
        for frame in sprite["frames"]:
            frame["originX"] = fields[3]
            frame["originY"] = fields[4]


def png_chunk(kind, data):
    return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", binascii.crc32(kind + data) & 0xFFFFFFFF)


def encode_png(width, height, rgba):
    if width < 1 or height < 1 or len(rgba) != width * height * 4:
        raise ValueError("invalid PNG dimensions")
    stride = width * 4
    filtered = b"".join(b"\0" + rgba[y * stride:(y + 1) * stride] for y in range(height))
    return b"\x89PNG\r\n\x1a\n" + png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)) + png_chunk(b"IDAT", zlib.compress(filtered, 9)) + png_chunk(b"IEND", b"")


def decode_generated_png(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError("invalid PNG signature")
    cursor = 8
    compressed = b""
    width = height = 0
    while cursor < len(data):
        size = struct.unpack_from(">I", data, cursor)[0]
        kind = data[cursor + 4:cursor + 8]
        payload = data[cursor + 8:cursor + 8 + size]
        checksum = struct.unpack_from(">I", data, cursor + 8 + size)[0]
        if checksum != binascii.crc32(kind + payload) & 0xFFFFFFFF:
            raise ValueError("PNG CRC mismatch")
        if kind == b"IHDR":
            width, height = struct.unpack_from(">II", payload)
        if kind == b"IDAT":
            compressed += payload
        cursor += size + 12
    filtered = zlib.decompress(compressed)
    stride = width * 4
    if len(filtered) != (stride + 1) * height:
        raise ValueError("invalid PNG pixel size")
    rows = []
    for y in range(height):
        offset = y * (stride + 1)
        if filtered[offset] != 0:
            raise ValueError("unexpected PNG filter")
        rows.append(filtered[offset + 1:offset + stride + 1])
    return width, height, b"".join(rows)


def crop_rgba(texture, x, y, width, height):
    if min(x, y) < 0 or min(width, height) < 1 or x + width > texture["width"] or y + height > texture["height"]:
        raise ValueError("crop exceeds texture")
    stride = texture["width"] * 4
    return b"".join(texture["rgba"][(y + row) * stride + x * 4:(y + row) * stride + (x + width) * 4] for row in range(height))


def write_texture(output, key, source, texture):
    path = output / "images" / (key + ".png")
    path.parent.mkdir(parents=True, exist_ok=True)
    data = encode_png(texture["width"], texture["height"], texture["rgba"])
    if decode_generated_png(data)[2] != texture["rgba"]:
        raise ValueError("PNG roundtrip mismatch")
    path.write_bytes(data)
    return {"file": str(path.relative_to(output)), "width": texture["width"], "height": texture["height"], "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(), "source": source}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--wad", type=Path, default=DEFAULT_WAD)
    parser.add_argument("--output", type=Path, default=ROOT / ".local/hotlinemiami/assets")
    args = parser.parse_args()
    output = validate_output(args.output)
    output.mkdir(parents=True, exist_ok=True)
    data = args.wad.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if digest != EXPECTED_SHA256:
        raise ValueError("original WAD digest mismatch")
    entries = read_wad(data)
    manifest = {"version": 1, "sourceSHA256": digest, "images": {}, "sprites": {}, "characters": [], "weapons": [], "maps": []}
    failures = []
    for source, payload in entries.items():
        if not source.startswith("GL/AssetScripts/") or not source.endswith(".phyre"):
            continue
        key = Path(source).name.removesuffix(".ags.phyre")
        try:
            texture = decode_texture(payload)
        except ValueError as error:
            failures.append({"source": source, "error": str(error)})
            continue
        manifest["images"][key] = write_texture(output, key, source, texture)
        for frame in texture["frames"]:
            name = sprite_identity(frame["name"])
            sprite = manifest["sprites"].setdefault(name, {"image": key, "frames": []})
            sprite["frames"].append({k: v for k, v in frame.items() if k != "name"})
    add_sprite_metadata(manifest)
    (output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")) + "\n")
    report = {"textures": len(manifest["images"]), "sprites": len(manifest["sprites"]), "failures": failures, "sourceSHA256": digest}
    report_path = output.parent / "research/texture-verification.json"
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(report, ensure_ascii=False))


if __name__ == "__main__":
    main()
