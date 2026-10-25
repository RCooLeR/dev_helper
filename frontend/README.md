# Embedded web UI

The Go binary embeds the production React and Bootstrap bundles. Starting
devhelper and using the UI require no Node.js installation or CDN connection.

To edit the UI, use Node.js 22 or newer:

```sh
cd frontend
npm ci
npm test
npm run build
```

Commit the source, `package-lock.json`, and regenerated
`internal/http/ui/app.js` and `app.css` together. Then rebuild the Go binary.
`internal/http/ui/index.html` is the Go HTML template; `style.css` holds local
style overrides. esbuild compiles JSX ahead of time and retains dependency
license notices in the generated files.
