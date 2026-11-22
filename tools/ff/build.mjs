import { spawnSync } from "node:child_process";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
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
const transcript = new URL("vendor/github.com/yetone/magpie/internal/sessions/transcript.go", import.meta.url);
let source = readFileSync(transcript, "utf8");
function replace(before, after) {
  if (!source.includes(before)) throw new Error("Pinned Magpie transcript reader changed: " + before);
  source = source.replace(before, after);
}
// Keep the same branch traversal for dated and full Pi-format reads.
replace('Summary  string `json:"summary"`', 'Timestamp json.RawMessage `json:"timestamp"`\n\tSummary  string `json:"summary"`');
replace('ToolName string          `json:"toolName"`', 'Timestamp json.RawMessage `json:"timestamp"`\n\t\tToolName string          `json:"toolName"`');
replace("func piTranscript(path string, add func(bool, Part) bool) error {", `func piTranscript(path string, add func(bool, Part) bool) error {
	return piTranscriptEntries(path, func(_ piEntry, input bool, part Part) bool { return add(input, part) })
}

func piTranscriptEntries(path string, add func(piEntry, bool, Part) bool) error {`);
replace('!bytes.Contains(b, piMessage) && e.Type != "compaction"', 'e.Type != "message" && e.Type != "compaction"');
replace('var piMessage = []byte(`"type":"message"`)\n\n', '');
replace('if !add(p.Role != "assistant", p) {', 'if !add(branch[i], p.Role != "assistant", p) {');
writeFileSync(transcript, source);
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
