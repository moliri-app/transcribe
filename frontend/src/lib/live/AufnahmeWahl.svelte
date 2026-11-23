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
		{ wert: 'mic', name: /* @wc-include */ 'Microphone', satz: /* @wc-include */ 'What is said into the microphone.' },
		{
			wert: 'system',
			name: /* @wc-include */ 'System audio',
			satz: /* @wc-include */ 'What the computer plays, such as the others in an online meeting.'
		},
		{ wert: 'both', name: /* @wc-include */ 'Both mixed', satz: /* @wc-include */ 'Best with headphones, otherwise the others are heard twice.' }
	]);
</script>

<fieldset class="quellen">
	<legend class="feldname">What gets recorded</legend>
	{#each QUELLWAHL as { wert, name, satz } (wert)}
		{@const aus = wert !== 'mic' && !live.systemMoeglich}
		<label class="quelle" class:an={live.quelle === wert} class:aus>
			<input type="radio" name="quelle" value={wert} disabled={aus} bind:group={live.quelle} onchange={live.quelleWaehlen} />
			<span>
				<b>{name}</b>
				<span class="satz">{aus ? /* @wc-include */ 'Not possible on this computer.' : satz}</span>
			</span>
		</label>
	{/each}
</fieldset>

{#if live.quelle !== 'system'}
	<label class="feldname">
		Microphone
		<select class="feld" bind:value={live.geraet} onchange={live.geraetWaehlen} disabled={live.geraete === null}>
			<option value="">
				{live.geraete === null
					? /* @wc-include */ 'searching…'
					: live.standard
						? /* @wc-include */ `System default: ${live.standard}`
						: /* @wc-include */ 'System default'}
			</option>
			{#if fehlt}
				<option value={live.geraet}>{/* @wc-include */ `${live.geraet} (not connected)`}</option>
			{/if}
			{#each live.geraete ?? [] as g (g)}<option value={g}>{g}</option>{/each}
		</select>
		<span class="satz">
			{#if fehlt}
				This microphone is not connected right now.
			{:else if live.geraet}
				Stays with this microphone, even when another one is connected.
			{:else if live.standard}
				Uses the microphone set as default in Windows. If another one becomes the default, such as a newly
				connected headset, the recording switches along.
			{:else}
				Uses the microphone set as default in the system.
			{/if}
		</span>
	</label>
{/if}

<Toggle
	bind:checked={() => live.speichern, (v) => live.speichernSetzen(v)}
	disabled={!!live.laufend}
	title="Save recording"
	description={live.speichern
		? /* @wc-include */ 'It goes to your recordings and can be fully transcribed afterwards, with speakers.'
		: /* @wc-include */ 'Off: only the text is kept.'}
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
