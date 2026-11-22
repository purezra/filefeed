import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";

const cwd = fileURLToPath(new URL(".", import.meta.url));
const baseEnv = { ...process.env, CGO_ENABLED: "0" };

function go(args, env = baseEnv) {
  const result = spawnSync("go", args, { cwd, stdio: "inherit", env });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

// Add the typed transcript entry point to a disposable vendor copy of
// pinned Magpie. Its module omits the updater's generated launcher embed.
// The companion does not import the updater; the build checks its own deps.
go(["mod", "vendor", "-e"]);
copyFileSync(
  new URL("magpie_parts.go.txt", import.meta.url),
  new URL("vendor/github.com/yetone/magpie/internal/sessions/filefeed_parts.go", import.meta.url),
);

if (process.argv.includes("--test")) {
  go(["test", "./..."]);
  process.exit(0);
}

const arches = process.argv.includes("--host")
  ? [process.arch === "arm64" ? "arm64" : "amd64"]
  : ["amd64", "arm64"];

for (const arch of arches) {
  const dir = fileURLToPath(new URL(`../../resources/windows-${arch}/`, import.meta.url));
  mkdirSync(dir, { recursive: true });
  go(["build", "-trimpath", "-ldflags", "-s -w", "-o", `${dir}ff.exe`, "."], { ...baseEnv, GOOS: "windows", GOARCH: arch });
}
