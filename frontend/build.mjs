import { build } from "esbuild";
import { fileURLToPath } from "node:url";

await build({
  absWorkingDir: fileURLToPath(new URL(".", import.meta.url)),
  entryPoints: ["src/app.jsx"],
  outfile: "../internal/http/ui/app.js",
  bundle: true,
  minify: true,
  platform: "browser",
  target: ["es2022"],
  jsx: "automatic",
  define: { "process.env.NODE_ENV": '"production"' },
  legalComments: "eof",
  logLevel: "info",
});
