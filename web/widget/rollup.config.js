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
    compact: true,
  },
  plugins: [
    typescript({ tsconfig: "./tsconfig.json" }),
    terser({
      ecma: 2015,
      compress: {
        passes: 3,
        toplevel: true,
        pure_getters: true,
        unsafe_arrows: true,
        hoist_funs: true,
        hoist_vars: true,
        inline: 3,
        reduce_funcs: true,
        collapse_vars: true,
        booleans_as_integers: true,
      },
      mangle: { toplevel: true, properties: { regex: /^_/ } },
      format: { comments: false },
    }),
  ],
};
