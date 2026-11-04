<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Clip.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Clip.svelte). -->
<!-- Dialog für Clips, an der Stelle, an der er geöffnet wurde: einen neuen Clip anlegen (aus einem
     markierten Bereich) oder einen bestehenden ändern, mit Titel, Schlagworten, Anfang und Ende; ein
     bestehender lässt sich von hier auch abspielen, exportieren und löschen. Als Export bietet er Audio,
     bei Videos Video, und Text mit Titel und Schlagworten an, für einen Clip oder einen
     Zusammenschnitt mehrerer. Schließt mit Esc, einem Klick daneben oder nach der Wahl. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { untrack } from 'svelte';
	import { fmt, fmtDur, fmtT, parseT, rangeText, defaultTitle } from './model.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLDivElement | undefined} */ let box = $state();
	/** @type {HTMLInputElement | undefined} */ let titleField = $state();
	let pos = $state({ left: 0, top: 0 });

	const d = $derived(ed.clipDlg);
	const c = $derived(d?.kind === 'clip' ? d.clip : null);
	const isNew = $derived(!!c && !c.id);
	const zitat = $derived(c ? rangeText(ed.words, c.start, c.end) : '');

	// jedes Öffnen beginnt mit dem, was am Clip steht
	let title = $derived(c?.title ?? '');
	let tags = $derived(c ? (c.tags ?? []).join(', ') : '');
	let von = $derived(c ? fmtT(c.start) : '');
	let bis = $derived(c ? fmtT(Math.ceil(c.end * 10) / 10) : '');
	let fehler = $derived(c && '');
	const tagList = $derived([...new Set(tags.split(',').map((t) => t.trim()).filter(Boolean))]);

	const list = $derived(d?.kind === 'export' ? d.list : []);
	const many = $derived(list.length > 1);
	const total = $derived(list.reduce((t, k) => t + k.end - k.start, 0));

	// neben der Maus, darunter, wenn Platz ist, sonst darüber; wächst er (Fehler), rückt er nach oben
	$effect(() => {
		if (!d || !box) return;
		const w = box.offsetWidth,
			h = box.offsetHeight;
		const below = d.y + 16 + h < innerHeight - 12;
		pos = {
			left: Math.max(12, Math.min(d.x - 24, innerWidth - w - 12)),
			top: below ? d.y + 16 : Math.max(72, d.y - h - 16)
		};
		untrack(() => titleField?.focus());
		const el = box;
		const ro = new ResizeObserver(() => {
			const over = pos.top + el.offsetHeight - (innerHeight - 12);
			if (over > 0) pos = { ...pos, top: Math.max(72, pos.top - over) };
		});
		ro.observe(el);
		return () => ro.disconnect();
	});

	function close() {
		if (isNew) ed.sel = null;
		ed.clipDlg = null;
	}
	function toggleTag(/** @type {string} */ t) {
		tags = (tagList.includes(t) ? tagList.filter((k) => k !== t) : [...tagList, t]).join(', ');
	}
	function save() {
		if (!c) return;
		const a = parseT(von),
			b = Math.min(parseT(bis), ed.duration());
		if (!(a >= 0 && b > a)) {
			fehler = m.te_clip_times_error();
			return;
		}
		ed.saveClip(c, { start: a, end: b, title, tags: tagList });
		ed.clipDlg = null;
	}
	function outside(/** @type {PointerEvent} */ e) {
		const t = /** @type {HTMLElement} */ (e.target);
		if (d && box && !box.contains(t) && !t.closest('[data-clip-oeffnet]')) close();
	}
</script>

<svelte:window onpointerdown={outside} onkeydown={(e) => d && e.key === 'Escape' && close()} />

{#if d}
	{#key d}
		<div
			class="clipdlg"
			role="dialog"
			aria-label={c ? (isNew ? m.te_clip_new() : m.te_clip_named({ title: c.title ?? '' })) : many ? m.te_clips_export_many({ count: list.length }) : m.te_clip_export_named({ title: list[0]?.title ?? '' })}
			bind:this={box}
			style:left="{pos.left}px"
			style:top="{pos.top}px"
		>
			{#if c}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						save();
					}}
				>
					<div class="kopf">
						<IconMaterialSymbolsContentCutRounded />
						<b>{isNew ? m.te_clip_new() : m.te_clip_default()}</b>
						<span class="dauer">{fmt(c.start)} – {fmt(c.end)} · {fmtDur(c.end - c.start)}</span>
					</div>
					<p class="zitat" class:leer={!zitat}>{zitat || m.te_clip_no_words()}</p>
					{#if !isNew}
						{@const clip = /** @type {import('./model.js').Clip} */ (c)}
						<div class="reihe">
							<button
								type="button"
								class="pille"
								onclick={() => {
									const k = clip;
									close();
									ed.showClip(k, true);
								}}><IconMaterialSymbolsPlayArrowRounded />{m.te_play()}</button
							>
							<button type="button" class="pille" onclick={() => d && (ed.clipDlg = { kind: 'export', list: [clip], x: d.x, y: d.y })}
								><IconMaterialSymbolsDownloadRounded />{m.transcript_export()}</button
							>
						</div>
					{/if}
					<div class="felder">
						<label class="feldname">
							{m.te_clip_title()}
							<input class="feld titel" bind:this={titleField} bind:value={title} placeholder={defaultTitle(zitat)} autocomplete="off" />
						</label>
						<label class="feldname">
							{m.te_clip_tags()}
							<input class="feld" bind:value={tags} placeholder={m.te_clip_tags_placeholder()} autocomplete="off" />
						</label>
						{#if ed.allTags.length}
							<div class="marken" role="group" aria-label={m.te_clip_tag_suggestions()}>
								{#each ed.allTags as t (t)}
									<button type="button" class="marke" aria-pressed={tagList.includes(t)} onclick={() => toggleTag(t)}>{t}</button>
								{/each}
							</div>
						{/if}
						<div class="zeiten">
							<label class="feldname">
								{m.te_clip_start()}
								<input class="feld zahl" bind:value={von} inputmode="decimal" aria-invalid={!!fehler && !(parseT(von) >= 0)} autocomplete="off" />
							</label>
							<label class="feldname">
								{m.te_clip_end()}
								<input class="feld zahl" bind:value={bis} inputmode="decimal" aria-invalid={!!fehler && !(parseT(bis) > parseT(von))} autocomplete="off" />
							</label>
						</div>
						<p class="fehler" aria-live="polite">{fehler}</p>
					</div>
					<div class="fuss">
						{#if !isNew}
							{@const id = /** @type {string} */ (c.id)}
							<button
								type="button"
								class="pille loeschen"
								onclick={() => {
									const k = id;
									close();
									ed.deleteClip(k);
								}}><IconMaterialSymbolsDeleteOutlineRounded />{m.delete_btn()}</button
							>
						{/if}
						<button type="button" class="pille" onclick={close}>{m.te_cancel()}</button>
						<button type="submit" class="pille tinte">{isNew ? m.te_clip_create() : m.te_apply()}</button>
					</div>
				</form>
			{:else}
				<div class="kopf">
					<IconMaterialSymbolsDownloadRounded />
					<b>{many ? m.te_clips_compilation({ count: list.length }) : list[0]?.title}</b>
					<span class="dauer">{fmtDur(total)}</span>
				</div>
				<p class="zitat leer">
					{many ? m.te_clips_joined() : `${fmt(list[0]?.start)} – ${fmt(list[0]?.end)}`}
				</p>
				<div class="formate">
					{#each [['mp3', m.te_clip_format_mp3()], ...(ed.isVideo ? [['mp4', m.te_clip_format_mp4()]] : []), ['txt', m.te_clip_format_txt()]] as [f, label] (f)}
						<button
							class="format"
							onclick={() => {
								const l = list;
								close();
								ed.exportClips(l, f);
							}}
						>
							{#if f === 'txt'}<IconMaterialSymbolsDescriptionOutlineRounded />{:else if f === 'mp4'}<IconMaterialSymbolsMovieOutlineRounded />{:else}<IconMaterialSymbolsGraphicEqRounded />{/if}
							<span>{label}</span>
						</button>
					{/each}
				</div>
				<div class="fuss">
					<button type="button" class="pille" onclick={close}>{m.te_cancel()}</button>
				</div>
			{/if}
		</div>
	{/key}
{/if}

<style>
	.clipdlg {
		position: fixed;
		z-index: 46;
		width: 23rem;
		max-width: calc(100vw - 1.5rem);
		max-height: calc(100dvh - 84px);
		overflow-y: auto;
		padding: 0.875rem 1rem 1rem;
		background: var(--paper);
		color: var(--ink);
		border-radius: 1rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	form,
	.clipdlg {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 0.75rem;
	}
	.kopf {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
	}
	.kopf :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		flex: none;
	}
	.kopf b {
		font-size: 1.0625rem;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dauer {
		margin-left: auto;
		font-size: 0.875rem;
		color: var(--ink-3);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	/* der Wortlaut des Clips: die Klammer darüber ist dieselbe wie im Text */
	.zitat {
		margin: 0;
		padding: 0.5rem 0.75rem;
		border-radius: 0.5rem;
		background: var(--desk);
		box-shadow: inset 0 2px 0 var(--clip);
		font-size: 1rem;
		line-height: 1.5;
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.zitat.leer {
		box-shadow: none;
		color: var(--ink-2);
		font-size: 0.9375rem;
	}
	.reihe,
	.fuss,
	.marken {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.fuss {
		justify-content: flex-end;
	}
	.felder {
		display: grid;
		gap: 0.625rem;
		padding-top: 0.75rem;
		border-top: 1px solid var(--rule);
	}
	.titel {
		font-weight: 600;
	}
	.zeiten {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		gap: 0.75rem;
	}
	.zeiten label {
		min-width: 0;
	}
	.felder input {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}
	.zahl {
		font-variant-numeric: tabular-nums;
	}
	.zahl[aria-invalid='true'] {
		border-color: var(--ink);
		box-shadow: inset 0 0 0 1px var(--ink);
	}
	/* Fehler stehen in Tinte (Rot gehört der menschlichen Hand) */
	.fehler {
		margin: 0;
		font-size: 0.9375rem;
		font-weight: 600;
		line-height: 1.4;
		color: var(--ink);
	}
	.fehler:empty {
		display: none;
	}
	.marke {
		min-height: 2.25rem;
		padding: 0 0.875rem;
		border: 1px solid var(--rule-2);
		border-radius: 2rem;
		background: var(--paper);
		color: var(--ink-2);
		font: inherit;
		font-size: 0.9375rem;
		cursor: pointer;
	}
	.marke:hover {
		border-color: var(--ink-3);
		color: var(--ink);
	}
	.marke[aria-pressed='true'] {
		background: var(--clip);
		border-color: var(--clip);
		color: var(--clip-ink);
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
	.reihe .pille,
	.pille.loeschen {
		padding-left: 0.75rem;
	}
	.pille:hover {
		background: var(--desk);
		border-color: var(--ink-3);
	}
	.pille :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		flex: none;
	}
	.pille.loeschen {
		margin-right: auto;
	}
	/* ein Clip ist ein Schnitt in Tinte: Anlegen und Übernehmen ebenso */
	.pille.tinte {
		background: var(--clip);
		border-color: var(--clip);
		color: var(--clip-ink);
	}
	.pille.tinte:hover {
		background: color-mix(in oklab, var(--clip) 80%, var(--paper));
		border-color: transparent;
	}
	.formate {
		display: grid;
		gap: 0.375rem;
	}
	.format {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		min-height: 3rem;
		padding: 0 1rem 0 0.875rem;
		border: 1px solid var(--rule-2);
		border-radius: 0.75rem;
		background: var(--paper);
		color: var(--ink);
		font: inherit;
		font-weight: 600;
		text-align: left;
		cursor: pointer;
	}
	.format:hover {
		background: var(--clip);
		border-color: var(--clip);
		color: var(--clip-ink);
	}
	.format :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
		margin: 0;
		flex: none;
	}
</style>
