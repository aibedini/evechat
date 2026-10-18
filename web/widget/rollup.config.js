import typescript from "@rollup/plugin-typescript";
import terser from "@rollup/plugin-terser";

export default {
  input: "src/index.ts",
  output: {
    file: "dist/widget.js",
    format: "iife",
    name: "EveChat",
    sourcemap: false,
    generatedCode: "es2015",
  },
  plugins: [
    typescript({ tsconfig: "./tsconfig.json" }),
    terser({
      compress: { passes: 3, toplevel: true, drop_console: false, pure_getters: true, unsafe_arrows: true },
      mangle: { toplevel: true, properties: { regex: /^_/ } },
      format: { comments: false },
    }),
  ],
};
