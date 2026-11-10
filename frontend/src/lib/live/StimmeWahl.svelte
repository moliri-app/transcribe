<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- „Meine Stimme“ für Live und Diktieren: die Sprachen, eigene Wörter und angesagte Satzzeichen.
     Mit `kurz` nur die Sprachen, für „Einrichten“ auf der Seite Live; alles steht in den
     Einstellungen unter „Live und Diktieren“. -->
<script>
	import Sprachenwahl from '$lib/Sprachenwahl.svelte';
	import Toggle from '$lib/moliri/components/ui/Toggle.svelte';
	import MeineWoerter from './MeineWoerter.svelte';
	import { live } from './stand.svelte.js';

	/** @type {{ kurz?: boolean }} */
	let { kurz = false } = $props();
</script>

<div class="sprache">
	<Sprachenwahl bind:value={live.sprache} label="Sprachen" onchange={live.spracheWaehlen} />
	<span class="satz">
		{live.sprache === 'de'
			? 'Beim Sprechen zeigt das schnelle deutsche Modell Kroko den Text; jeden Satz schreibt danach das genauere Parakeet.'
			: live.sprache === 'auto'
				? 'Der Live-Text erkennt die Sprache selbst.'
				: live.sprache.includes(',')
					? 'Der Live-Text erkennt die Sprache selbst. Vollständig transkribiert bekommt jeder Abschnitt eine dieser Sprachen.'
					: 'Der Live-Text erkennt die Sprache selbst; die Wahl gilt fürs vollständige Transkribieren.'}
	</span>
</div>
{#if !kurz}
	<MeineWoerter />
	<Toggle
		bind:checked={() => live.ansagen, (v) => live.ansagenSetzen(v)}
		title="Satzzeichen ansagen"
		description={live.ansagen
			? 'Sag „Komma“, „Punkt“, „Fragezeichen“ oder „neuer Absatz“, und das Zeichen steht da. Sonst setzt die App sie selbst.'
			: 'Aus: Die App setzt alle Satzzeichen selbst, „Komma“ bleibt ein Wort.'}
	/>
{/if}

<style>
	.sprache {
		display: grid;
		gap: 0.3rem;
		max-width: 36rem;
	}
	.sprache :global(.sprachen) {
		width: 100%;
	}
	.satz {
		font-size: 0.9375rem;
		font-weight: 400;
		color: hsl(var(--color-text-secondary));
	}
</style>
