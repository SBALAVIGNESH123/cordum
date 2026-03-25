import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";
import { createRequire } from "module";

const require = createRequire(import.meta.url);

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      // Force CJS entries so Rollup's commonjs plugin can statically
      // resolve require("@dagrejs/graphlib"). The ESM build of dagre
      // wraps the require in a dynamic helper that Rollup cannot analyse.
      "@dagrejs/dagre": path.dirname(require.resolve("@dagrejs/dagre")) + "/dagre.cjs.js",
      "@dagrejs/graphlib": require.resolve("@dagrejs/graphlib"),
    },
  },
  build: {
    commonjsOptions: {
      include: [/node_modules/],
    },
  },
  optimizeDeps: {
    include: ["@dagrejs/dagre", "@dagrejs/graphlib"],
  },
  server: {
    allowedHosts: true,
    proxy: {
      "/api": {
        target: process.env.VITE_API_TARGET || "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
