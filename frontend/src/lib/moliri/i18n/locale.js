// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Mirrored from Moliri (src/lib/i18n/locale.js) by frontend/scripts/sync-moliri.mjs.
// Do not change it here: change it in Moliri, then pnpm sync:moliri.

import { loadLocale } from 'wuchale/load-utils';

/**
 * The interface language for wuchale (docs/wuchale-migration-plan.md, target architecture).
 *
 * Same exports as Paraglide's runtime had (`getLocale`, `setLocale`, `locales`, `baseLocale`,
 * `isLocale`, `cookieName`, `cookieMaxAge`, `localStorageKey`), so the codemod only swapped the
 * import path. The language is resolved as Paraglide's strategy did it: the stored choice, then the
 * browser languages, then English. It is stored under `MOLIRI_LOCALE`; Paraglide's old
 * `PARAGLIDE_LOCALE` is read once, as long as nothing is stored under the new key, and copied over.
 *
 * `LanguageSwitch.svelte` sets the language and then writes the `MOLIRI_LOCALE` cookie for the
 * backend (`localeCookie.js`).
 */

export const baseLocale = 'en';
export const locales = /** @type {const} */ (['en', 'de', 'es', 'fr', 'ja', 'uk', 'el']);
export const localStorageKey = 'MOLIRI_LOCALE';
export const cookieName = 'MOLIRI_LOCALE';
export const cookieMaxAge = 34560000;

/** Paraglide's key, read once to carry an existing choice over. */
const legacyKey = 'PARAGLIDE_LOCALE';

/** @typedef {(typeof locales)[number]} Locale */

/** @type {Locale | undefined} */
let current;

/**
 * @param {unknown} locale
 * @returns {locale is Locale}
 */
export function isLocale(locale) {
	return typeof locale === 'string' && locales.some((l) => l === locale.toLowerCase());
}

/** @param {string | null | undefined} value */
const asLocale = (value) =>
	isLocale(value) ? /** @type {Locale} */ (value.toLowerCase()) : undefined;

/** @returns {Storage | undefined} */
function storage() {
	try {
		return typeof localStorage === 'undefined' ? undefined : localStorage;
	} catch {
		return undefined; // storage blocked (privacy settings, sandboxed frame)
	}
}

/** The first browser language we have, by full tag or base tag (as Paraglide's `preferredLanguage`). */
function fromNavigator() {
	const languages = typeof navigator === 'undefined' ? [] : (navigator.languages ?? []);
	for (const lang of languages) {
		const found = asLocale(lang) ?? asLocale(lang.split('-')[0]);
		if (found) return found;
	}
	return undefined;
}

/** @returns {Locale} */
function resolve() {
	const store = storage();
	const stored = asLocale(store?.getItem(localStorageKey));
	if (stored) return stored;
	const legacy = asLocale(store?.getItem(legacyKey));
	if (legacy) {
		store?.setItem(localStorageKey, legacy);
		return legacy;
	}
	return fromNavigator() ?? baseLocale;
}

/** @returns {Locale} */
export function getLocale() {
	current ??= resolve();
	return current;
}

/** @type {Set<(locale: Locale) => void>} */
const listeners = new Set();

/**
 * Calls `listener` with the new language after each `setLocale`, once its catalogues are loaded:
 * for texts outside wuchale that would otherwise stay in the language they were built in (Uppy).
 *
 * @param {(locale: Locale) => void} listener
 * @returns {() => void} removes the listener
 */
export function onLocaleChange(listener) {
	listeners.add(listener);
	return () => listeners.delete(listener);
}

/**
 * Stores the language and loads its catalogues; texts rendered by wuchale follow without a reload.
 * `reload: true` reloads the page afterwards, for texts that are read only once.
 *
 * @param {Locale} newLocale
 * @param {{ reload?: boolean }} [options]
 */
export async function setLocale(newLocale, options = {}) {
	const locale = asLocale(newLocale);
	if (!locale)
		throw new Error(`Invalid locale: ${newLocale}. Expected one of: ${locales.join(', ')}`);
	current = locale;
	storage()?.setItem(localStorageKey, locale);
	await loadLocale(locale);
	for (const listener of listeners) listener(locale);
	if (options.reload && typeof window !== 'undefined') window.location.reload();
}
