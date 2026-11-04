<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Die Sprachen einer Aufnahme: keine (automatisch), eine oder mehrere. Gewählte Sprachen stehen als
     Pillen im Feld, dahinter sucht man die nächste wie in der Sprachwahl des Editors („Engl“, „en“).
     Angeboten wird nur, was Whisper kennt: Bei mehreren wählt die Erkennung je Abschnitt nur unter
     ihnen. Pfeiltasten wählen, Enter nimmt dazu, Rückschritt im leeren Feld nimmt die letzte weg, Esc
     schließt die Liste. value ist „auto“ oder die Codes mit Komma, etwa „de,en“. -->
<script>
	import { langName, vorschlaege } from '$lib/moliri/components/files/transcript/editor/sprachen.js';

	/** @type {{ value: string, label?: string, onchange?: () => void }} */
	let { value = $bindable('auto'), label = 'Sprache', onchange } = $props();

	const id = 'sp' + Math.random().toString(36).slice(2, 8);
	const codes = $derived(!value || value === 'auto' ? [] : value.split(','));

	/** @type {HTMLInputElement | undefined} */ let feld = $state();
	let text = $state('');
	let offen = $state(false);
	let aktiv = $state(0);

	const liste = $derived(vorschlaege(text, 40).filter((s) => s.neu && !codes.includes(s.code)).slice(0, 8));

	function setzen(/** @type {string[]} */ c) {
		value = c.length ? c.join(',') : 'auto';
		onchange?.();
	}
	function dazu(/** @type {{ code: string }} */ s) {
		setzen([...codes, s.code]);
		text = '';
		offen = false;
		feld?.focus();
	}
	function weg(/** @type {string} */ c) {
		setzen(codes.filter((x) => x !== c));
		feld?.focus();
	}
	function zeigen() {
		aktiv = 0;
		offen = true;
	}
	function taste(/** @type {KeyboardEvent} */ e) {
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			if (!offen) return zeigen();
			if (!liste.length) return;
			aktiv = (aktiv + (e.key === 'ArrowDown' ? 1 : -1) + liste.length) % liste.length;
			document.getElementById(`${id}-${aktiv}`)?.scrollIntoView({ block: 'nearest' });
		} else if (e.key === 'Enter' && offen) {
			// Enter nimmt die Sprache dazu und schickt kein Formular ab
			e.preventDefault();
			if (liste[aktiv]) dazu(liste[aktiv]);
		} else if (e.key === 'Backspace' && !text && codes.length) {
			setzen(codes.slice(0, -1));
		} else if (e.key === 'Escape' && offen) {
			// nur die Liste schließen, nicht das Fenster darum
			e.preventDefault();
			e.stopPropagation();
			offen = false;
		}
	}
	// Feld verlassen: genau ein getippter Name oder Code kommt dazu, sonst bleibt alles, wie es war
	function verlassen() {
		const q = text.trim().toLowerCase();
		const treffer = q && liste.find((s) => s.code === q || s.name.toLowerCase() === q);
		if (treffer) setzen([...codes, treffer.code]);
		text = '';
		offen = false;
	}
</script>

<div class="sprachen">
	<label class="feldname" for={id}>{label}</label>
	<!-- ein Klick ins Feld, auch neben die Pillen, geht in die Suche -->
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="rahmen" onclick={() => feld?.focus()}>
		{#each codes as c (c)}
			<span class="gewaehlt">
				{langName(c)}
				<button
					type="button"
					class="weg"
					aria-label="{langName(c)} entfernen"
					onpointerdown={(e) => e.preventDefault()}
					onclick={(e) => {
						e.stopPropagation();
						weg(c);
					}}><IconMaterialSymbolsCloseRounded aria-hidden="true" /></button
				>
			</span>
		{/each}
		<input
			{id}
			bind:this={feld}
			role="combobox"
			aria-autocomplete="list"
			aria-expanded={offen}
			aria-controls="{id}-liste"
			aria-activedescendant={offen && liste[aktiv] ? `${id}-${aktiv}` : undefined}
			bind:value={text}
			placeholder={codes.length ? 'weitere …' : 'Automatisch'}
			spellcheck="false"
			autocomplete="off"
			oninput={zeigen}
			onclick={() => !offen && zeigen()}
			onkeydown={taste}
			onblur={verlassen}
			onchange={(e) => e.stopPropagation()}
		/>
	</div>
	{#if offen}
		<ul class="liste" id="{id}-liste" role="listbox" aria-label={label}>
			{#each liste as s, i (s.code)}
				<!-- Maus: pointerdown hält den Fokus im Feld; die Tastatur bleibt im Feld (aria-activedescendant) -->
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					id="{id}-{i}"
					role="option"
					aria-selected={i === aktiv}
					class:aktiv={i === aktiv}
					onpointerdown={(e) => e.preventDefault()}
					onpointermove={() => (aktiv = i)}
					onclick={() => dazu(s)}
				>
					<span class="name">{s.name}</span>
					<span class="code">{s.code}</span>
				</li>
			{:else}
				<li class="nichts" role="presentation">Keine Sprache gefunden, die die Erkennung kennt.</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.sprachen {
		position: relative;
		display: grid;
		gap: 0.3rem;
		/* fest breit: weitere Sprachen brechen im Feld um, die Felder daneben bleiben stehen */
		width: min(100%, 21rem);
	}
	/* sieht aus wie ein Feld (.feld) und trägt die gewählten Sprachen als Pillen */
	.rahmen {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0.3rem 0.5rem;
		border: 1px solid hsl(var(--color-border-secondary));
		border-radius: 0.5rem;
		background-color: hsl(var(--color-bg-primary));
		cursor: text;
		transition: border-color 0.15s;
	}
	.rahmen:hover {
		border-color: hsl(var(--color-text-secondary));
	}
	.rahmen:focus-within {
		outline: 2px solid hsl(var(--color-border-focus));
		outline-offset: 2px;
	}
	.gewaehlt {
		display: inline-flex;
		align-items: center;
		gap: 0.125rem;
		height: 2rem;
		padding: 0 0.25rem 0 0.75rem;
		border-radius: 999px;
		background: hsl(var(--color-bg-inverse));
		color: hsl(var(--color-text-inverse));
		font-size: 0.9375rem;
		font-weight: 600;
		white-space: nowrap;
	}
	.weg {
		display: grid;
		place-items: center;
		width: 1.5rem;
		height: 1.5rem;
		border: 0;
		border-radius: 50%;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}
	.weg:hover {
		background: hsl(var(--color-text-inverse) / 0.2);
	}
	.weg :global(svg) {
		width: 1rem;
		height: 1rem;
		margin: 0;
	}
	input {
		flex: 1 1 6rem;
		min-width: 6rem;
		height: 2rem;
		border: 0;
		outline: 0;
		background: transparent;
		color: hsl(var(--color-text-primary));
		font: inherit;
		font-size: 1rem;
		font-weight: 400;
	}
	input::placeholder {
		color: hsl(var(--color-text-tertiary));
	}
	/* die Vorschläge schweben unter dem Feld, damit die Felder daneben stehen bleiben */
	.liste {
		position: absolute;
		top: 100%;
		left: 0;
		right: 0;
		z-index: 20;
		list-style: none;
		margin: 0.25rem 0 0;
		padding: 0.25rem;
		max-height: 17rem;
		overflow-y: auto;
		border: 1px solid hsl(var(--color-border-secondary));
		border-radius: 0.5rem;
		background: hsl(var(--color-bg-primary));
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	li[role='option'] {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		column-gap: 0.75rem;
		min-height: var(--hit);
		padding: 0.375rem 0.75rem;
		border-radius: 0.375rem;
		color: hsl(var(--color-text-primary));
		cursor: pointer;
	}
	li.aktiv {
		background: hsl(var(--color-bg-secondary));
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
		font-weight: 400;
		color: hsl(var(--color-text-tertiary));
		font-variant-numeric: tabular-nums;
	}
	.nichts {
		padding: 0.5rem 0.75rem;
		font-size: 0.9375rem;
		font-weight: 400;
		line-height: 1.45;
		color: hsl(var(--color-text-secondary));
	}
</style>
