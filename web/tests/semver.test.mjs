import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/utils/semver.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.ESNext },
});
const { compareVersions, highestVersion, isValidVersion, suggestVersions } =
  await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);

test("SemVer precedence includes numeric prereleases and ignores build metadata", () => {
  const order = ["1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"];
  for (let i = 1; i < order.length; i++) {
    assert.equal(compareVersions(order[i - 1], order[i]), -1);
    assert.equal(compareVersions(order[i], order[i - 1]), 1);
  }
  assert.equal(compareVersions("1.2.3+build.1", "1.2.3+build.2"), 0);
  assert.equal(compareVersions("1.2", "1.2.0"), 0);
  assert.equal(compareVersions("1", "1.0.0"), 0);
});

test("suggestions use the highest semantic version rather than upload order", () => {
  const highest = highestVersion(["1.9.0", "1.2.0", "1.10.0", "1.10.0-beta.11"]);
  assert.equal(highest, "1.10.0");
  assert.deepEqual(suggestVersions(highest), { patch: "1.10.1", minor: "1.11.0", major: "2.0.0" });
  assert.equal(highestVersion([]), "");
  assert.equal(suggestVersions("").patch, "0.1.0");
});

test("prereleases recommend their corresponding stable version", () => {
  assert.equal(suggestVersions("1.2.4-beta.2").patch, "1.2.4");
  assert.equal(suggestVersions("1.2.4-beta.2+ci.9").patch, "1.2.4");
  assert.equal(suggestVersions("1.2.4+ci-beta").patch, "1.2.5");
  assert.equal(suggestVersions("1.2").patch, "1.2.1");
});

test("strict version input rejects ambiguous or malformed versions", () => {
  for (const version of ["1.2.3", "0.1.0", "1.3.0-beta.1", "2.0.0+build.001", "1.0.0-0", "1.0.0-alpha-1"]) assert.equal(isValidVersion(version), true, version);
  for (const version of ["", "1", "1.2", "v1.2.3", "01.2.3", "1.02.3", "1.2.03", "1.2.3-01", "1.2.3-beta..1", "1.2.3+", "1.2.3\n", "-1.2.3"]) assert.equal(isValidVersion(version), false, version);
});

test("large numeric components do not lose precision", () => {
  assert.equal(compareVersions("9007199254740993.0.0", "9007199254740992.0.0"), 1);
  assert.equal(suggestVersions("1.0.9007199254740993").patch, "1.0.9007199254740994");
});
