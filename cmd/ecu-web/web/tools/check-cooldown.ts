/**
 * Dependency cooldown gate.
 *
 * Supply-chain quarantine: refuse to build against any installed npm
 * package version published less than COOLDOWN_DAYS ago. Freshly
 * published malicious versions are typically caught and yanked within a
 * few days; waiting out a cooldown window dodges that class of attack.
 *
 * It walks the entire installed tree under node_modules (direct AND
 * transitive), looks up each version's publish time from the npm
 * registry `time` map, and fails the build if any version is too young.
 * Verified publish dates are cached in cooldown.lock.json so normal
 * builds run offline and only a version bump triggers a network check.
 *
 * Run: `bun run tools/check-cooldown.ts`
 */

const COOLDOWN_DAYS = 7;
const COOLDOWN_MS = COOLDOWN_DAYS * 24 * 60 * 60 * 1000;
const CACHE_PATH = new URL("../cooldown.lock.json", import.meta.url).pathname;
const NODE_MODULES = new URL("../node_modules", import.meta.url).pathname;

type Cache = Record<string, string>; // "name@version" -> publish ISO date

async function loadCache(): Promise<Cache> {
  try {
    return (await Bun.file(CACHE_PATH).json()) as Cache;
  } catch {
    return {};
  }
}

/** Enumerate every installed package as name@version (deduped). */
async function installedVersions(): Promise<Map<string, string>> {
  const out = new Map<string, string>(); // key -> name
  const glob = new Bun.Glob("**/package.json");
  for await (const rel of glob.scan({ cwd: NODE_MODULES, onlyFiles: true })) {
    try {
      const pj = (await Bun.file(`${NODE_MODULES}/${rel}`).json()) as {
        name?: string;
        version?: string;
      };
      if (!pj.name || !pj.version) continue;
      out.set(`${pj.name}@${pj.version}`, pj.name);
    } catch {
      // ignore malformed/partial package.json
    }
  }
  return out;
}

/** Fetch the publish time for one name@version from the npm registry. */
async function publishTime(name: string, version: string): Promise<string> {
  const enc = name.replace("/", "%2f"); // scoped names: @scope/pkg
  const res = await fetch(`https://registry.npmjs.org/${enc}`);
  if (!res.ok) throw new Error(`registry ${name}: HTTP ${res.status}`);
  const meta = (await res.json()) as { time?: Record<string, string> };
  const t = meta.time?.[version];
  if (!t) throw new Error(`no publish time for ${name}@${version}`);
  return t;
}

async function main() {
  const cache = await loadCache();
  const installed = await installedVersions();
  const now = Date.now();
  const tooYoung: string[] = [];
  let fetched = 0;

  for (const [key, name] of installed) {
    const version = key.slice(name.length + 1);
    let iso = cache[key];
    if (!iso) {
      try {
        iso = await publishTime(name, version);
        cache[key] = iso;
        fetched++;
      } catch (e) {
        console.error(`cooldown: cannot verify ${key}: ${(e as Error).message}`);
        process.exitCode = 1;
        continue;
      }
    }
    const ageMs = now - Date.parse(iso);
    if (ageMs < COOLDOWN_MS) {
      const ageDays = (ageMs / 86_400_000).toFixed(1);
      tooYoung.push(`${key} (published ${ageDays}d ago, need ${COOLDOWN_DAYS}d)`);
    }
  }

  if (fetched > 0) {
    await Bun.write(CACHE_PATH, JSON.stringify(sortObject(cache), null, 2) + "\n");
  }

  if (tooYoung.length > 0) {
    console.error(
      `\n✗ cooldown gate FAILED — ${tooYoung.length} dependency version(s) younger than ${COOLDOWN_DAYS} days:`,
    );
    for (const t of tooYoung) console.error(`  - ${t}`);
    console.error(
      "\nPin to an older version, or wait out the cooldown window before building.\n",
    );
    process.exit(1);
  }

  console.log(
    `✓ cooldown gate passed — ${installed.size} package version(s) all ≥ ${COOLDOWN_DAYS} days old` +
      (fetched ? ` (${fetched} newly verified)` : " (from cache)"),
  );
}

function sortObject(o: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(o).sort(([a], [b]) => a.localeCompare(b)));
}

await main();
