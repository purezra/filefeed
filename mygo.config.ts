import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "FileFeed",
  identifier: "com.local.filefeed",
  version: "0.1.0",
  devUrl: "http://localhost:5173",
  devCommand: "node tools/ff/build.mjs --host && npm run dev:web",
  buildCommand: "node tools/ff/build.mjs && npm run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
