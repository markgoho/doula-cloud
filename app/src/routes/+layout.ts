// adapter-static's fallback mode requires every page to skip SSR --
// the fallback page (build/200.html) has no page-specific server data
// to hydrate, so SvelteKit must run entirely client-side. See
// vite.config.ts's adapter({ fallback: '200.html' }).
// eslint-disable-next-line unicorn/consistent-boolean-name -- `ssr` is SvelteKit's mandated export name for this config
export const ssr = false;
