// Every page on doula.cloud is a static HTML file on Firebase Hosting's
// CDN, built ahead of time: nothing has to be up at the moment a Client
// or a Stripe reviewer opens a Practice page (#441).
// eslint-disable-next-line unicorn/consistent-boolean-name -- `prerender` is SvelteKit's mandated export name for this config
export const prerender = true;

// And none of them ships JavaScript. Each page is prose and a mailto
// link; there is nothing for a script to do, so the Rule of Least Power
// leaves it out rather than hydrating a page that never changes.
// eslint-disable-next-line unicorn/consistent-boolean-name -- `csr` is SvelteKit's mandated export name for this config
export const csr = false;

// Every page is a directory with an index.html in it -- /pilot-terms/,
// /p/<slug>/ -- which is the exact shape the Hugo site published. The URL
// Stripe holds for each Practice (#382), and the one the #443 probe
// requests, end in that slash.
export const trailingSlash = 'always';
