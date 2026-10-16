interface Semver {
  core: [bigint, bigint, bigint];
  prerelease: string[];
}

function parse(version: string, allowShort = false): Semver | null {
  // Older API clients can publish Go semver's abbreviated 1 / 1.2 forms.
  if (allowShort && /^(0|[1-9]\d*)(\.(0|[1-9]\d*))?$/.test(version)) {
    version += version.includes(".") ? ".0" : ".0.0";
  }
  const match = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.exec(version);
  if (!match || match[0] !== version) return null;
  const prerelease = match[4]?.split(".") ?? [];
  if (prerelease.some((part) => /^\d+$/.test(part) && part.length > 1 && part[0] === "0")) return null;
  return { core: [BigInt(match[1]), BigInt(match[2]), BigInt(match[3])], prerelease };
}

export function isValidVersion(version: string): boolean {
  return parse(version) !== null;
}

/** Full SemVer precedence; build metadata never affects ordering. */
export function compareVersions(a: string, b: string): number {
  const left = parse(a, true);
  const right = parse(b, true);
  if (!left || !right) throw new Error("Invalid semantic version");
  for (let i = 0; i < 3; i++) {
    if (left.core[i] !== right.core[i]) return left.core[i] > right.core[i] ? 1 : -1;
  }
  if (!left.prerelease.length || !right.prerelease.length) {
    return left.prerelease.length === right.prerelease.length ? 0 : left.prerelease.length ? -1 : 1;
  }
  for (let i = 0; i < Math.max(left.prerelease.length, right.prerelease.length); i++) {
    const x = left.prerelease[i];
    const y = right.prerelease[i];
    if (x === undefined || y === undefined) return x === y ? 0 : x === undefined ? -1 : 1;
    if (x === y) continue;
    const xn = /^\d+$/.test(x);
    const yn = /^\d+$/.test(y);
    if (xn && yn) return BigInt(x) > BigInt(y) ? 1 : -1;
    if (xn !== yn) return xn ? -1 : 1;
    return x > y ? 1 : -1;
  }
  return 0;
}

export function highestVersion(versions: string[]): string {
  return versions.reduce((highest, version) => !highest || compareVersions(version, highest) > 0 ? version : highest, "");
}

export function suggestVersions(current: string): { patch: string; minor: string; major: string } {
  const parsed = parse(current, true);
  if (!parsed) return { patch: "0.1.0", minor: "0.1.0", major: "1.0.0" };
  const [major, minor, patch] = parsed.core;
  return {
    patch: `${major}.${minor}.${parsed.prerelease.length ? patch : patch + 1n}`,
    minor: `${major}.${minor + 1n}.0`,
    major: `${major + 1n}.0.0`,
  };
}

