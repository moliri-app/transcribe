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
	<Sprachenwahl bind:value={live.sprache} label="Languages" onchange={live.spracheWaehlen} />
	<span class="satz">
		{live.sprache === 'de'
			? /* @wc-include */ 'While you speak, the fast German model Kroko shows the text; then the more accurate Parakeet writes each sentence.'
			: live.sprache === 'auto'
				? /* @wc-include */ 'Live text detects the language by itself.'
				: live.sprache.includes(',')
					? /* @wc-include */ 'Live text detects the language by itself. When fully transcribed, each section gets one of these languages.'
					: /* @wc-include */ 'Live text detects the language by itself; the choice applies to full transcription.'}
	</span>
</div>
{#if !kurz}
	<MeineWoerter />
	<Toggle
		bind:checked={() => live.ansagen, (v) => live.ansagenSetzen(v)}
		title="Dictate punctuation"
		description={live.ansagen
			? /* @wc-include */ 'Say “comma”, “period”, “question mark” or “new paragraph”, and the mark appears. Otherwise the app sets them itself.'
			: /* @wc-include */ 'Off: the app sets all punctuation itself, “comma” stays a word.'}
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
