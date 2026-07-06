// One-off generator for placeholder PWA icons.
//
// Produces solid-colour PNGs (theme colour with a lighter centred square) at
// 192x192 and 512x512 without any image library, by hand-assembling a valid
// PNG (IHDR + IDAT + IEND) with zlib-deflated raw RGBA scanlines.
//
// Run: node scripts/gen-icons.mjs
import { deflateSync } from 'node:zlib';
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const publicDir = resolve(here, '..', 'public');

// slate-900 background, slate-100 foreground square (matches theme_color).
const BG = [15, 23, 42, 255]; // #0f172a
const FG = [241, 245, 249, 255]; // #f1f5f9

function crc32(buf) {
  let c = ~0;
  for (let i = 0; i < buf.length; i++) {
    c ^= buf[i];
    for (let k = 0; k < 8; k++) c = c & 1 ? (c >>> 1) ^ 0xedb88320 : c >>> 1;
  }
  return (~c) >>> 0;
}

function chunk(type, data) {
  const typeBuf = Buffer.from(type, 'ascii');
  const lenBuf = Buffer.alloc(4);
  lenBuf.writeUInt32BE(data.length, 0);
  const crcBuf = Buffer.alloc(4);
  crcBuf.writeUInt32BE(crc32(Buffer.concat([typeBuf, data])), 0);
  return Buffer.concat([lenBuf, typeBuf, data, crcBuf]);
}

function makePng(size) {
  const raw = Buffer.alloc(size * (size * 4 + 1));
  // Inset foreground square (centred, ~55% of the canvas) as a simple glyph.
  const inset = Math.floor(size * 0.225);
  for (let y = 0; y < size; y++) {
    const rowStart = y * (size * 4 + 1);
    raw[rowStart] = 0; // filter type: none
    for (let x = 0; x < size; x++) {
      const inSquare = x >= inset && x < size - inset && y >= inset && y < size - inset;
      const c = inSquare ? FG : BG;
      const o = rowStart + 1 + x * 4;
      raw[o] = c[0];
      raw[o + 1] = c[1];
      raw[o + 2] = c[2];
      raw[o + 3] = c[3];
    }
  }

  const sig = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0);
  ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 6; // colour type: RGBA
  ihdr[10] = 0; // compression
  ihdr[11] = 0; // filter
  ihdr[12] = 0; // interlace
  const idat = deflateSync(raw);
  return Buffer.concat([
    sig,
    chunk('IHDR', ihdr),
    chunk('IDAT', idat),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

writeFileSync(resolve(publicDir, 'icon-192.png'), makePng(192));
writeFileSync(resolve(publicDir, 'icon-512.png'), makePng(512));
console.log('Wrote icon-192.png and icon-512.png to', publicDir);
