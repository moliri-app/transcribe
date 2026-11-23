// Loader of the `js` adapter (plain modules). It is a custom loader (`loader: 'custom'` in
// wuchale.config.js), so wuchale does not rewrite it: the default vite loader keeps the runtimes
// in a plain array, and a text a component reads from a plain module (field type names, AI error
// labels …) would then not follow a language switch, which happens without a reload. Here the
// runtimes sit in `$state`, as in main.loader.svelte.js, so whatever reads them is re-run on
// switch. SSR is off, so there is no server loader.
import { defaultCollection, registerLoaders } from 'wuchale/load-utils'
import { loadCatalog, loadCount } from './.wuchale/js.proxy.js'
import { runtimes } from './js.runtimes.svelte.js'

const key = 'js'

export const getRuntime = registerLoaders(key, loadCatalog, loadCount, defaultCollection(runtimes))
export const getRuntimeRx = getRuntime
