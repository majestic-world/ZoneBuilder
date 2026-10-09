import { Resvg } from "@resvg/resvg-js";
import { readFileSync, writeFileSync } from "node:fs";

const svg = readFileSync(process.argv[2] ?? "zonebuilder.svg");
const png = (size) => new Resvg(svg, { fitTo: { mode: "width", value: size } }).render().asPng();

if (process.argv[3] === "preview") {
  for (const s of [512, 48, 32, 16]) writeFileSync(`preview-${s}.png`, png(s));
  process.exit(0);
}

// ICO with PNG-compressed entries (Windows Vista+).
const sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256];
const images = sizes.map(png);
const header = Buffer.alloc(6);
header.writeUInt16LE(0, 0);
header.writeUInt16LE(1, 2);
header.writeUInt16LE(sizes.length, 4);
const dir = Buffer.alloc(16 * sizes.length);
let offset = 6 + dir.length;
sizes.forEach((s, i) => {
  const o = i * 16;
  dir.writeUInt8(s >= 256 ? 0 : s, o);
  dir.writeUInt8(s >= 256 ? 0 : s, o + 1);
  dir.writeUInt8(0, o + 2);
  dir.writeUInt8(0, o + 3);
  dir.writeUInt16LE(1, o + 4);
  dir.writeUInt16LE(32, o + 6);
  dir.writeUInt32LE(images[i].length, o + 8);
  dir.writeUInt32LE(offset, o + 12);
  offset += images[i].length;
});
writeFileSync(process.argv[3] ?? "zonebuilder.ico", Buffer.concat([header, dir, ...images]));
console.log("ico", offset, "bytes,", sizes.length, "sizes");
