import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

const srcDir = path.dirname(fileURLToPath(import.meta.url));

async function loadWebtoonPositionModule() {
  const source = await readFile(path.join(srcDir, "webtoon-position.ts"), "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2020,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString("base64")}`);
}

test("webtoon anchor position is based on page key and normalized page offset", async () => {
  const { buildWebtoonPosition } = await loadWebtoonPositionModule();
  const position = buildWebtoonPosition({
    pages: [
      { index: 0, pageKey: "archive:0000.webp", displayedTop: 0, displayedHeight: 1000, logicalHeight: 2 },
      { index: 1, pageKey: "archive:0001.webp", displayedTop: 1000, displayedHeight: 2000, logicalHeight: 4 },
      { index: 2, pageKey: "archive:0002.webp", displayedTop: 3000, displayedHeight: 1000, logicalHeight: 2 },
    ],
    scrollTop: 1500,
    viewportHeight: 1000,
    viewportAnchorRatio: 0.28,
  });

  assert.equal(position.schema, "webtoon-position-v1");
  assert.equal(position.pageIndex, 1);
  assert.equal(position.pageKey, "archive:0001.webp");
  assert.equal(position.viewportAnchorRatio, 0.28);
  assert.equal(position.pageCount, 3);
  assert.equal(Number(position.pageYOffsetRatio.toFixed(2)), 0.39);
  assert.equal(Number(position.documentProgress.toFixed(3)), 0.445);
});

test("webtoon restore prefers page key and computes scrollTop from the viewport anchor", async () => {
  const { resolveWebtoonRestoreTarget } = await loadWebtoonPositionModule();
  const target = resolveWebtoonRestoreTarget({
    position: {
      schema: "webtoon-position-v1",
      pageIndex: 20,
      pageKey: "archive:0001-renamed.webp",
      pageYOffsetRatio: 0.4,
      viewportAnchorRatio: 0.28,
      documentProgress: 0.5,
      pageCount: 3,
    },
    pages: [
      { index: 0, pageKey: "archive:0000.webp", displayedTop: 0, displayedHeight: 1000, logicalHeight: 2 },
      { index: 1, pageKey: "archive:0001-renamed.webp", displayedTop: 1000, displayedHeight: 2000, logicalHeight: 4 },
      { index: 2, pageKey: "archive:0002.webp", displayedTop: 3000, displayedHeight: 1000, logicalHeight: 2 },
    ],
    viewportHeight: 1000,
  });

  assert.deepEqual(target, { pageIndex: 1, scrollTop: 1520 });
});

test("webtoon restore falls back to document progress when page key and index are unavailable", async () => {
  const { resolveWebtoonRestoreTarget } = await loadWebtoonPositionModule();
  const target = resolveWebtoonRestoreTarget({
    position: {
      schema: "webtoon-position-v1",
      pageIndex: 99,
      pageKey: "archive:missing.webp",
      pageYOffsetRatio: 0.2,
      viewportAnchorRatio: 0.28,
      documentProgress: 0.75,
      pageCount: 99,
    },
    pages: [
      { index: 0, pageKey: "archive:0000.webp", displayedTop: 0, displayedHeight: 1000, logicalHeight: 2 },
      { index: 1, pageKey: "archive:0001.webp", displayedTop: 1000, displayedHeight: 2000, logicalHeight: 4 },
      { index: 2, pageKey: "archive:0002.webp", displayedTop: 3000, displayedHeight: 1000, logicalHeight: 2 },
    ],
    viewportHeight: 1000,
  });

  assert.deepEqual(target, { pageIndex: 2, scrollTop: 2720 });
});

test("webtoon document progress preserves the previous value when full logical heights are unavailable", async () => {
  const { buildWebtoonPosition, stabilizeWebtoonDocumentProgress } = await loadWebtoonPositionModule();
  const pages = [
    { index: 0, pageKey: "archive:0000.webp", displayedTop: 0, displayedHeight: 2200 },
    { index: 1, pageKey: "archive:0001.webp", displayedTop: 2200, displayedHeight: 2200 },
    { index: 2, pageKey: "archive:0002.webp", displayedTop: 4400, displayedHeight: 2200, logicalHeight: 10 },
  ];
  const position = buildWebtoonPosition({
    pages,
    scrollTop: 5280,
    viewportHeight: 1000,
    viewportAnchorRatio: 0.28,
  });

  const stabilized = stabilizeWebtoonDocumentProgress(
    position,
    {
      schema: "webtoon-position-v1",
      pageIndex: 2,
      pageKey: "archive:0002.webp",
      pageYOffsetRatio: 0.4,
      viewportAnchorRatio: 0.28,
      documentProgress: 0.5,
      pageCount: 3,
    },
    pages,
  );

  assert.equal(Number(position.documentProgress.toFixed(3)), 0.999);
  assert.equal(Number(stabilized.documentProgress.toFixed(3)), 0.542);
});
