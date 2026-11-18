import assert from "node:assert/strict";
import { crc32, deflateSync, inflateSync } from "node:zlib";

const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

export function decodePNG(bytes) {
  assert.ok(Buffer.isBuffer(bytes) && bytes.length >= 45 && bytes.length <= 32 * 1024 * 1024, "Invalid native PNG size");
  assert.ok(bytes.subarray(0, 8).equals(signature), "Invalid native PNG signature");
  let offset = 8, width, height, channels, ended = false, dataEnded = false;
  const compressed = [];
  while (offset < bytes.length) {
    assert.ok(offset + 12 <= bytes.length, "Truncated native PNG chunk");
    const length = bytes.readUInt32BE(offset), start = offset + 8, end = start + length;
    assert.ok(end + 4 <= bytes.length, "Truncated native PNG payload");
    const kind = bytes.toString("ascii", offset + 4, start);
    assert.equal(crc32(bytes.subarray(offset + 4, end)), bytes.readUInt32BE(end), "Native PNG checksum mismatch");
    assert.ok(width !== undefined || kind === "IHDR", "Native PNG has no initial header");
    if (kind === "IHDR") {
      assert.ok(offset === 8 && length === 13 && width === undefined, "Invalid native PNG header");
      width = bytes.readUInt32BE(start);
      height = bytes.readUInt32BE(start + 4);
      assert.ok(width > 0 && height > 0 && width * height <= 16777216, "Unsupported native PNG dimensions");
      assert.ok(bytes[start + 8] === 8 && [2, 6].includes(bytes[start + 9]), "Native PNG must contain 8-bit RGB or RGBA pixels");
      assert.ok(bytes[start + 10] === 0 && bytes[start + 11] === 0 && bytes[start + 12] === 0, "Unsupported native PNG compression or interlace");
      channels = bytes[start + 9] === 6 ? 4 : 3;
    } else if (kind === "IDAT") {
      assert.equal(dataEnded, false, "Native PNG data chunks are not consecutive");
      compressed.push(bytes.subarray(start, end));
    } else {
      if (compressed.length) dataEnded = true;
      if (kind === "IEND") {
        assert.equal(length, 0);
        assert.ok(compressed.length > 0, "Native PNG has no image data");
        ended = true;
        offset = end + 4;
        break;
      }
      assert.ok(kind === "PLTE" || kind[0] >= "a" && kind[0] <= "z", "Unsupported native PNG critical chunk");
    }
    offset = end + 4;
  }
  assert.ok(ended && offset === bytes.length, "Native PNG has no complete ending");
  const stride = width * channels, expected = (stride + 1) * height;
  const encoded = inflateSync(Buffer.concat(compressed), { maxOutputLength: expected });
  assert.equal(encoded.length, expected, "Native PNG scanline size mismatch");
  const pixels = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y++) {
    const source = y * (stride + 1), row = y * stride, filter = encoded[source];
    assert.ok(filter <= 4, "Unsupported native PNG row filter");
    for (let x = 0; x < stride; x++) {
      const left = x >= channels ? pixels[row + x - channels] : 0;
      const up = y ? pixels[row + x - stride] : 0;
      const diagonal = y && x >= channels ? pixels[row + x - stride - channels] : 0;
      let predictor = 0;
      if (filter === 1) predictor = left;
      else if (filter === 2) predictor = up;
      else if (filter === 3) predictor = Math.floor((left + up) / 2);
      else if (filter === 4) {
        const value = left + up - diagonal, a = Math.abs(value - left), b = Math.abs(value - up), c = Math.abs(value - diagonal);
        predictor = a <= b && a <= c ? left : b <= c ? up : diagonal;
      }
      pixels[row + x] = encoded[source + x + 1] + predictor;
    }
  }
  if (channels === 4) return { width, height, rgba: pixels };
  const rgba = Buffer.alloc(width * height * 4);
  for (let source = 0, target = 0; source < pixels.length; source += 3, target += 4) {
    pixels.copy(rgba, target, source, source + 3);
    rgba[target + 3] = 255;
  }
  return { width, height, rgba };
}

function chunk(kind, data) {
  const result = Buffer.alloc(data.length + 12);
  result.writeUInt32BE(data.length, 0);
  result.write(kind, 4, "ascii");
  data.copy(result, 8);
  result.writeUInt32BE(crc32(result.subarray(4, result.length - 4)), result.length - 4);
  return result;
}

export function cropPNG(image, bounds) {
  const { left, top, width, height } = bounds;
  assert.ok([left, top, width, height].every(Number.isInteger) && left >= 0 && top >= 0 && width > 0 && height > 0 && left + width <= image.width && top + height <= image.height, "Invalid native PNG crop rectangle");
  assert.equal(image.rgba.length, image.width * image.height * 4);
  const rgba = Buffer.alloc(width * height * 4), rows = Buffer.alloc((width * 4 + 1) * height);
  for (let y = 0; y < height; y++) {
    const start = ((top + y) * image.width + left) * 4;
    image.rgba.copy(rgba, y * width * 4, start, start + width * 4);
    image.rgba.copy(rows, y * (width * 4 + 1) + 1, start, start + width * 4);
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8;
  header[9] = 6;
  const png = Buffer.concat([signature, chunk("IHDR", header), chunk("IDAT", deflateSync(rows, { level: 1 })), chunk("IEND", Buffer.alloc(0))]);
  return { rgba, png };
}
