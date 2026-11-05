// QA-only vite config (CAT-TAURI-1): serves the real wizard UI on 127.0.0.1 (LAN only via QA_HOST) with a browser IPC shim.
//   QA_PORT (default 1421)
import { defineConfig, mergeConfig } from "vite";
import { readFileSync } from "fs";
import { resolve } from "path";
import base from "./vite.config";

// Default loopback. For LAN QA set QA_HOST=0.0.0.0 and QA_ALLOWED_HOSTS=192.168.1.115,localhost (comma list).
const QA_HOST = process.env.QA_HOST || "127.0.0.1";
const ALLOWED = (process.env.QA_ALLOWED_HOSTS || "localhost,127.0.0.1").split(",").map((h) => h.trim()).filter(Boolean);
const shim = readFileSync(resolve(__dirname, "qa/tauri-ipc-shim.js"), "utf8");

export default mergeConfig(base, defineConfig({
  plugins: [
    {
      name: "qa-tauri-ipc-shim",
      transformIndexHtml: () => [{ tag: "script", children: shim, injectTo: "head-prepend" }],
    },
  ],
  server: { host: QA_HOST, allowedHosts: ALLOWED, port: Number(process.env.QA_PORT || 1421), strictPort: true },
  preview: { host: QA_HOST, allowedHosts: ALLOWED, port: Number(process.env.QA_PORT || 1421), strictPort: true },
}));
