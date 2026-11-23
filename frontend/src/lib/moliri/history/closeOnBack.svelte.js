// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Mirrored from Moliri (src/lib/history/closeOnBack.svelte.js) by frontend/scripts/sync-moliri.mjs.
// Do not change it here: change it in Moliri, then pnpm sync:moliri.

// The back button closes this overlay: wires overlayHistory.js to SvelteKit's shallow routing
// (`pushState`, `replaceState`, `page.state`). Call it while the component initialises:
//
//     closeOnBack({ prefix: 'sheet', isOpen: () => open, close: () => { open = false; } });
//
// `close` runs when back was pressed; return false to stay open (unsaved input the user wants to
// keep). Closing by the overlay's own button needs nothing more: the helper notices `isOpen` going
// false and takes the history entry out again.
import { browser } from '$app/environment';
import { navigating, page } from '$app/state';
import { afterNavigate, beforeNavigate, pushState, replaceState } from '$app/navigation';
import { onDestroy, untrack } from 'svelte';
import { createBackQueue, createOverlayHistory, overlayKey } from './overlayHistory.js';

// One queue for the whole tab: a push waits for the backs asked before it.
const queue = createBackQueue({
    back: () => history.back(),
    push: (state) => pushState('', state),
    delayMs: 40
});
/** A navigation has started since the overlay closed: its back would cancel it. */
const navigationStarted = () => navigating.type != null;
// After SvelteKit's own popstate handler (it may be registered after this module ran), so a
// waiting push reads the state of the entry the back landed on.
if (browser) addEventListener('popstate', () => setTimeout(() => queue.popped(), 0));

/**
 * @param {object} opts
 * @param {() => boolean} opts.isOpen
 * @param {() => (boolean|void)} opts.close   back was pressed; false keeps the overlay open
 * @param {string} [opts.key]     a fixed key, for overlays that open again when their entry comes
 *                                back (with `reopen`); otherwise a key of this instance
 * @param {string} [opts.prefix]  readable start of the instance key
 * @param {() => void} [opts.reopen]  open again when the entry comes back (back from a page opened
 *                                    out of the overlay, or forward)
 * @param {() => boolean} [opts.enabled]  false: no history entry (the overlay is part of the page
 *                                        at this width, say)
 */
export function closeOnBack(opts) {
    if (!browser) return;
    const key = opts.key ?? overlayKey(opts.prefix ?? 'overlay');
    const enabled = opts.enabled ?? (() => true);
    const machine = createOverlayHistory(key, {
        state: () => page.state,
        push: (stateAtPush) => queue.push(stateAtPush),
        replace: (state) => replaceState('', state),
        back: () => queue.back(navigationStarted),
        close: opts.close,
        reopen: opts.reopen
    });
    const openNow = () => enabled() && opts.isOpen();

    $effect(() => {
        const open = openNow();
        void page.state;
        untrack(() => machine.sync(open));
    });
    beforeNavigate(() => machine.leaving());
    afterNavigate(() => {
        machine.arrived();
        // The effect above may not run again (nothing it reads changed), so take stock now.
        queueMicrotask(() => machine.sync(untrack(openNow)));
    });
    onDestroy(() => {
        // After the current update: the overlay may be going away because the page navigates.
        queueMicrotask(() => machine.destroyed());
    });
}
