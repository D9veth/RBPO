import { mkdirSync, readFileSync } from "node:fs";
import { spawn, spawnSync } from "node:child_process";
import { resolve } from "node:path";
const config = Object.fromEntries(
  readFileSync(".env", "utf8")
    .split(/\r?\n/)
    .filter((line) => /^[A-Z_]+=/.test(line))
    .map((line) => {
      const index = line.indexOf("=");
      return [line.slice(0, index), line.slice(index + 1)];
    }),
);
const processEnv = {
  ...config,
  ...process.env,
  DATABASE_URL:
    process.env.DATABASE_URL ??
    config.DATABASE_URL ??
    "postgres://campus:campus_dev_only@127.0.0.1:55432/campus?sslmode=disable",
};
mkdirSync(".local", { recursive: true });
const binary = resolve(".local", process.platform === "win32" ? "campus.exe" : "campus");
const build = spawnSync("go", ["build", "-o", binary, "./cmd/server"], {
  env: processEnv,
  stdio: "inherit",
});
if (build.error) throw build.error;
if (build.status !== 0) process.exit(build.status ?? 1);
// Run the binary directly so shutdown signals reach the HTTP server.
const server = spawn(binary, [], {
  env: processEnv,
  stdio: "inherit",
});
server.on("error", (error) => {
  console.error(error.message);
  process.exitCode = 1;
});
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => server.kill(signal));
server.on("exit", (code) => {
  process.exitCode = code ?? 0;
});
