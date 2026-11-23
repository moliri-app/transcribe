// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Mirrored from Moliri (src/lib/history/overlayHistory.js) by frontend/scripts/sync-moliri.mjs.
// Do not change it here: change it in Moliri, then pnpm sync:moliri.

// The browser's back button closes sheets and dialogs (docs/mobile-companion-plan.md §2.2).
//
// Opening an overlay (sheet, dialog, full-screen view) adds a history entry through SvelteKit's
// shallow routing: the entry has the same URL and lists the open overlays in `page.state.overlays`.
// Back pops that entry, `page.state` loses the key, and the overlay closes. Closing it by its own
// button goes back one entry, so the history holds no dead entries. An overlay with unsaved input
// may refuse to close on back (its `close` returns false): the entry is pushed again.
//
// This module is the logic without Svelte, so it can be tested on its own; closeOnBack.svelte.js
// wires it to `page.state`, `pushState` and the navigation hooks.

export const STATE_KEY = 'overlays';

/** @param {any} state @returns {string[]} the overlay keys of a history entry, oldest first */
export function overlaysOf(state) {
    const list = state?.[STATE_KEY];
    return Array.isArray(list) ? list.filter((k) => typeof k === 'string') : [];
}

/** @param {any} state @param {string} key @returns {Record<string, any>} */
export function withOverlay(state, key) {
    return { ...(state ?? {}), [STATE_KEY]: [...overlaysOf(state).filter((k) => k !== key), key] };
}

/** @param {any} state @param {string} key @returns {Record<string, any>} */
export function withoutOverlay(state, key) {
    return { ...(state ?? {}), [STATE_KEY]: overlaysOf(state).filter((k) => k !== key) };
}

let counter = 0;
/**
 * A key for one overlay instance. Random, so a key kept in an old history entry (after a reload)
 * does not match a new instance by chance. Overlays that reopen on back use a fixed key instead.
 * @param {string} prefix
 */
export function overlayKey(prefix) {
    counter += 1;
    return `${prefix}-${counter}-${Math.random().toString(36).slice(2, 8)}`;
}

/**
 * @typedef {object} OverlayHistoryDeps
 * @property {() => any} state          the current `page.state`
 * @property {(state: () => any) => void} push  adds an entry with this state, read when the push
 *                                      happens (it may wait for a back, see createBackQueue)
 * @property {(state: any) => void} replace  replaces the current entry's state
 * @property {() => void} back          goes back one entry
 * @property {() => (boolean|void)} close  back was pressed: close the overlay; false keeps it open
 * @property {() => void} [reopen]      the overlay's entry came back (back or forward from another
 *                                      page): open it again
 */

/**
 * The history of one overlay. Call `sync(open)` whenever the overlay opens or closes and whenever
 * `page.state` changes; `leaving()` before a real navigation (not a shallow one), `arrived()`
 * after it, `destroyed()` when the overlay's component goes away.
 * @param {string} key
 * @param {OverlayHistoryDeps} deps
 */
export function createOverlayHistory(key, deps) {
    let wasOpen = false;
    /** This overlay's entry is in the history, pushed by us or found there on opening. */
    let owns = false;
    /** A push was asked for and has not happened yet. */
    let pending = false;
    /** Between beforeNavigate and afterNavigate of a real navigation. */
    let navigating = false;
    /** A real navigation ended: the next sync takes stock of the new entry. */
    let arrivedFlag = false;
    /** We went back to take our entry out; until it is gone, its key is no reason to reopen. */
    let releasing = false;

    const inState = () => overlaysOf(deps.state()).includes(key);
    const isTop = () => overlaysOf(deps.state()).at(-1) === key;

    function push() {
        pending = true;
        deps.push(() => withOverlay(deps.state(), key));
        // Usually the push happens at once; when it waits for a back, the next sync sees it.
        if (inState()) {
            pending = false;
            owns = true;
        }
    }

    /** Takes our entry out of the history: back when it is on top, else only our key. */
    function release() {
        if (isTop()) {
            releasing = true;
            deps.back();
        } else deps.replace(withoutOverlay(deps.state(), key));
    }

    /** @param {boolean} open */
    function sync(open) {
        if (releasing && !inState()) releasing = false;
        if (pending) {
            if (!inState()) return;
            pending = false;
            owns = true;
        }

        if (arrivedFlag) {
            arrivedFlag = false;
            if (open) {
                // Still open on the new entry: it closes on back from here.
                wasOpen = true;
                if (inState()) owns = true;
                else push();
                return;
            }
            wasOpen = false;
            owns = false;
            releasing = false;
            if (inState() && deps.reopen) deps.reopen();
            return;
        }

        if (open && !wasOpen) {
            // Opened, by the user or by reopen().
            wasOpen = true;
            if (inState()) owns = true;
            else push();
            return;
        }

        if (!open && wasOpen) {
            // Closed by its own button or code.
            wasOpen = false;
            if (owns && !navigating && inState()) release();
            owns = false;
            return;
        }

        if (open && owns && !navigating && !inState()) {
            // Back was pressed: our entry is gone.
            owns = false;
            if (deps.close() === false) push();
            return;
        }

        if (!open && !wasOpen && !navigating && !releasing && inState() && deps.reopen) {
            // The entry came back by a shallow step (forward).
            deps.reopen();
        }
    }

    return {
        sync,
        leaving() {
            navigating = true;
        },
        arrived() {
            navigating = false;
            arrivedFlag = true;
        },
        /** The overlay's component went away (often in the same update that closed it). */
        destroyed() {
            if (owns && !navigating && inState()) release();
            owns = false;
            wasOpen = false;
        },
        /** For tests. */
        get owns() {
            return owns;
        }
    };
}

/**
 * Pushes wait for the backs before them: `history.back()` is asynchronous, and a push made before
 * its popstate would land on the entry that is about to go. Switching from one sheet to another
 * goes back for the first and pushes for the second.
 *
 * A back is taken a moment later (`delayMs`) and dropped when `dropIf` says so by then: an
 * overlay often closes in the same click that starts a navigation (an object opened from the
 * basket), and SvelteKit's `beforeNavigate` comes only after an await. A back taken then would
 * cancel the navigation; dropped, the overlay's entry stays behind the new page, and back from
 * there finds it again.
 * @param {{ back: () => void, push: (state: any) => void, timeoutMs?: number, delayMs?: number,
 *           setTimer?: (fn: () => void, ms: number) => any, clearTimer?: (t: any) => void }} io
 */
export function createBackQueue(io) {
    let backs = 0;
    /** @type {(() => void)[]} */
    let queue = [];
    /** @type {any} */
    let timer = null;
    const setTimer = io.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
    const clearTimer = io.clearTimer ?? ((t) => clearTimeout(t));

    function flush() {
        const q = queue;
        queue = [];
        for (const fn of q) fn();
    }

    function settled() {
        if (backs > 0) backs -= 1;
        if (backs === 0) {
            if (timer) clearTimer(timer);
            timer = null;
            flush();
        }
    }

    return {
        /** @param {() => boolean} [dropIf] true when the moment has passed: no back then */
        back(dropIf) {
            backs += 1;
            setTimer(() => {
                if (dropIf?.()) return settled();
                io.back();
                // A popstate that never comes (the browser ignored the step) must not hold pushes forever.
                if (timer) clearTimer(timer);
                timer = setTimer(() => {
                    timer = null;
                    backs = 0;
                    flush();
                }, io.timeoutMs ?? 600);
            }, io.delayMs ?? 0);
        },
        /** @param {() => any} stateAtPush the state, read when the push really happens */
        push(stateAtPush) {
            if (backs > 0) queue.push(() => io.push(stateAtPush()));
            else io.push(stateAtPush());
        },
        /** Call on every popstate. */
        popped() {
            settled();
        },
        get waiting() {
            return backs;
        }
    };
}
