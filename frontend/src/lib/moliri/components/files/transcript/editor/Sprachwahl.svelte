<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Sprachwahl.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Sprachwahl.svelte). -->
<!-- Sprache für einen Sprecher oder einen Bereich: ein Suchfeld mit Vorschlägen. Man tippt einen Namen
     („Platt“), einen anderen Namen („Niederdeutsch“) oder den Code („nds“) und wählt „Plattdeutsch nds“.
     Bekannte Sprachen stehen immer mit Namen da; ein Code außerhalb der Liste lässt sich trotzdem wählen.
     Was die Maschine nicht neu erkennen kann (Mundarten), sagt der Vorschlag gleich. Pfeiltasten wählen,
     Enter übernimmt, Esc schließt die Liste. value ist der Code, '' heißt nichts festgelegt. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { untrack } from 'svelte';
	import { sprache, vorschlaege } from './sprachen.js';

	/** @type {{ value: string, none: string, label?: string, disabled?: boolean }} */
	let { value = $bindable(''), none, label = m.transcript_language(), disabled = false } = $props();

	const id = 'sw' + Math.random().toString(36).slice(2, 8);
	const anzeige = (/** @type {string} */ c) => (c ? `${sprache(c).name} (${c})` : '');

	/** @type {HTMLInputElement | undefined} */ let feld = $state();
	let text = $state(untrack(() => anzeige(value)));
	let offen = $state(false);
	let gesucht = $state('');
	let aktiv = $state(0);

	// ohne Eingabe zuerst „nichts festlegen“, dann die häufigen Sprachen
	const liste = $derived([...(gesucht.trim() ? [] : [{ code: '', name: none, also: [], neu: true }]), ...vorschlaege(gesucht)]);

	function zeigen(q = '') {
		gesucht = q;
		aktiv = 0;
		offen = true;
	}
	function waehlen(/** @type {{ code: string }} */ s) {
		value = s.code;
		text = anzeige(s.code);
		offen = false;
	}
	function zurueck() {
		text = anzeige(value);
		offen = false;
	}
	function taste(/** @type {KeyboardEvent} */ e) {
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			if (!offen) return zeigen();
			aktiv = (aktiv + (e.key === 'ArrowDown' ? 1 : -1) + liste.length) % liste.length;
			document.getElementById(`${id}-${aktiv}`)?.scrollIntoView({ block: 'nearest' });
		} else if (e.key === 'Enter' && offen) {
			// Enter wählt hier und schickt den Dialog noch nicht ab
			e.preventDefault();
			if (liste[aktiv]) waehlen(liste[aktiv]);
		} else if (e.key === 'Escape' && offen) {
			// nur die Liste schließen, nicht den Dialog
			e.preventDefault();
			e.stopPropagation();
			zurueck();
		}
	}
	// Feld verlassen: wer genau einen Namen oder Code getippt hat, bekommt ihn, sonst gilt das Bisherige
	function verlassen() {
		if (!offen) return;
		const q = gesucht.trim().toLowerCase();
		const treffer = q && liste.find((s) => s.code === q || s.name.toLowerCase() === q);
		if (treffer) waehlen(treffer);
		else zurueck();
	}
</script>

<div class="sprachwahl">
	<label class="feldname" for={id}>{label}</label>
	<div class="feldrahmen">
		<input
			{id}
			bind:this={feld}
			class="feld"
			role="combobox"
			aria-autocomplete="list"
			aria-expanded={offen}
			aria-controls="{id}-liste"
			aria-activedescendant={offen && liste[aktiv] ? `${id}-${aktiv}` : undefined}
			bind:value={text}
			placeholder={none}
			{disabled}
			spellcheck="false"
			autocomplete="off"
			oninput={() => zeigen(text)}
			onfocus={() => feld?.select()}
			onclick={() => !offen && zeigen()}
			onkeydown={taste}
			onblur={verlassen}
		/>
		<button
			type="button"
			class="pfeil"
			tabindex="-1"
			aria-label={m.te_languages_show()}
			{disabled}
			onpointerdown={(e) => e.preventDefault()}
			onclick={() => {
				if (offen) zurueck();
				else {
					feld?.focus();
					zeigen();
				}
			}}><IconMaterialSymbolsKeyboardArrowDownRounded /></button
		>
	</div>
	{#if offen}
		<ul class="liste" id="{id}-liste" role="listbox" aria-label={label}>
			{#each liste as s, i (s.code + i)}
				<!-- Maus: pointerdown hält den Fokus im Feld; die Tastatur bleibt im Feld (aria-activedescendant) -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="{id}-{i}"
					role="option"
					aria-selected={i === aktiv}
					class:aktiv={i === aktiv}
					class:leer={!s.code}
					onpointerdown={(e) => e.preventDefault()}
					onpointermove={() => (aktiv = i)}
					onclick={() => waehlen(s)}
				>
					<span class="name">{'eigen' in s && s.eigen && s.name === s.code ? m.te_language_code({ code: s.code }) : s.name}</span>
					{#if s.code}<span class="code">{s.code}</span>{/if}
					{#if s.code && !s.neu}<span class="nur">{m.te_language_noted_only()}</span>{/if}
				</li>
			{:else}
				<li class="nichts" role="presentation">{m.te_language_none_found()}</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.sprachwahl {
		display: grid;
		gap: 0.3rem;
	}
	.feldrahmen {
		position: relative;
	}
	.feldrahmen .feld {
		width: 100%;
		padding-right: 2.5rem;
		text-overflow: ellipsis;
	}
	.pfeil {
		position: absolute;
		right: 0.25rem;
		top: 50%;
		translate: 0 -50%;
		width: 2.25rem;
		height: 2.25rem;
		display: grid;
		place-items: center;
		border: 0;
		border-radius: 50%;
		background: transparent;
		color: var(--ink);
		cursor: pointer;
	}
	.pfeil:hover {
		background: var(--desk);
	}
	.pfeil :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
		margin: 0;
	}
	/* die Vorschläge im Fluss unter dem Feld: der Dialog wächst mit, nichts wird abgeschnitten */
	.liste {
		list-style: none;
		margin: 0;
		padding: 0.25rem;
		max-height: 17rem;
		overflow-y: auto;
		border: 1px solid var(--rule-2);
		border-radius: 0.5rem;
		background: var(--paper);
	}
	li[role='option'] {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		column-gap: 0.75rem;
		min-height: var(--hit);
		padding: 0.375rem 0.75rem;
		border-radius: 0.375rem;
		cursor: pointer;
	}
	li.aktiv {
		background: var(--desk);
	}
	li.leer .name {
		color: var(--ink-2);
	}
	.name {
		font-weight: 600;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.code {
		font-size: 0.875rem;
		color: var(--ink-3);
		font-variant-numeric: tabular-nums;
	}
	.nur {
		grid-column: 1 / -1;
		font-size: 0.875rem;
		line-height: 1.35;
		color: var(--ink-3);
	}
	.nichts {
		padding: 0.5rem 0.75rem;
		font-size: 0.9375rem;
		line-height: 1.45;
		color: var(--ink-2);
	}
</style>
