import assert from "node:assert/strict";
import QRCode from "qrcode";
import pngjs from "pngjs";
import {
  BinaryBitmap,
  HybridBinarizer,
  RGBLuminanceSource,
  QRCodeReader,
} from "@zxing/library";
const input = "CAMPUS:0123-4567-89AB-CDEF-GHJK-MNPQ-RSTV-WXYZ";
const bytes = await QRCode.toBuffer(input, {
  width: 600,
  margin: 3,
  errorCorrectionLevel: "M",
});
const { width, height, data } = pngjs.PNG.sync.read(bytes);
const pixels = new Int32Array(width * height);
for (let i = 0; i < pixels.length; i++)
  pixels[i] = (data[i * 4] << 16) | (data[i * 4 + 1] << 8) | data[i * 4 + 2];
const source = new RGBLuminanceSource(pixels, width, height);
const result = new QRCodeReader().decode(
  new BinaryBitmap(new HybridBinarizer(source)),
);
assert.equal(result.getText(), input);
console.log("QR generation and decoding round-trip passed.");
