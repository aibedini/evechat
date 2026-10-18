import { defineConfig } from "vite";

export default defineConfig({
  base: "/setup/",
  build: {
    outDir: "../../embed/setup",
    emptyOutDir: true,
    target: "es2018",
    assetsDir: "",
    rollupOptions: {
      output: {
        entryFileNames: "setup.js",
        assetFileNames: "setup.[ext]",
      },
    },
  },
  server: {
    port: 5174,
    proxy: {
      "/api": { target: "http://127.0.0.1:3000", changeOrigin: true },
      "/assets": { target: "http://127.0.0.1:3000", changeOrigin: true },
    },
  },
});
