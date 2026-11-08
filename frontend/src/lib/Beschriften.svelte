<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Eine Aufnahme benennen und verschlagworten, in der Bibliothek und im Editor. Name und
     Schlagworte gehören Transcribe (library.json, internal/shelf), die Datei behält ihren Namen; so
     geht es auch in verbundenen Ordnern. Schlagworte wie bei den Clips: mit Komma getrennt, die
     schon vergebenen als Pillen zum Antippen. Unten links „Ausblenden“, das die Aufnahme aus den
     Listen nimmt, ohne etwas zu löschen. open(a) öffnet, ondone(a, stand) meldet, was gespeichert ist. -->
<script>
	import Button from '$lib/moliri/components/ui/Button.svelte';
	import { beschriften } from '$lib/api.js';

	/** @typedef {{ stem: string, title: string, file: string, tags: string[], hidden: boolean }} Beschriftbar */
	/** @typedef {{ title: string, tags: string[], hidden: boolean }} Stand */

	/** vorhanden: die Schlagworte der ganzen Bibliothek, zum Antippen
	 * @type {{ vorhanden?: string[], ondone: (a: Beschriftbar, stand: Stand) => void }} */
	let { vorhanden = [], ondone } = $props();

	/** @type {HTMLDialogElement} */
	let dlg;
	/** @type {HTMLInputElement | undefined} */
	let namenfeld = $state();
	/** @type {Beschriftbar | null} */
	let a = $state(null);
	let name = $state('');
	let tags = $state('');
	let speichert = $state(false);
	let fehler = $state('');

	const tagList = $derived([...new Set(tags.split(',').map((t) => t.trim()).filter(Boolean))]);
	/** was angetippt werden kann: die vorhandenen und die dieser Aufnahme */
	const marken = $derived([...new Set([...(a?.tags ?? []), ...vorhanden])].sort((x, y) => x.localeCompare(y, 'de')));

	/** @param {Beschriftbar} aufnahme */
	export function open(aufnahme) {
		a = aufnahme;
		name = aufnahme.title === aufnahme.file ? '' : aufnahme.title;
		tags = aufnahme.tags.join(', ');
		fehler = '';
		dlg.showModal();
		namenfeld?.focus();
	}

	/** @param {string} t */
	function toggle(t) {
		const has = tagList.some((k) => k.toLowerCase() === t.toLowerCase());
		tags = (has ? tagList.filter((k) => k.toLowerCase() !== t.toLowerCase()) : [...tagList, t]).join(', ');
	}

	/** @param {{ title?: string, tags?: string[], hidden?: boolean }} was */
	async function speichern(was) {
		if (!a) return;
		speichert = true;
		fehler = '';
		try {
			const stand = await beschriften(a.stem, was);
			ondone(a, stand);
			dlg.close();
		} catch (e) {
			fehler = `Das ließ sich nicht speichern: ${/** @type {Error} */ (e).message}`;
		} finally {
			speichert = false;
		}
	}
</script>

<dialog class="beschriften" bind:this={dlg} aria-labelledby="h-beschriften" onclose={() => (a = null)}>
	{#if a}
		<form
			method="dialog"
			onsubmit={(e) => {
				e.preventDefault();
				speichern({ title: name, tags: tagList });
			}}
		>
			<h2 id="h-beschriften">Name und Schlagworte</h2>
			<label class="feldname">
				Name
				<input class="feld" bind:this={namenfeld} bind:value={name} placeholder={a.file} autocomplete="off" maxlength="200" />
			</label>
			<p class="hinweis">Nur in Transcribe. Die Datei heißt weiter „{a.file}“.</p>
			<label class="feldname">
				Schlagworte
				<input class="feld" bind:value={tags} placeholder="mit Komma trennen, etwa Interview, Papierfabrik" autocomplete="off" />
			</label>
			{#if marken.length}
				<div class="marken" role="group" aria-label="Vergebene Schlagworte">
					{#each marken as t (t)}
						<button type="button" class="marke" aria-pressed={tagList.some((k) => k.toLowerCase() === t.toLowerCase())} onclick={() => toggle(t)}
							>{t}</button
						>
					{/each}
				</div>
			{/if}
			{#if fehler}<p class="fehler" role="alert">{fehler}</p>{/if}
			<div class="fuss">
				<button type="button" class="leise" disabled={speichert} onclick={() => speichern({ hidden: !a?.hidden })}>
					{#if a.hidden}<IconMaterialSymbolsVisibilityOutlineRounded aria-hidden="true" />Wieder einblenden{:else}<IconMaterialSymbolsVisibilityOffOutlineRounded
							aria-hidden="true"
						/>Ausblenden{/if}
				</button>
				<span class="platz"></span>
				<Button type="secondary" size="large" onclick={() => dlg.close()}>Abbrechen</Button>
				<Button type="primary" size="large" nativeType="submit" loading={speichert}>Speichern</Button>
			</div>
		</form>
	{/if}
</dialog>

<style>
	.beschriften {
		width: min(34rem, calc(100vw - 2rem));
		margin: auto;
		padding: 1.5rem 1.75rem;
		border: 0;
		border-radius: 0.75rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-primary));
		box-shadow: 0 24px 64px -24px hsl(var(--color-neutral-100) / 0.5);
	}
	.beschriften::backdrop {
		background: hsl(var(--color-neutral-100) / 0.3);
		backdrop-filter: blur(4px);
	}
	form {
		display: grid;
		gap: 0.625rem;
	}
	h2 {
		margin-bottom: 0.5rem;
		font-size: 1.375rem;
		letter-spacing: -0.01em;
	}
	.feldname {
		display: grid;
		gap: 0.375rem;
	}
	.feld {
		width: 100%;
	}
	.hinweis {
		margin: -0.25rem 0 0.5rem;
		font-size: 0.9375rem;
		color: hsl(var(--color-text-secondary));
		overflow-wrap: anywhere;
	}
	.marken {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	/* wie die Schlagworte der Clips */
	.marke {
		min-height: 2.25rem;
		padding: 0 0.875rem;
		border: 1px solid hsl(190 12% 80%);
		border-radius: 2rem;
		background: hsl(var(--color-bg-primary));
		color: hsl(var(--color-text-secondary));
		font-size: 0.9375rem;
		cursor: pointer;
	}
	.marke:hover {
		border-color: hsl(var(--color-text-tertiary));
		color: hsl(var(--color-text-primary));
	}
	.marke[aria-pressed='true'] {
		background: hsl(var(--color-neutral-100));
		border-color: hsl(var(--color-neutral-100));
		color: hsl(var(--color-neutral-0));
	}
	.fehler {
		color: hsl(var(--color-text-error));
	}
	.fuss {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		margin-top: 1rem;
	}
	.platz {
		flex: 1;
	}
	.leise {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-height: var(--hit);
		padding: 0 0.75rem;
		margin-left: -0.75rem;
		border-radius: 2rem;
		font-weight: 600;
		color: hsl(var(--color-text-secondary));
		cursor: pointer;
	}
	.leise:hover:not(:disabled) {
		background: hsl(var(--color-bg-secondary));
		color: hsl(var(--color-text-primary));
	}
	.leise :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
	}
</style>
