// QA-only vite config (CAT-TAURI-1): serves the real desktop UI on 127.0.0.1 (LAN only via QA_HOST), with a
// browser shim for the Tauri IPC bridge and a same-origin proxy to the QA API (no CORS needed).
//   QA_API_URL (default http://127.0.0.1:38082)  QA_PORT (default 1420)
import { defineConfig, mergeConfig } from "vite";
import { readFileSync } from "fs";
import { resolve } from "path";
import base from "./vite.config";

// Default loopback. For LAN QA set QA_HOST=0.0.0.0 and QA_ALLOWED_HOSTS=192.168.1.115,localhost (comma list).
const QA_HOST = process.env.QA_HOST || "127.0.0.1";
const ALLOWED = (process.env.QA_ALLOWED_HOSTS || "localhost,127.0.0.1").split(",").map((h) => h.trim()).filter(Boolean);
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
      host: QA_HOST, allowedHosts: ALLOWED,
      port: Number(process.env.QA_PORT || 1420),
      proxy: {
        "/qa-api": {
          target: process.env.QA_API_URL || "http://127.0.0.1:38082",
          changeOrigin: true,
          rewrite: (p: string) => p.replace(/^\/qa-api/, ""),
        },
      },
    },
    preview: { host: QA_HOST, allowedHosts: ALLOWED, port: Number(process.env.QA_PORT || 1420), strictPort: true },
  });
});
