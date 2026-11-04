<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Die Optionen eines neuen Auftrags: in der Bibliothek und im Dialog nach dem Ablegen von
     Dateien. Startwerte sind die Vorgaben aus den Einstellungen. Die Einstellungen bearbeiten mit
     denselben Feldern die Vorgaben selbst und speichern bei jeder Änderung (onchange).
     Zu wählen sind nur Sprecher und Sprachen: Eine Voreinstellung legte früher nur die Zahl der
     Sprecher fest, und die Zahl der Threads wählt die App selbst. Ist in den Einstellungen beides
     hinterlegt, lokal und extern, und tragen die Optionen eine engine, lässt sich auch wählen, was
     erkennt. -->
<script>
	import { ENGINES } from '$lib/api.js';
	import Sprachenwahl from '$lib/Sprachenwahl.svelte';

	/** @type {{ optionen: import('$lib/api.js').Optionen, onchange?: () => void, erkennung?: string }} */
	let { optionen = $bindable(), onchange, erkennung = 'local' } = $props();

	const waehlbar = $derived(erkennung === 'both' && optionen.engine !== undefined);

	/** die Zahl als Text, wie das Auswahlfeld sie führt; '' heißt automatisch */
	const SPRECHER = [
		['', 'Automatisch erkennen'],
		['1', '1, etwa Vortrag, Lesung oder Führung'],
		['2', '2, etwa ein Interview'],
		...Array.from({ length: 10 }, (_, i) => [String(i + 3), String(i + 3)])
	];
</script>

<!-- change steigt von den Feldern auf; die Bindungen haben den neuen Wert da schon übernommen -->
<div class="optionen" onchange={() => onchange?.()}>
	<label class="feldname">
		Sprecher
		<select
			class="feld sprecher"
			bind:value={() => String(optionen.speakers ?? ''), (v) => (optionen.speakers = v)}
		>
			{#each SPRECHER as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
		</select>
	</label>
	<Sprachenwahl bind:value={optionen.language} label="Sprachen" onchange={() => onchange?.()} />
	{#if waehlbar}
		<label class="feldname">
			Erkennung
			<select class="feld" bind:value={optionen.engine}>
				{#each Object.entries(ENGINES) as [wert, name] (wert)}<option value={wert}>{name}</option>{/each}
			</select>
		</label>
	{/if}
</div>

<style>
	.optionen {
		display: flex;
		flex-wrap: wrap;
		/* mehrere Sprachen machen ihr Feld höher, die anderen Felder bleiben, wie sie sind */
		align-items: flex-start;
		gap: 1rem 1.25rem;
	}
	.sprecher {
		min-width: 17rem;
	}
</style>
