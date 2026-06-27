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
    { id: 1, title: "Books / Guides", collectionType: "directory", primaryType: "book", bookCount: 3 },
  ]);

  assert.deepEqual(options, [
    { value: "nes", label: "NES", count: 10 },
    { value: "md", label: "Mega Drive", count: 12 },
    { value: "neogeo", label: "Neo Geo", count: 7 },
  ]);
});
