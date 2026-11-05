// QA-only vite config (CAT-TAURI-1): serves the real desktop UI on 0.0.0.0 for LAN manual QA, with a
// browser shim for the Tauri IPC bridge and a same-origin proxy to the QA API (no CORS needed).
//   QA_API_URL (default http://127.0.0.1:38082)  QA_PORT (default 1420)
import { defineConfig, mergeConfig } from "vite";
import { readFileSync } from "fs";
import { resolve } from "path";
import base from "./vite.config";

const shim = readFileSync(resolve(__dirname, "qa/tauri-ipc-shim.js"), "utf8");

export default defineConfig(async (env) => {
  const b = await (base as unknown as (e: typeof env) => Promise<object>)(env);
  return mergeConfig(b, {
    plugins: [
      {
        name: "qa-tauri-ipc-shim",
        transformIndexHtml: () => [{ tag: "script", children: shim, injectTo: "head-prepend" }],
      },
    ],
    server: {
      host: "0.0.0.0", allowedHosts: true,
      port: Number(process.env.QA_PORT || 1420),
      proxy: {
        "/qa-api": {
          target: process.env.QA_API_URL || "http://127.0.0.1:38082",
          changeOrigin: true,
          rewrite: (p: string) => p.replace(/^\/qa-api/, ""),
        },
      },
    },
    preview: { host: "0.0.0.0", allowedHosts: true, port: Number(process.env.QA_PORT || 1420), strictPort: true },
  });
});
