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

// Every page is a URL with no trailing slash -- /pilot-terms, /p/<slug> --
// which is SvelteKit's default `trailingSlash: 'never'`, so it is not set
// here. The build writes pilot-terms.html, and firebase.json's `cleanUrls`
// serves it at /pilot-terms (#1475). That is the form
// website.HostedPageURL hands Stripe (#382), so the URL Stripe holds
// answers 200 with no redirect first.
