import js from "@eslint/js";
import stylistic from "@stylistic/eslint-plugin";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import { globalIgnores } from "eslint/config";
import tseslint from "typescript-eslint";

export default tseslint.config([
  globalIgnores(["dist", "dev-dist", "vite.config.ts"]),
  {
    // dashbrr-only: qui has no equivalent script, and the shared config below
    // matches .ts/.tsx only, so this would otherwise go unlinted.
    files: ["scripts/**/*.js"],
    extends: [js.configs.recommended],
    languageOptions: {
      globals: {
        ...globals.node,
        process: true,
        console: true,
      },
    },
  },
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      "@stylistic": stylistic,
      "react-hooks": reactHooks,
    },
    rules: {
      "@stylistic/quotes": ["warn", "double"],
      "@stylistic/comma-dangle": [
        "warn",
        {
          arrays: "always-multiline",
          objects: "always-multiline",
          imports: "never",
          exports: "always-multiline",
          functions: "never",
        },
      ],
      "@stylistic/indent": ["error", 2, { "SwitchCase": 1 }],
      // qui sets multiline-ternary to "never"; omitted here because dashbrr
      // uses nested multiline ternaries for JSX className logic, and the
      // autofix collapses those into 300+ char lines.
      "@stylistic/no-trailing-spaces": ["warn"],
      "@stylistic/object-curly-spacing": ["error", "always"],
      "@typescript-eslint/no-unused-vars": ["warn"],
      "@typescript-eslint/no-explicit-any": "error",
      "linebreak-style": ["error", "unix"],
      "react-refresh/only-export-components": ["warn", { allowConstantExport: true }],
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
]);
