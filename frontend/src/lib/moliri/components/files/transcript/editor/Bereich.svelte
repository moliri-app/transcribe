<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Bereich.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Bereich.svelte). -->
<!-- Dialog an einem markierten Bereich (Auswahl im Text, auch über Sprecher und Absätze hinweg, oder
     Ziehen in der Zeitleiste): Zeit und Wortlaut, abspielen oder in Schleife hören, als Clip festhalten, den Sprecher für
     den Bereich festlegen, ebenso seine Sprache; in einer Pause ohne Wörter Text einfügen. Festlegen, Einfügen
     und Clips nur für die, die korrigieren dürfen. Er nimmt der Schreibmarke den
     Fokus nicht: wer die Auswahl einfach übertippt, ersetzt sie. Schließt mit Esc, einem Klick daneben
     oder nach dem Übernehmen. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { getLocale } from '$lib/moliri/locale.js';
	import { fmt, rangeText } from './model.js';
	import Sprachwahl from './Sprachwahl.svelte';
	import { langName, recognizable } from './sprachen.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLDivElement | undefined} */ let box = $state();
	let pos = $state({ left: 0, top: 0 });
	const r = $derived(ed.range);
	// jeder neue Bereich beginnt unverändert
	let speaker = $derived(r && '');
	let language = $derived(r && '');
	let einfuegen = $derived(r && '');
	const zitat = $derived(r ? rangeText(ed.words, r.a, r.b) : '');
	const looping = $derived(!!r && !!ed.loop && Math.abs(ed.loop.a - r.a) < 0.01 && Math.abs(ed.loop.b - r.b) < 0.01);
	const dauer = $derived(r ? (r.b - r.a).toLocaleString(getLocale(), { minimumFractionDigits: 1, maximumFractionDigits: 1 }) + ' s' : '');

	// neben der Maus, unter ihr, wenn Platz ist, sonst darüber (gemessen, daher im Effekt)
	$effect(() => {
		if (!r || !box) return;
		const w = box.offsetWidth,
			h = box.offsetHeight;
		const below = r.y + 16 + h < innerHeight - 12;
		pos = {
			left: Math.max(12, Math.min(r.x - 24, innerWidth - w - 12)),
			top: below ? r.y + 16 : Math.max(12, r.y - h - 16)
		};
	});

	function close() {
		ed.range = null;
		ed.sel = null;
	}
	function apply() {
		if (!r) return;
		ed.setSpan(r.a, r.b, { speaker, language });
		close();
	}
	function insert() {
		if (!r) return;
		ed.insertText(r.a, r.b, einfuegen);
		close();
	}
	function outside(/** @type {PointerEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		if (r && box && !box.contains(t) && !t.closest('.lanes')) close();
	}
</script>

<svelte:window onpointerdown={outside} onkeydown={(e) => r && e.key === 'Escape' && close()} />

{#if r}
	<div class="bereich" role="dialog" aria-label={m.te_range_dialog({ from: fmt(r.a), to: fmt(r.b) })} bind:this={box} style:left="{pos.left}px" style:top="{pos.top}px">
		<div class="kopf">
			<b class="zeit">{fmt(r.a)} – {fmt(r.b)}</b>
			<span class="dauer">{dauer}</span>
		</div>
		{#if zitat}
			<p class="zitat">{zitat}</p>
		{/if}
		<div class="reihe">
			<button class="pille" onclick={() => ed.playRange(r.a, r.b)}><IconMaterialSymbolsPlayArrowRounded />{m.te_play()}</button>
			<button
				class="pille"
				aria-pressed={looping}
				title={m.te_loop_hint()}
				onclick={() => {
					if (looping) {
						ed.loop = null;
						ed.media?.pause();
					} else ed.playRange(r.a, r.b, true);
				}}><IconMaterialSymbolsRepeatRounded />{m.te_loop()}</button
			>
			{#if ed.canEdit}<button
				class="pille"
				title={m.te_as_clip_hint()}
				onclick={() => {
					const { a, b, x, y } = r;
					ed.range = null;
					ed.sel = { a, b };
					ed.clipDlg = { kind: 'clip', clip: { start: a, end: b }, x, y };
				}}><IconMaterialSymbolsContentCutRounded />{m.te_as_clip()}</button
			>{/if}
		</div>

		{#if !ed.canEdit}
			<!-- nur lesen -->
		{:else if zitat}
			<form
				class="festlegen"
				onsubmit={(e) => {
					e.preventDefault();
					apply();
				}}
			>
				<div class="felder">
					<label class="feldname">
						{m.transcript_speakers()}
						<select class="feld" bind:value={speaker}>
							<option value="">{m.te_unchanged()}</option>
							{#each ed.speakers as k (k)}<option value={String(k)}>{ed.spkName(k)}</option>{/each}
							<option value="neu">{m.transcript_speaker_new()}</option>
						</select>
					</label>
					{#key r}<Sprachwahl bind:value={language} none={m.te_unchanged()} />{/key}
				</div>
				<p class="hinweis">
					{m.te_range_note()}{#if language && recognizable(language)}{' '}{ed.recomputes ? m.te_language_redone_recompute({ language: langName(language) }) : m.te_language_redone({ language: langName(language) })}{:else if language}{' '}{m.te_language_noted({ language: langName(language) })}{/if}
				</p>
				<div class="fuss">
					<button type="button" class="pille" onclick={close}>{m.close()}</button>
					<button type="submit" class="pille rot" disabled={!speaker && !language}>{m.te_apply()}</button>
				</div>
			</form>
		{:else}
			<form
				class="festlegen"
				onsubmit={(e) => {
					e.preventDefault();
					insert();
				}}
			>
				<label>
					<span>{m.te_gap_question()}</span>
					<input bind:value={einfuegen} placeholder={m.te_gap_placeholder({ time: fmt(r.a) })} spellcheck="false" />
				</label>
				<div class="fuss">
					<button type="button" class="pille" onclick={close}>{m.close()}</button>
					<button type="submit" class="pille rot" disabled={!einfuegen.trim()}>{m.te_insert()}</button>
				</div>
			</form>
		{/if}
	</div>
{/if}

<style>
	.bereich {
		position: fixed;
		z-index: 45;
		width: 22rem;
		max-width: calc(100vw - 1.5rem);
		display: grid;
		gap: 0.75rem;
		padding: 0.75rem 0.875rem 0.875rem 1rem;
		background: var(--paper);
		color: var(--ink);
		border-radius: 1rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	.kopf {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.zeit {
		font-size: 1.0625rem;
		font-variant-numeric: tabular-nums;
	}
	.dauer {
		margin-left: auto;
		font-size: 0.875rem;
		color: var(--ink-3);
		font-variant-numeric: tabular-nums;
	}
	/* der Wortlaut, wie er im Text steht: höchstens drei Zeilen */
	.zitat {
		margin: 0;
		padding: 0.5rem 0.75rem;
		border-radius: 0.5rem;
		background: var(--desk);
		font-size: 1rem;
		line-height: 1.5;
		color: var(--ink);
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.reihe,
	.fuss {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.fuss {
		justify-content: flex-end;
	}
	.pille {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 1rem;
		border: 1px solid var(--rule-2);
		border-radius: 2rem;
		background: var(--paper);
		color: var(--ink);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.reihe .pille {
		padding-left: 0.625rem;
	}
	.pille:hover:not(:disabled) {
		background: var(--desk);
		border-color: var(--ink-3);
	}
	.pille[aria-pressed='true'] {
		background: var(--head);
		border-color: var(--head);
		color: var(--signal);
	}
	.pille :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
		margin: 0;
	}
	/* Übernehmen ist die Hand des Menschen: Rotstift */
	.pille.rot {
		background: var(--red);
		border-color: var(--red);
		color: hsl(var(--color-neutral-0));
	}
	.pille.rot:hover:not(:disabled) {
		background: hsl(var(--color-red-200) / 0.88);
		border-color: transparent;
	}
	.pille:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.festlegen {
		display: grid;
		gap: 0.5rem;
		padding-top: 0.75rem;
		border-top: 1px solid var(--rule);
	}
	label {
		display: grid;
		gap: 0.25rem;
	}
	label span {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--ink-2);
	}
	input {
		height: var(--hit);
		padding: 0 0.75rem;
		border: 1px solid var(--rule-2);
		border-radius: 0.5rem;
		background: var(--paper);
		font: inherit;
		/* was hier getippt wird, ist die Hand des Menschen */
		color: var(--red);
	}
	input:hover {
		border-color: var(--ink-3);
	}
	/* Sprecher und Sprache untereinander: die Vorschläge der Sprache brauchen die volle Breite */
	.felder {
		display: grid;
		gap: 0.625rem;
	}
	.hinweis {
		margin: 0;
		font-size: 0.875rem;
		line-height: 1.45;
		color: var(--ink-3);
	}
</style>
