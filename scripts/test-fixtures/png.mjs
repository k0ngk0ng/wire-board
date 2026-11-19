import { crc32, deflateSync } from "node:zlib";

export function createPNG() {
  const chunk = (type, payload) => {
    const name = Buffer.from(type);
    const output = Buffer.alloc(payload.length + 12);
    output.writeUInt32BE(payload.length, 0);
    name.copy(output, 4);
    payload.copy(output, 8);
    output.writeUInt32BE(crc32(Buffer.concat([name, payload])), output.length - 4);
    return output;
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(1, 0);
  header.writeUInt32BE(1, 4);
  header[8] = 8;
  header[9] = 6;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk("IHDR", header),
    chunk("IDAT", deflateSync(Buffer.from([0, 255, 255, 255, 255]))),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}
