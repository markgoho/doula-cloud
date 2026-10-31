// A client-side app behind a sign-in, as app/ is: nothing to render on a
// server, and the build is one fallback page (vite.config.ts).
// eslint-disable-next-line unicorn/consistent-boolean-name -- `ssr` is SvelteKit's mandated export name
export const ssr = false;
