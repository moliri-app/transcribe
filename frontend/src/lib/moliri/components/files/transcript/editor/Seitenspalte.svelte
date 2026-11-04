<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/Seitenspalte.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/Seitenspalte.svelte). -->
<!-- Seitenspalte: oben das Video (bei Audio unsichtbar, spielt aber den Ton), dann die Sprecher mit
     Sprechzeit, Ausblenden (Auge) und Fokus (nur diesen zeigen), der Name öffnet „Sprecher bearbeiten“;
     die Clips mit Filter nach Schlagwort, Abspielen, Exportieren und Bearbeiten, abgehakt nacheinander
     abspielen oder als Zusammenschnitt exportieren; die Korrekturen;
     dazu die Zeichen im Text und die Modelle, mit denen transkribiert wurde (CC BY 4.0 verlangt ihre
     Nennung). Unter 1180 px wird sie ein aufklappbarer Block unter dem Manuskript. -->
<script>
	import * as m from '$lib/moliri/texte.js';
	import { onMount } from 'svelte';
	import { describeEdit, fmt, fmtDur, langName, speakingTime } from './model.js';
	import { spkColor } from './editor.svelte.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLVideoElement} */ let video;
	let open = $state(true);

	onMount(() => {
		const off = ed.attach(video);
		const narrow = matchMedia('(max-width: 1180px)');
		const sync = () => (open = !narrow.matches);
		sync();
		narrow.addEventListener('change', sync);
		return () => {
			off();
			narrow.removeEventListener('change', sync);
		};
	});

	const tot = $derived(speakingTime(ed.words));
	const list = $derived([...tot.keys()].sort((a, b) => (tot.get(b) ?? 0) - (tot.get(a) ?? 0)));
	const many = $derived(list.length > 1);

	// alle Korrekturen in zeitlicher Folge: Bereiche (Sprecher, Sprache) und Text samt Zeitankern
	const corr = $derived(
		[
			...ed.ann.spans.map((s, i) => ({
				key: 's' + i,
				kind: /** @type {const} */ ('spans'),
				i,
				t: s.start,
				what: [s.language && langName(s.language), s.speaker !== undefined && s.speaker !== '' && '→ ' + ed.spkName(s.speaker)].filter(Boolean).join(' · '),
				em: '',
				after: m.te_until({ time: fmt(s.end) })
			})),
			...ed.ann.edits.map((e, i) => ({ key: 'e' + i, kind: /** @type {const} */ ('edits'), i, ...describeEdit(e) }))
		].sort((a, b) => a.t - b.t)
	);

	// Clips: gefiltert nach Schlagwort; abgehakt zählt nur, was es noch gibt (nach Löschen, Rückgängig)
	const shownClips = $derived(ed.ann.clips.filter((c) => !ed.clipTag || (c.tags || []).includes(ed.clipTag)));
	const chosen = $derived(ed.ann.clips.filter((c) => ed.clipSel.has(c.id)));
	const visSel = $derived(shownClips.filter((c) => ed.clipSel.has(c.id)).length);
	const playingChosen = $derived(
		ed.playing && !!ed.playList && ed.playList.length === chosen.length && chosen.every((c) => ed.playList?.some((r) => r.id === c.id))
	);
	// ein Schlagwort, das kein Clip mehr trägt, filtert nicht mehr
	$effect(() => {
		if (ed.clipTag && !ed.allTags.includes(ed.clipTag)) ed.clipTag = null;
	});
</script>

<aside class="seite" aria-label={m.te_side()}>
	<div class="video" class:da={ed.isVideo}>
		<!-- svelte-ignore a11y_media_has_caption -->
		<video bind:this={video} preload="auto" playsinline onclick={() => ed.toggle()}></video>
	</div>
	<details bind:open>
		<summary>{m.te_side()}</summary>
		<h2 id="h-sprecher">{m.transcript_speakers()}</h2>
		<div role="list" aria-label={m.transcript_speakers()} class="sprecher">
			{#each list as k (k)}
				{@const vis = !ed.hidden.has(k)}
				{@const nm = ed.spkName(k)}
				{@const sp = ed.ann.speakers[k] || {}}
				{@const sub = [sp.language && langName(sp.language), sp.merge_into !== undefined && m.te_merged_with({ name: ed.spkName(sp.merge_into) })].filter(Boolean).join(' · ')}
				<div class="schild" class:aus={!vis} role="listitem" style:--c={spkColor(k)}>
					<span class="dot" aria-hidden="true"></span>
					<button
						class="nm"
						title={m.te_speaker_edit_hint()}
						aria-haspopup="dialog"
						onclick={(e) => (ed.speakerMenu = { k, anchor: /** @type {HTMLElement} */ (e.currentTarget) })}>{nm}</button
					>
					<span class="st">{fmt(tot.get(k) ?? 0)}</span>
					{#if many}
						<button
							class="sw"
							aria-pressed={!vis}
							disabled={ed.focused === k}
							title={ed.focused === k ? m.te_one_stays_visible() : vis ? m.te_hide_named({ name: nm }) : m.te_show_named({ name: nm })}
							aria-label={vis ? m.te_hide_named({ name: nm }) : m.te_show_named({ name: nm })}
							onclick={() => ed.toggleSpeaker(k)}
						>
							{#if vis}<IconMaterialSymbolsVisibilityOutlineRounded />{:else}<IconMaterialSymbolsVisibilityOffOutlineRounded />{/if}
						</button>
						<button
							class="sw fokus"
							aria-pressed={ed.focused === k}
							title={ed.focused === k ? m.te_show_all_speakers() : m.te_only_named_play({ name: nm })}
							aria-label={m.te_only_named({ name: nm })}
							onclick={() => ed.focusSpeaker(k)}
						>
							<IconMaterialSymbolsFilterCenterFocusRounded />
						</button>
					{/if}
					{#if sub}<span class="sub">{sub}</span>{/if}
				</div>
			{:else}
				<p class="leer">{m.te_no_speakers()}</p>
			{/each}
		</div>
		{#if many}
			<p class="hinweis">{ed.canEdit ? m.te_speakers_hint_edit() : ''} {m.te_speakers_hint()}</p>
		{/if}

		<h2>{m.te_clips()}</h2>
		{#if ed.allTags.length && ed.ann.clips.length > 1}
			<div class="marken" role="group" aria-label={m.te_filter_by_tag()}>
				{#each ed.allTags as t (t)}
					<button
						class="marke"
						aria-pressed={ed.clipTag === t}
						title={ed.clipTag === t ? m.te_show_all_clips() : m.te_only_clips_with({ tag: t })}
						onclick={() => (ed.clipTag = ed.clipTag === t ? null : t)}>{t}</button
					>
				{/each}
			</div>
		{/if}
		<div role="list" aria-label={m.te_clips()} class="clips">
			{#each shownClips as c (c.id)}
				{@const spielt = ed.playingClip === c.id}
				<div class="clip" class:aktiv={ed.activeClip === c.id} class:spielt role="listitem">
					<input
						type="checkbox"
						checked={ed.clipSel.has(c.id)}
						aria-label={m.te_select_named({ title: c.title })}
						title={m.te_select_clip_hint()}
						onchange={(e) => ed.toggleClipSel(c.id, e.currentTarget.checked)}
					/>
					<div class="inhalt">
						<button class="ct" title={m.te_jump_to_clip()} onclick={() => ed.showClip(c)}>{c.title}</button>
						<span class="zt">{fmt(c.start)} – {fmt(c.end)} · {fmtDur(c.end - c.start)}</span>
					</div>
					<div class="unten">
						{#if c.tags?.length}
							<span class="tags">{#each c.tags as t (t)}<span>{t}</span>{/each}</span>
						{/if}
						<span class="knoepfe">
							<button
								class="sw cplay"
								aria-label={spielt ? m.te_pause() : m.te_play_this_clip()}
								title={spielt ? m.te_pause() : m.te_play_this_clip()}
								onclick={() => (spielt ? ed.media?.pause() : ed.showClip(c, true))}
							>
								{#if spielt}<IconMaterialSymbolsPauseRounded />{:else}<IconMaterialSymbolsPlayArrowRounded />{/if}
							</button>
							<button
								class="sw"
								data-clip-oeffnet
								aria-label={m.te_clip_export_named({ title: c.title })}
								title={ed.isVideo ? m.te_clip_export_hint_video() : m.te_clip_export_hint()}
								onclick={(e) => (ed.clipDlg = { kind: 'export', list: [c], x: e.clientX - 340, y: e.clientY })}
							>
								<IconMaterialSymbolsDownloadRounded />
							</button>
							{#if ed.canEdit}<button
								class="sw"
								data-clip-oeffnet
								aria-label={m.te_edit_named({ title: c.title })}
								title={m.te_clip_edit_hint()}
								onclick={(e) => {
									ed.showClip(c);
									ed.clipDlg = { kind: 'clip', clip: c, x: e.clientX - 340, y: e.clientY };
								}}
							>
								<IconMaterialSymbolsEditOutlineRounded />
							</button>{/if}
						</span>
					</div>
				</div>
			{:else}
				<p class="leer">
					{ed.ann.clips.length ? m.te_no_clip_with_tag() : ed.canEdit ? m.te_no_clips_hint() : m.te_none_yet()}
				</p>
			{/each}
		</div>
		{#if ed.ann.clips.length > 1}
			<div class="clipbar">
				<label class="alle">
					<input
						type="checkbox"
						aria-label={m.te_select_all()}
						checked={shownClips.length > 0 && visSel === shownClips.length}
						indeterminate={visSel > 0 && visSel < shownClips.length}
						onchange={(e) => {
							const on = e.currentTarget.checked;
							const s = new Set(ed.clipSel);
							for (const c of shownClips) on ? s.add(c.id) : s.delete(c.id);
							ed.clipSel = s;
						}}
					/>
					<span>{chosen.length ? m.te_selected({ count: chosen.length, time: fmtDur(chosen.reduce((t, c) => t + c.end - c.start, 0)) }) : m.te_select_all()}</span>
				</label>
				<div class="reihe">
					<!-- spielt von vorn; anhalten über den Player (der Name bleibt, damit „Anhalten“ eindeutig der Player ist) -->
					<button class="pille" class:spielt={playingChosen} disabled={!chosen.length} onclick={() => ed.playClips(chosen)}>
						<IconMaterialSymbolsPlaylistPlayRounded />{m.te_play_in_turn()}
					</button>
					<button
						class="pille tinte"
						data-clip-oeffnet
						disabled={!chosen.length}
						onclick={(e) => (ed.clipDlg = { kind: 'export', list: chosen, x: e.clientX - 340, y: e.clientY - 160 })}
					>
						<IconMaterialSymbolsDownloadRounded />{chosen.length > 1 ? m.te_export_compilation() : m.transcript_export()}
					</button>
				</div>
			</div>
		{/if}

		<h2>{m.te_corrections()}</h2>
		<div role="list" aria-label={m.te_corrections()} class="korrekturen">
			{#each corr as c (c.key)}
				<div class="korr" role="listitem">
					<button class="hin" title={m.te_play_from({ time: fmt(c.t) })} onclick={() => ed.seek(c.t)}>
						<span class="t">{fmt(c.t)}</span>
						<span class="was">{#if c.what}{c.what + ' '}{/if}{#if c.em}<em>{c.em}</em>{/if}{#if c.after}{' '}<span class="nach">{c.after}</span>{/if}</span>
					</button>
					{#if ed.canEdit}<button class="x" title={m.te_remove_correction()} aria-label={m.te_remove_correction()} onclick={() => ed.removeCorrection(c.kind, c.i)}>
						<IconMaterialSymbolsCloseRounded />
					</button>{/if}
				</div>
			{:else}
				<p class="leer">{ed.canEdit ? m.te_no_corrections_hint() : m.te_none_yet()}</p>
			{/each}
		</div>

		<h2>{m.te_signs()}</h2>
		<ul class="zeichen">
			<li><i class="linie" style:border-color="var(--red)"></i>{m.te_sign_edited()}</li>
			<li><i class="linie" style:border-color="var(--doubt)" style:border-bottom-style="dotted"></i>{m.te_sign_doubt()}</li>
			<li><i class="linie" style:border-color="var(--blue)"></i>{m.te_sign_redone()}</li>
			<li><i class="linie" style:border-color="var(--fill)"></i>{m.te_sign_fill()}</li>
			<li><i class="anker"></i>{m.te_sign_anchor()}</li>
			<li><i class="marker"></i>{m.te_sign_now()}</li>
			<li><i class="klammer"></i>{m.te_sign_clip()}</li>
		</ul>
		{#if ed.attribution}<p class="modelle">{ed.attribution}</p>{/if}
	</details>
</aside>

<style>
	.seite {
		border-left: 1px solid var(--rule);
		background: var(--paper);
		overflow-y: auto;
		padding: 1.375rem 1.375rem 2.5rem;
		min-height: 0;
		scrollbar-width: thin;
		scrollbar-color: var(--rule-2) transparent;
	}
	.video {
		display: none;
	}
	.video.da {
		display: block;
		position: sticky;
		top: -1.375rem;
		z-index: 2;
		margin: -1.375rem -1.375rem 1rem;
		background: #000;
	}
	video {
		display: block;
		width: 100%;
		max-height: 40vh;
		background: #000;
		cursor: pointer;
	}
	summary {
		display: none;
	}
	h2 {
		font-size: 0.8125rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--ink-2);
		margin: 2rem 0 0.375rem;
	}
	h2:first-of-type {
		margin-top: 0;
	}

	.schild {
		display: grid;
		grid-template-columns: 0.75rem minmax(0, 1fr) auto auto auto;
		gap: 0.125rem 0.5rem;
		align-items: center;
		border-bottom: 1px solid var(--rule);
		padding: 0.25rem 0;
		color: var(--ink);
	}
	.dot {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		background: var(--c);
		border: 2px solid var(--c);
	}
	.nm {
		background: none;
		border: 0;
		padding: 0.375rem 0;
		min-height: var(--hit);
		text-align: left;
		font: inherit;
		font-weight: 600;
		font-size: 1rem;
		color: inherit;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		cursor: pointer;
	}
	.nm:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.st {
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: var(--ink-3);
		margin-right: 0.125rem;
	}
	.sub {
		grid-column: 2 / 6;
		font-size: 0.875rem;
		color: var(--ink-3);
		padding-bottom: 0.25rem;
	}
	.sw {
		border: 0;
		background: none;
		color: var(--ink-3);
		width: var(--hit);
		height: var(--hit);
		border-radius: 50%;
		display: grid;
		place-items: center;
		padding: 0;
		cursor: pointer;
	}
	.sw :global(svg) {
		width: 1.375rem;
		height: 1.375rem;
		margin: 0;
	}
	.sw:hover:not(:disabled) {
		background: var(--desk);
		color: var(--ink);
	}
	.sw:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.sw[aria-pressed='true'] {
		background: var(--head);
		color: var(--signal);
	}
	.sw[aria-pressed='true']:hover:not(:disabled) {
		background: var(--head);
		color: var(--signal);
	}
	.schild.aus .dot {
		background: transparent;
	}
	.schild.aus .nm,
	.schild.aus .st {
		color: var(--ink-3);
	}
	.schild.aus .nm {
		text-decoration: line-through;
		text-decoration-color: var(--rule-2);
	}
	.leer,
	.hinweis {
		font-size: 0.9375rem;
		color: var(--ink-3);
		line-height: 1.5;
	}
	.hinweis {
		margin-top: 0.75rem;
	}

	/* Clips: Schild mit Häkchen, Titel (springt hin), Zeit und Dauer, Schlagworten und drei Kreisknöpfen */
	.marken {
		display: flex;
		flex-wrap: wrap;
		gap: 0.375rem;
		margin: 0.5rem 0 0.375rem;
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
	.clip {
		display: grid;
		grid-template-columns: 1.75rem minmax(0, 1fr);
		column-gap: 0.375rem;
		padding: 0.625rem 0 0.25rem;
		border-bottom: 1px solid var(--rule);
	}
	.clip input,
	.alle input {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0.5rem 0 0 0.125rem;
		cursor: pointer;
	}
	.inhalt {
		min-width: 0;
		/* die Klammer des Clips, wie über seinen Wörtern im Text */
		border-top: 2px solid var(--clip);
		padding-top: 0.25rem;
	}
	.ct {
		display: block;
		max-width: 100%;
		background: none;
		border: 0;
		padding: 0.125rem 0;
		text-align: left;
		font: inherit;
		font-size: 1rem;
		font-weight: 600;
		line-height: 1.35;
		color: var(--ink);
		cursor: pointer;
	}
	.ct:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.zt {
		display: block;
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: var(--ink-3);
	}
	.unten {
		grid-column: 2;
		display: flex;
		align-items: center;
		gap: 0.25rem;
		min-width: 0;
	}
	.tags {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		min-width: 0;
	}
	.tags span {
		font-size: 0.875rem;
		color: var(--ink-2);
		background: var(--desk);
		border-radius: 1rem;
		padding: 0.0625rem 0.625rem;
	}
	.knoepfe {
		display: flex;
		margin-left: auto;
		margin-right: -0.5rem;
	}
	/* zuletzt gezeigt: der Titel bekommt den Auswahl-Hauch; läuft: der Abspielknopf wird Signal */
	.clip.aktiv .inhalt {
		background: linear-gradient(var(--sel), transparent 85%);
	}
	.cplay {
		color: var(--ink);
	}
	.clip.spielt .cplay {
		background: var(--signal);
		color: var(--head);
	}
	.clip.spielt .cplay:hover {
		background: var(--signal);
		color: var(--head);
	}
	.clipbar {
		margin-top: 0.75rem;
		padding: 0.625rem 0.75rem 0.75rem;
		background: var(--desk);
		border-radius: 0.75rem;
		display: grid;
		gap: 0.5rem;
	}
	.alle {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		font-size: 0.9375rem;
		color: var(--ink-2);
		font-variant-numeric: tabular-nums;
		cursor: pointer;
	}
	.alle input {
		margin: 0 0.125rem;
	}
	.clipbar .reihe {
		display: grid;
		gap: 0.375rem;
	}
	.pille {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		min-height: var(--hit);
		padding: 0 0.875rem 0 0.625rem;
		border: 1px solid var(--rule-2);
		border-radius: 2rem;
		background: var(--paper);
		color: var(--ink);
		font: inherit;
		font-size: 0.9375rem;
		font-weight: 600;
		cursor: pointer;
	}
	.pille :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		flex: none;
	}
	.pille:hover:not(:disabled) {
		border-color: var(--ink-3);
	}
	.pille:disabled {
		opacity: 0.45;
		cursor: default;
	}
	.pille.spielt :global(svg) {
		color: hsl(var(--color-green-300));
	}
	.pille.tinte {
		background: var(--clip);
		border-color: var(--clip);
		color: var(--clip-ink);
	}
	.pille.tinte:hover:not(:disabled) {
		background: color-mix(in oklab, var(--clip) 80%, var(--paper));
	}

	/* Korrekturen: Objektschild mit Zeit, was ein Mensch getan hat (in Rot) und Entfernen */
	.korr {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		border-bottom: 1px solid var(--rule);
	}
	.hin {
		display: grid;
		grid-template-columns: 2.75rem minmax(0, 1fr);
		align-items: baseline;
		gap: 0.5rem;
		min-height: var(--hit);
		padding: 0.5rem 0.25rem 0.5rem 0;
		border: 0;
		border-radius: 0.375rem;
		background: none;
		color: var(--ink);
		font: inherit;
		font-size: 0.9375rem;
		text-align: left;
		cursor: pointer;
	}
	.hin:hover .was {
		text-decoration: underline;
		text-decoration-color: var(--rule-2);
		text-underline-offset: 3px;
	}
	.t {
		font-size: 0.875rem;
		font-variant-numeric: tabular-nums;
		color: var(--ink-3);
	}
	.was {
		overflow-wrap: anywhere;
		line-height: 1.4;
	}
	.was em {
		font-style: normal;
		font-weight: 600;
		color: var(--red);
	}
	.nach {
		color: var(--ink-3);
		font-variant-numeric: tabular-nums;
	}
	.x {
		width: var(--hit);
		height: var(--hit);
		display: grid;
		place-items: center;
		padding: 0;
		border: 0;
		border-radius: 50%;
		background: none;
		color: var(--ink-3);
		cursor: pointer;
	}
	.x:hover {
		background: var(--desk);
		color: var(--ink);
	}
	.x :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
	}

	.zeichen {
		list-style: none;
		margin: 0.5rem 0 0;
		padding: 0;
		display: grid;
		gap: 0.5rem;
		font-size: 0.9375rem;
		color: var(--ink-2);
	}
	.modelle {
		margin: 1.5rem 0 0;
		font-size: 0.8125rem;
		line-height: 1.5;
		color: var(--ink-3);
	}
	.zeichen li {
		display: flex;
		align-items: center;
		gap: 0.625rem;
	}
	.zeichen i {
		display: inline-block;
		flex: none;
		width: 1.375rem;
	}
	.linie {
		height: 0;
		border-bottom: 2px solid;
	}
	.marker {
		height: 0.875rem;
		border-radius: 3px;
		background: var(--marker);
	}
	.klammer {
		height: 0.75rem;
		border-top: 2px solid var(--clip);
		margin-top: 0.5rem;
	}
	.anker {
		height: 13px;
		position: relative;
	}
	.anker::before,
	.anker::after {
		content: '';
		position: absolute;
		top: 0;
		width: 8px;
		height: 13px;
		background: var(--ink-3);
		mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 8 13'%3E%3Cpath d='M0 0h1.6v13H0z'/%3E%3Cpath d='M1.6 0 8 2.7 1.6 5.4z'/%3E%3C/svg%3E")
			no-repeat 0 0 / 8px 13px;
	}
	.anker::before {
		left: 2px;
	}
	.anker::after {
		left: 13px;
		background: var(--red);
	}

	@media (max-width: 1180px) {
		.seite {
			border-left: 0;
			border-top: 1px solid var(--rule);
			max-height: 42vh;
			padding: 0 1.375rem;
		}
		.video.da {
			top: 0;
			margin: 0 -1.375rem 0.5rem;
		}
		summary {
			display: flex;
			align-items: center;
			gap: 0.5rem;
			min-height: 3rem;
			cursor: pointer;
			font-weight: 700;
			font-size: 0.8125rem;
			letter-spacing: 0.08em;
			text-transform: uppercase;
			color: var(--ink-2);
		}
		details[open] {
			padding-bottom: 1.5rem;
		}
		h2:first-of-type {
			margin-top: 0.25rem;
		}
	}
</style>
