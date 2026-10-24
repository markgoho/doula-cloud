/*
 * Runs before each browser spec's own imports (#1856). The cascade layer
 * order has to reach the document before any component's stylesheet does,
 * or `components` becomes the lowest layer once a spec loads `app.css`
 * (`#lib/styles/layers.css` says why). Only the order, not the app's
 * rules: a spec that wants the real cascade imports `app.css` itself.
 */
import '#lib/styles/layers.css';
