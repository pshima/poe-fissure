import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In dev, proxy API calls to the Go server (run it with APP_DEV=1 PORT=8080).
// In production the Go server serves this build's dist/ on the same origin.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { "/api": "http://localhost:8080" },
  },
  build: { outDir: "dist" },
});
