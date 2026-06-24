import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";
import path from "path";

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      devOptions: {
        enabled: true,
      },
      manifest: {
        name: "Openclaw Orchestrator",
        short_name: "Claworc",
        description: "Kubernetes dashboard for managing OpenClaw instances",
        start_url: "/",
        display: "standalone",
        background_color: "#111827",
        theme_color: "#111827",
        icons: [
            {
                src: "/pwa_images/launchericon-48x48.png",
                sizes: "48x48",
                type: "image/png",
            },          {
            src: "/pwa_images/launchericon-72x72.png",
            sizes: "72x72",
            type: "image/png",
          },
          {
            src: "/pwa_images/launchericon-96x96.png",
            sizes: "96x96",
            type: "image/png",
          },
          {
            src: "/pwa_images/launchericon-144x144.png",
            sizes: "144x144",
            type: "image/png",
          },
          {
            src: "/pwa_images/launchericon-192x192.png",
            sizes: "192x192",
            type: "image/png",
          },
          {
            src: "/pwa_images/launchericon-512x512.png",
            sizes: "512x512",
            type: "image/png",
          },
          {
            src: "/pwa_images/launchericon-512x512.png",
            sizes: "512x512",
            type: "image/png",
            purpose: "maskable",
          },
        ],
      },
      workbox: {
        // Deliberately exclude HTML so the app shell (index.html) is always
        // fetched from the network. An offline / off-VPN cold start should
        // surface the browser's own connectivity error instead of serving a
        // stale login form that then fails mysteriously on submit.
        globPatterns: ["**/*.{js,css,ico,png,svg,woff2}"],
        // No navigateFallback — server handles SPA routing.
        // This prevents the SW from intercepting navigation to /openclaw/.
        // Must be explicitly null to override VitePWA's default of "index.html".
        navigateFallback: null,
        runtimeCaching: [
          {
            urlPattern: /^https?:\/\/.*\/(api|openclaw)\//,
            handler: "NetworkOnly",
          },
        ],
      },
    }),
  ],
  build: {
    // Down-level the production bundle to a fixed browser baseline rather than
    // "esnext". "esnext" tells esbuild to emit the very newest syntax with
    // minimal transpilation; an up-to-date desktop Chrome copes, but an older
    // mobile Safari / Android WebView can fail to *parse* a bleeding-edge token
    // a dependency happens to ship, and a parse failure means React never
    // mounts — a blank white screen with no error.
    //
    // The floor is set by a dependency that uses top-level await (a WebCodecs
    // feature probe), which is why this can't go below Safari 15 / Chrome 89.
    // Pinning here down-levels everything newer than that baseline so phones on
    // Safari 15/16 are guaranteed a bundle their engine can parse.
    target: ["es2022", "chrome89", "edge89", "firefox89", "safari15"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8000",
        changeOrigin: true,
        autoRewrite: true,
        ws: true,
      },
      "/health": {
        target: "http://127.0.0.1:8000",
        changeOrigin: true,
        autoRewrite: true,
      },
      "/openclaw": {
        target: "http://127.0.0.1:8000",
        changeOrigin: true,
        autoRewrite: true,
        ws: true,
      },
    },
  },
});
