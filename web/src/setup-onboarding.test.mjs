import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

const srcDir = path.dirname(fileURLToPath(import.meta.url));

async function loadSetupOnboardingModule() {
  const source = await readFile(path.join(srcDir, "setup-onboarding.ts"), "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2020,
      target: ts.ScriptTarget.ES2020,
    },
  }).outputText;
  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString("base64")}`);
}

test("SpatialEMU onboarding keeps product and connection routes distinct", async () => {
  const onboarding = await loadSetupOnboardingModule();

  assert.deepEqual(onboarding.setupOnboardingLinks("en"), {
    learnMore: "https://spatialemu.com/foliospace/",
    setupGuide: "https://spatialemu.com/guides/foliospace-connection/",
  });
  assert.notEqual(onboarding.SPATIALEMU_FOLIOSPACE_URL, onboarding.SPATIALEMU_CONNECTION_GUIDE_URL);
});

test("simplified and traditional Chinese UI use the verified zh-cn routes", async () => {
  const onboarding = await loadSetupOnboardingModule();
  const expected = {
    learnMore: "https://spatialemu.com/zh-cn/foliospace/",
    setupGuide: "https://spatialemu.com/zh-cn/guides/foliospace-connection/",
  };

  assert.deepEqual(onboarding.setupOnboardingLinks("zh"), expected);
  assert.deepEqual(onboarding.setupOnboardingLinks("zht"), expected);
});

test("onboarding copy states the optional local-file path and Docker requirements", async () => {
  const onboarding = await loadSetupOnboardingModule();
  const english = onboarding.setupOnboardingCopy("en");
  const chinese = onboarding.setupOnboardingCopy("zh");

  assert.match(english.optionalBody, /open local files directly/i);
  assert.match(english.hostRequirement, /NAS and coding are not required/i);
  assert.match(english.configurationRequirement, /Docker.*service URL.*access token/i);
  assert.match(chinese.optionalBody, /直接打开本地文件/);
  assert.match(chinese.hostRequirement, /不需要 NAS.*不需要写代码/);
});
