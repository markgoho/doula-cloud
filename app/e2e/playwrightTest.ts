// ops/'s e2e suite reuses this directory's stack and helpers, which
// import @playwright/test from app/'s node_modules. Playwright refuses a
// second copy of itself in one run, so ops/ runs app/'s Playwright and
// takes it through here rather than installing its own.
export * from '@playwright/test';
