import { defineConfig } from "vite";

// The build output is embedded into the Go binary (internal/webui).
export default defineConfig({
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
    target: "es2022",
  },
  server: {
    port: 5173,
    proxy: {
      "/api": process.env.API_URL ?? "http://localhost:8000",
    },
  },
});
