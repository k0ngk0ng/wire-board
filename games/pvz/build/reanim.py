"""Decode PC 1.0.0.1051's serialized 32-bit ReanimatorDefinition data."""
import math
import struct
import zlib

FIELDS = ("x", "y", "kx", "ky", "sx", "sy", "f", "a")


def decode(encoded):
    try:
        if len(encoded) < 8 or struct.unpack_from("<I", encoded)[0] != 0xDEADFED4:
            raise ValueError("Unsupported compressed reanimation")
        expected = struct.unpack_from("<I", encoded, 4)[0]
        data = zlib.decompress(encoded[8:])
        if len(data) != expected or len(data) < 24:
            raise ValueError("Wrong reanimation size")
        magic, _, count, fps, _, track_size = struct.unpack_from("<IIIfII", data)
        if magic != 0xB393B4C0 or track_size != 12 or not 0 < count <= 512 or not 0 < fps <= 120:
            raise ValueError("Unsupported reanimation layout")
        at = 24
        counts = []
        for _ in range(count):
            _, _, frames = struct.unpack_from("<III", data, at)
            at += 12
            if frames > 10000:
                raise ValueError("Too many reanimation frames")
            counts.append(frames)

        def string():
            nonlocal at
            length = struct.unpack_from("<I", data, at)[0]
            at += 4
            if length > len(data) - at:
                raise ValueError("Truncated reanimation string")
            value = data[at:at + length].decode("utf8")
            at += length
            return value

        tracks = []
        for frames in counts:
            name = string()
            size = struct.unpack_from("<I", data, at)[0]
            at += 4
            if size != 44:
                raise ValueError("Unsupported transform layout")
            values = []
            previous = dict(zip(FIELDS, (0, 0, 0, 0, 1, 1, 0, 1)))
            previous.update(image="", font="", text="")
            for _ in range(frames):
                floats = struct.unpack_from("<8fIII", data, at)[:8]
                at += 44
                if not all(math.isfinite(value) for value in floats):
                    raise ValueError("Non-finite reanimation transform")
                values.append(dict(zip(FIELDS, floats)))
            out = []
            for value in values:
                strings = {key: string() for key in ("image", "font", "text")}
                current = {key: previous[key] if number == -10000 else number for key, number in value.items()}
                current.update({key: value or previous[key] for key, value in strings.items()})
                out.append(current)
                previous = current
            tracks.append({"name": name, "frames": out})
        if at != len(data):
            raise ValueError("Unexplained reanimation bytes")
        return {"fps": fps, "tracks": tracks}
    except (struct.error, zlib.error, UnicodeDecodeError) as error:
        raise ValueError("Malformed reanimation") from error
