<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Wie Live aufnimmt: Quelle (Mikrofon, Systemaudio, beides), welches Mikrofon und ob die Aufnahme
     gespeichert wird. Steht unter „Einrichten“ auf der Seite Live und in den Einstellungen unter
     „Live und Diktieren“; beide zeigen denselben Stand (stand.svelte.js). -->
<script>
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import { live } from './stand.svelte.js';

	/** das gewählte Mikrofon ist gerade nicht angeschlossen */
	const fehlt = $derived(!!live.geraet && !!live.geraete && !live.geraete.includes(live.geraet));

	const QUELLWAHL = /** @type {const} */ ([
		['mic', 'Mikrofon', 'Was am Mikrofon gesagt wird.'],
		['system', 'Systemaudio', 'Was der Rechner abspielt, etwa die anderen in einem Online-Meeting.'],
		['both', 'Beides gemischt', 'Am besten mit Kopfhörern, sonst sind die anderen doppelt zu hören.']
	]);
</script>

<fieldset class="quellen">
	<legend class="feldname">Was aufgenommen wird</legend>
	{#each QUELLWAHL as [wert, name, satz] (wert)}
		{@const aus = wert !== 'mic' && !live.systemMoeglich}
		<label class="quelle" class:an={live.quelle === wert} class:aus>
			<input type="radio" name="quelle" value={wert} disabled={aus} bind:group={live.quelle} onchange={live.quelleWaehlen} />
			<span>
				<b>{name}</b>
				<span class="satz">{aus ? 'Auf diesem Rechner nicht möglich.' : satz}</span>
			</span>
		</label>
	{/each}
</fieldset>

{#if live.quelle !== 'system'}
	<label class="feldname">
		Mikrofon
		<select class="feld" bind:value={live.geraet} onchange={live.geraetWaehlen} disabled={live.geraete === null}>
			<option value="">
				{live.geraete === null ? 'wird gesucht …' : live.standard ? `Systemstandard: ${live.standard}` : 'Systemstandard'}
			</option>
			{#if fehlt}
				<option value={live.geraet}>{live.geraet} (nicht angeschlossen)</option>
			{/if}
			{#each live.geraete ?? [] as g (g)}<option value={g}>{g}</option>{/each}
		</select>
		<span class="satz">
			{#if fehlt}
				Dieses Mikrofon ist gerade nicht angeschlossen.
			{:else if live.geraet}
				Bleibt bei diesem Mikrofon, auch wenn ein anderes angeschlossen wird.
			{:else if live.standard}
				Nimmt das Mikrofon, das in Windows als Standard eingestellt ist. Wird ein anderes zum Standard, etwa ein
				angeschlossenes Headset, wechselt die Aufnahme mit.
			{:else}
				Nimmt das Mikrofon, das im System als Standard eingestellt ist.
			{/if}
		</span>
	</label>
{/if}

<Toggle
	bind:checked={() => live.speichern, (v) => live.speichernSetzen(v)}
	disabled={!!live.laufend}
	title="Aufnahme speichern"
	description={live.speichern
		? 'Sie kommt zu deinen Aufnahmen und lässt sich danach vollständig transkribieren, mit Sprechern.'
		: 'Aus: Es bleibt nur der Text.'}
/>

<style>
	.quellen {
		display: grid;
		gap: 0.375rem;
		max-width: 36rem;
		border: 0;
		margin: 0;
		padding: 0;
	}
	.quellen legend {
		margin-bottom: 0.375rem;
	}
	.quelle {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 0.625rem 0.75rem;
		border: 1px solid hsl(var(--color-border-primary));
		border-radius: 0.75rem;
		cursor: pointer;
		transition:
			background-color 0.15s,
			border-color 0.15s;
	}
	.quelle:hover:not(.aus) {
		border-color: hsl(var(--color-text-secondary));
	}
	.quelle.an {
		border-color: hsl(var(--color-neutral-100));
		box-shadow: inset 0 0 0 1px hsl(var(--color-neutral-100));
	}
	.quelle.aus {
		cursor: default;
		opacity: 0.55;
	}
	.quelle input {
		margin-top: 0.1875rem;
	}
	.quelle > span {
		display: grid;
	}
	.feldname {
		max-width: 36rem;
	}
	.feldname select {
		width: 100%;
		color: hsl(var(--color-text-primary));
		font-weight: 400;
	}
	.satz {
		font-size: 0.9375rem;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
</style>
