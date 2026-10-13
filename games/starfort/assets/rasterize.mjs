// resvg handles SVG gradients consistently with the browser; MuPDF does not.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const dependency = process.env.STARFORT_RESVG_MODULE ?? resolve(".local/starfort/rasterizer/node_modules/@resvg/resvg-js/index.js");
const { Resvg } = await import(pathToFileURL(dependency).href);
const source = readFileSync(process.argv[2]);
const rendered = new Resvg(source, {
  fitTo: { mode: "zoom", value: 2 },
  font: { loadSystemFonts: false },
}).render();
process.stdout.write(rendered.asPng());
