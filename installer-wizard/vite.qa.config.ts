// QA-only vite config (CAT-TAURI-1): serves the real wizard UI on 0.0.0.0 with a browser IPC shim.
//   QA_PORT (default 1421)
import { defineConfig, mergeConfig } from "vite";
import { readFileSync } from "fs";
import { resolve } from "path";
import base from "./vite.config";

const shim = readFileSync(resolve(__dirname, "qa/tauri-ipc-shim.js"), "utf8");

export default mergeConfig(base, defineConfig({
  plugins: [
    {
      name: "qa-tauri-ipc-shim",
      transformIndexHtml: () => [{ tag: "script", children: shim, injectTo: "head-prepend" }],
    },
  ],
  server: { host: "0.0.0.0", allowedHosts: true, port: Number(process.env.QA_PORT || 1421), strictPort: true },
  preview: { host: "0.0.0.0", allowedHosts: true, port: Number(process.env.QA_PORT || 1421), strictPort: true },
}));
