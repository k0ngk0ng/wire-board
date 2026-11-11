export type HSV8 = readonly number[];
export type ClassicPalette = {
  primary: HSV8;
  secondary: HSV8;
  interior: HSV8;
  driver: HSV8;
};

export function hsv8ToRGB(value: HSV8): [number, number, number] {
  const h = ((value[0] ?? 0) / 255) * 6;
  const s = Math.max(0, Math.min(1, (value[1] ?? 0) / 255));
  const v = Math.max(0, Math.min(1, (value[2] ?? 0) / 255));
  const f = h - Math.floor(h), p = v * (1 - s), q = v * (1 - s * f), t = v * (1 - s * (1 - f));
  const rgb = [[v,t,p],[q,v,p],[p,v,t],[p,q,v],[t,p,v],[v,p,q]][Math.floor(h) % 6];
  return rgb.map((n) => Math.round(n * 255)) as [number, number, number];
}

// These byte ranges are native texture data labels. Classify original pixels
// before filtering; interpolating alpha first would recolour texture borders.
export function recolorCarPixels(
  source: Uint8ClampedArray,
  primary: readonly number[],
  palette: ClassicPalette,
): Uint8ClampedArray<ArrayBuffer> {
  const output = new Uint8ClampedArray(source);
  const secondary = hsv8ToRGB(palette.secondary);
  const interior = hsv8ToRGB(palette.interior);
  const driver = hsv8ToRGB(palette.driver);
  for (let at = 0; at < output.length; at += 4) {
    const alpha = source[at + 3];
    const tint = alpha >= 223 && alpha <= 230 ? primary
      : alpha >= 160 && alpha <= 182 ? interior
      : alpha >= 96 && alpha <= 118 ? secondary
      : alpha >= 32 && alpha <= 46 ? driver : undefined;
    if (!tint) continue;
    for (let channel = 0; channel < 3; channel++)
      output[at + channel] = Math.round((source[at + channel] * tint[channel]) / 255);
    output[at + 3] = 255;
  }
  return output;
}
