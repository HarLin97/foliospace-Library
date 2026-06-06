import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

const srcDir = path.dirname(fileURLToPath(import.meta.url));

async function loadReaderFitModule() {
  const source = await readFile(path.join(srcDir, "reader-fit.ts"), "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2020,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString("base64")}`);
}

test("fullscreen image fit expands high resolution art without exceeding source-aware clarity", async () => {
  const { fullscreenImageFit } = await loadReaderFitModule();

  const fit = fullscreenImageFit({
    naturalWidth: 2400,
    naturalHeight: 3600,
    devicePixelRatio: 2,
    mode: "single",
  });

  assert.equal(fit.maxCssWidth, 1620);
  assert.equal(fit.maxCssHeight, 2430);
});

test("fullscreen image fit keeps lower resolution art from being stretched across wide screens", async () => {
  const { fullscreenImageFit } = await loadReaderFitModule();

  const fit = fullscreenImageFit({
    naturalWidth: 800,
    naturalHeight: 1200,
    devicePixelRatio: 2,
    mode: "webtoon",
  });

  assert.equal(fit.maxCssWidth, 540);
  assert.equal(fit.maxCssHeight, 810);
});

test("fullscreen reader css uses source-aware fit variables for paged and webtoon images", async () => {
  const styleSource = await readFile(path.join(srcDir, "styles.css"), "utf8");
  const appSource = await readFile(path.join(srcDir, "App.tsx"), "utf8");

  assert.match(
    styleSource,
    /\.reader\.immersiveMode\s+\.pageSpread\s+img\s*\{[\s\S]*var\(--reader-fit-width/s,
    "fullscreen paged images should use a source-aware max width variable",
  );
  assert.match(
    styleSource,
    /\.reader\.immersiveMode\s+\.webtoonPage\s+img\s*\{[\s\S]*var\(--reader-fit-width/s,
    "fullscreen webtoon images should use a source-aware width variable",
  );
  assert.ok(appSource.includes("fullscreenImageFit"), "reader should calculate image fit from natural dimensions");
  assert.ok(appSource.includes("--reader-fit-width"), "reader should pass fit width through CSS variables");
});

test("fullscreen webtoon reader uses a subtle scrollbar", async () => {
  const styleSource = await readFile(path.join(srcDir, "styles.css"), "utf8");

  assert.match(
    styleSource,
    /\.reader\.immersiveMode\s+\.webtoonReader\s*\{[\s\S]*scrollbar-width:\s*thin/s,
    "fullscreen webtoon should use a thin native scrollbar",
  );
  assert.match(
    styleSource,
    /\.reader\.immersiveMode\s+\.webtoonReader::\-webkit\-scrollbar-thumb\s*\{[\s\S]*background-clip:\s*padding-box/s,
    "fullscreen webtoon should render a padded translucent scrollbar thumb",
  );
});
