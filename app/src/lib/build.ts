/**
 * The app's own build stamp (#1527): `__APP_BUILD__` is a Vite `define`
 * (`vite.config.ts`), not `import.meta.env` -- it is swapped for a literal
 * string at build time, deployed commit (`GITHUB_SHA`) or `'dev'`, rather
 * than read at runtime off `process.env`, which the browser has none of.
 * Re-exported through a plain function, the same shape `apiBaseURL()`
 * gives `import.meta.env.VITE_API_BASE_URL`, so a caller reads one thing
 * from `#lib` rather than the bare identifier scattered through the app.
 */
export function appBuild(): string {
	return __APP_BUILD__;
}
