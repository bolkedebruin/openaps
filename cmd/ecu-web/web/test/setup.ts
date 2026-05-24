// Registers a happy-dom-backed global DOM (window, document,
// customElements, …) so Lit components can be defined and rendered
// inside `bun test`. Preloaded via bunfig.toml [test].preload.
import { GlobalRegistrator } from "@happy-dom/global-registrator";

GlobalRegistrator.register();
