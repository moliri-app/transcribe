<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<script>
	// The language of the interface: the languages Moliri speaks, each named in itself. Until one is
	// chosen it follows the browser's (the system's) language, otherwise English
	// ($lib/moliri/i18n/locale.js). The page reloads on a switch, so dates and numbers formatted
	// once follow too.
	import { getLocale, setLocale } from '$lib/moliri/i18n/locale.js';

	/** @type {{ id?: string }} */
	let { id } = $props();

	const SPRACHEN = [
		['de', 'Deutsch'],
		['en', 'English'],
		['es', 'Español'],
		['fr', 'Français'],
		['ja', '日本語'],
		['uk', 'Українська'],
		['el', 'Ελληνικά']
	];

	const aktuell = getLocale();
</script>

<select {id} class="feld" value={aktuell} onchange={(e) => setLocale(/** @type {any} */ (e.currentTarget.value), { reload: true })}>
	{#each SPRACHEN as [code, name] (code)}
		<option value={code} lang={code}>{name}</option>
	{/each}
</select>
