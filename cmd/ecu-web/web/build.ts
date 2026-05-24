/**
 * Builds the SPA into dist/ for embedding into the ecu-web Go binary.
 *
 *   dist/index.html        (copied from src/index.html)
 *   dist/assets/main.js    (minified bundle of src/main.ts + Lit + uPlot)
 *
 * The cooldown gate runs first via package.json's "prebuild" script.
 * Run: `bun run build` (or `bun run build.ts`).
 */
import { rm, mkdir, cp } from "node:fs/promises";

const ROOT = new URL(".", import.meta.url).pathname;
const DIST = `${ROOT}dist`;

await rm(DIST, { recursive: true, force: true });
await mkdir(`${DIST}/assets`, { recursive: true });

const result = await Bun.build({
  entrypoints: [`${ROOT}src/main.ts`],
  outdir: `${DIST}/assets`,
  naming: "[name].[ext]",
  minify: true,
  target: "browser",
  sourcemap: "none",
});

if (!result.success) {
  console.error("✗ build failed:");
  for (const log of result.logs) console.error(log);
  process.exit(1);
}

await cp(`${ROOT}src/index.html`, `${DIST}/index.html`);

let total = 0;
for (const out of result.outputs) total += out.size;
const kb = (total / 1024).toFixed(1);
console.log(`✓ built dist/ — bundle ${kb} KB (${result.outputs.length} asset(s))`);
