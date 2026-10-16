import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/utils/designPreviews.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.ESNext },
});
const { selectDesignPreview: select } =
  await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
const image = (theme, device) => ({ theme, device, url: `${theme}/${device}` });

test("default and legacy previews take priority; a dark-only mobile design remains usable", () => {
  assert.equal(select([]), undefined);
  assert.equal(select([image("dark", "mobile")]).url, "dark/mobile");
  assert.equal(select([image("dark", "desktop"), image("light", "desktop")]).url, "light/desktop");
  assert.equal(select([image("light", "desktop"), image("default", "mobile")]).url, "default/mobile");
});

test("switching themes keeps the device if available and otherwise picks a matching image", () => {
  const previews = [image("default", "desktop"), image("light", "desktop"), image("dark", "mobile")];
  assert.equal(select(previews, "dark", "desktop").url, "dark/mobile");
  assert.equal(select(previews, "light", "mobile").url, "light/desktop");
  assert.equal(select([...previews, image("dark", "desktop")], "dark", "desktop").url, "dark/desktop");
});

test("historical versions preserve available selections and fall back when groups disappear", () => {
  const selected = select([image("dark", "desktop")], "dark", "desktop");
  const legacy = select([image("default", "mobile")], selected.theme, selected.device);
  assert.equal(legacy.url, "default/mobile");
  assert.equal(select([image("dark", "mobile"), image("light", "mobile")], legacy.theme, legacy.device).url, "light/mobile");
});
