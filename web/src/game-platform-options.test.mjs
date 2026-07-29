import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

const srcDir = path.dirname(fileURLToPath(import.meta.url));

async function loadGamePlatformOptionsModule() {
  const source = await readFile(path.join(srcDir, "game-platform-options.ts"), "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2020,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString("base64")}`);
}

test("game platform filter options come from game platform collections", async () => {
  const { gamePlatformFilterOptions } = await loadGamePlatformOptionsModule();

  const options = gamePlatformFilterOptions([
    { id: -1010, title: "Games / NES", collectionType: "game_platform", primaryType: "game", bookCount: 10 },
    { id: -1060, title: "Games / Mega Drive", collectionType: "game_platform", primaryType: "game", bookCount: 12 },
    { id: -1080, title: "Games / Neo Geo", collectionType: "game_platform", primaryType: "game", bookCount: 7 },
    { id: -1081, title: "Games / CPS-1", collectionType: "game_platform", primaryType: "game", bookCount: 14 },
    { id: -1082, title: "Games / CPS-2", collectionType: "game_platform", primaryType: "game", bookCount: 15 },
    { id: -1083, title: "Games / CPS-3", collectionType: "game_platform", primaryType: "game", bookCount: 16 },
    { id: -1077, title: "Games / NEC PC-98", collectionType: "game_platform", primaryType: "game", bookCount: 42 },
    { id: -1086, title: "Games / NAOMI 2", collectionType: "game_platform", primaryType: "game", bookCount: 43 },
    { id: -1087, title: "Games / PSP", collectionType: "game_platform", primaryType: "game", bookCount: 8 },
    { id: -1088, title: "Games / Nintendo GameCube", collectionType: "game_platform", primaryType: "game", bookCount: 9 },
    { id: -1089, title: "Games / PlayStation 2", collectionType: "game_platform", primaryType: "game", bookCount: 10 },
    { id: 1, title: "Books / Guides", collectionType: "directory", primaryType: "book", bookCount: 3 },
  ]);

  assert.deepEqual(options, [
    { value: "nes", label: "NES", count: 10 },
    { value: "md", label: "Mega Drive", count: 12 },
    { value: "neogeo", label: "Neo Geo", count: 7 },
    { value: "cps1", label: "CPS-1", count: 14 },
    { value: "cps2", label: "CPS-2", count: 15 },
    { value: "cps3", label: "CPS-3", count: 16 },
    { value: "pc98", label: "NEC PC-98", count: 42 },
    { value: "naomi2", label: "NAOMI 2", count: 43 },
    { value: "psp", label: "PSP", count: 8 },
    { value: "ngc", label: "Nintendo GameCube", count: 9 },
    { value: "ps2", label: "PlayStation 2", count: 10 },
  ]);
});

test("game platform filter options prefer canonical client facets", async () => {
  const { gamePlatformFilterOptionsFromFacets } = await loadGamePlatformOptionsModule();

  assert.deepEqual(gamePlatformFilterOptionsFromFacets([
    { platform: "cps1", title: "CPS-1", count: 414 },
    { platform: "cps2", title: "CPS-2", count: 390 },
  ]), [
    { value: "cps1", label: "CPS-1", count: 414 },
    { value: "cps2", label: "CPS-2", count: 390 },
  ]);
});
