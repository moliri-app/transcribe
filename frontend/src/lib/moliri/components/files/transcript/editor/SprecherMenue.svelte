<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Gespiegelt aus Moliri (src/lib/components/files/transcript/editor/SprecherMenue.svelte) von frontend/scripts/sync-moliri.mjs.
     Nicht hier ändern: in Moliri ändern, dann pnpm sync:moliri. -->
<!-- This Source Code Form is subject to the terms of the Mozilla Public
     License, v. 2.0. If a copy of the MPL was not distributed with this
     file, You can obtain one at https://mozilla.org/MPL/2.0/. -->
<!-- Ported from transcribe-go (frontend/src/lib/editor/SprecherMenue.svelte). -->
<!-- Sprecher bearbeiten, an einem Sprechernamen (im Text oder in der Seitenspalte). Oben die Ansicht, die
     sofort wirkt: nur diesen Sprecher zeigen oder ihn ausblenden. Darunter, was als Korrektur vermerkt
     wird: Name, Sprache und Zusammenlegen mit einem anderen Sprecher; Übernehmen oder Enter im Namen
     vermerkt es, Abbrechen, Esc oder ein Klick daneben lässt alles, wie es war. Zusammenlegen wendet
     Moliris Server gleich beim Speichern an; eine Sprache, die er kennt, erkennt er von selbst neu (in
     transcribe-go wirkt beides erst nach „Neu berechnen“). Wer
     nicht korrigieren darf, sieht nur die Ansicht. -->
<script>
	import * as msg from '$lib/moliri/texte.js';
	import { untrack } from 'svelte';
	import { fmt, speakingTime } from './model.js';
	import { spkColor } from './editor.svelte.js';
	import Sprachwahl from './Sprachwahl.svelte';
	import { langName, recognizable } from './sprachen.js';

	/** @type {{ ed: import('./editor.svelte.js').Editor }} */
	let { ed } = $props();

	/** @type {HTMLDivElement | undefined} */ let box = $state();
	/** @type {HTMLInputElement | undefined} */ let nameField = $state();
	let pos = $state({ left: 0, top: 0 });

	const m = $derived(ed.speakerMenu);
	const vis = $derived(m ? !ed.hidden.has(m.k) : true);
	const lastVisible = $derived(!!m && vis && ed.speakers.filter((k) => !ed.hidden.has(k)).length === 1);
	const dur = $derived(m ? (speakingTime(ed.words).get(m.k) ?? 0) : 0);
	const others = $derived(m ? ed.speakers.filter((k) => k !== m.k) : []);

	// jedes Öffnen beginnt mit dem, was vermerkt ist
	const cur = $derived(m ? untrack(() => ed.ann.speakers[m.k] ?? {}) : {});
	let name = $derived(m ? untrack(() => ed.spkName(m.k)) : '');
	let language = $derived(cur.language ?? '');
	let merge = $derived(cur.merge_into ?? '');
	const changed = $derived(
		!!m && (name.trim() !== untrack(() => ed.spkName(m.k)) || language !== (cur.language ?? '') || merge !== (cur.merge_into ?? ''))
	);
	const target = $derived(merge !== '' ? ed.spkName(merge) : '');

	$effect(() => {
		if (!m || !box) return;
		const r = m.anchor.getBoundingClientRect(),
			w = box.offsetWidth,
			h = box.offsetHeight;
		// unter dem Namen, sonst darüber (nicht unter die App-Leiste), sonst daneben, ohne ihn zu verdecken
		const top0 = 68;
		if (r.bottom + 8 + h < innerHeight - 8) pos = { left: Math.max(8, Math.min(r.left, innerWidth - w - 8)), top: r.bottom + 8 };
		else if (r.top - h - 8 >= top0) pos = { left: Math.max(8, Math.min(r.left, innerWidth - w - 8)), top: r.top - h - 8 };
		else
			pos = {
				left: r.left - w - 12 >= 8 ? r.left - w - 12 : Math.min(r.right + 12, innerWidth - w - 8),
				top: Math.max(top0, Math.min(r.top - 24, innerHeight - h - 8))
			};
		untrack(() => {
			nameField?.focus();
			nameField?.select();
		});
		// wächst der Dialog (Hinweis zum Zusammenlegen, Feld für den Sprachcode), rückt er nach oben
		const el = box;
		const ro = new ResizeObserver(() => {
			const over = pos.top + el.offsetHeight - (innerHeight - 8);
			if (over > 0) pos = { ...pos, top: Math.max(top0, pos.top - over) };
		});
		ro.observe(el);
		return () => ro.disconnect();
	});

	function close(focusBack = true) {
		const a = m?.anchor;
		ed.speakerMenu = null;
		if (focusBack && a?.isConnected) a.focus();
	}
	function apply() {
		if (!m) return;
		if (changed) ed.setSpeaker(m.k, { name, language, merge_into: merge });
		close();
	}
	function outside(/** @type {PointerEvent} */ e) {
		if (m && box && !box.contains(/** @type {Node} */ (e.target)) && e.target !== m.anchor) close(false);
	}
</script>

<svelte:window onpointerdown={outside} onkeydown={(e) => m && e.key === 'Escape' && close()} />

{#if m}
	{#key m}
		<div
			class="menue"
			role="dialog"
			tabindex="-1"
			aria-label={msg.te_speaker_dialog({ name: ed.spkName(m.k) })}
			bind:this={box}
			style:left="{pos.left}px"
			style:top="{pos.top}px"
			style:--c={spkColor(m.k)}
		>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					apply();
				}}
			>
				<div class="kopf">
					<span class="pkt" aria-hidden="true"></span>
					<b>{ed.spkName(m.k)}</b>
					<span class="dauer">{msg.te_speaks({ time: fmt(dur) })}</span>
				</div>

				{#if others.length}
					<div class="ansicht">
						<button
							type="button"
							class="pille"
							onclick={() => {
								ed.focusSpeaker(m.k);
								close();
							}}
						>
							<IconMaterialSymbolsFilterCenterFocusRounded />
							{ed.focused === m.k ? msg.te_show_all() : msg.te_show_only_this()}
						</button>
						<button
							type="button"
							class="pille"
							disabled={lastVisible}
							title={lastVisible ? msg.te_one_stays_visible() : msg.te_hide_hint()}
							onclick={() => {
								ed.toggleSpeaker(m.k);
								close();
							}}
						>
							{#if vis}<IconMaterialSymbolsVisibilityOffOutlineRounded />{msg.te_hide()}{:else}<IconMaterialSymbolsVisibilityOutlineRounded />{msg.te_show_again()}{/if}
						</button>
					</div>
				{/if}

				{#if ed.canEdit}
				<div class="felder">
					<label class="feldname">
						{msg.te_name()}
						<input class="feld name" bind:this={nameField} bind:value={name} spellcheck="false" autocomplete="off" />
					</label>
					<Sprachwahl bind:value={language} none={msg.te_language_auto()} />
					{#if others.length}
						<label class="feldname">
							{msg.te_merge_with()}
							<select class="feld" bind:value={merge}>
								<option value="">{msg.te_merge_none()}</option>
								{#each others as k (k)}<option value={String(k)}>{ed.spkName(k)}</option>{/each}
							</select>
						</label>
					{/if}
					{#if target}
						<p class="folge">
							<IconMaterialSymbolsCallMergeRounded />
							<span>{ed.recomputes ? msg.te_merge_note_recompute({ from: ed.spkName(m.k), into: target }) : msg.te_merge_note({ from: ed.spkName(m.k), into: target })}</span>
						</p>
					{:else if language}
						<p class="folge">
							<IconMaterialSymbolsTranslateRounded />
							<span
								>{msg.te_speaker_language_note({ name: ed.spkName(m.k) })}
								{#if recognizable(language)}{ed.recomputes ? msg.te_language_redone_recompute({ language: langName(language) }) : msg.te_language_redone({ language: langName(language) })}{:else}{msg.te_language_noted({ language: langName(language) })}{/if}</span
							>
						</p>
					{/if}
				</div>

				<div class="fuss">
					<button type="button" class="pille" onclick={() => close()}>{msg.te_cancel()}</button>
					<button type="submit" class="pille rot" disabled={!changed}>{msg.te_apply()}</button>
				</div>
				{/if}
			</form>
		</div>
	{/key}
{/if}

<style>
	.menue {
		position: fixed;
		z-index: 45;
		width: 22rem;
		max-width: calc(100vw - 1rem);
		max-height: calc(100dvh - 76px);
		overflow-y: auto;
		padding: 0.875rem 1rem 1rem;
		background: var(--paper);
		color: var(--ink);
		border-radius: 1rem;
		box-shadow:
			0 1px 2px hsl(var(--color-neutral-100) / 0.08),
			0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3);
	}
	form {
		display: grid;
		gap: 0.75rem;
	}
	.kopf {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0 0.125rem;
	}
	/* der Name, wie er in der Randspalte steht: Versalien in Sprecherfarbe */
	.kopf b {
		font-size: 0.9375rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--c);
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
	.pkt {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		background: var(--c);
		flex: none;
	}
	.ansicht,
	.fuss {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.fuss {
		justify-content: flex-end;
	}
	/* was vermerkt wird, steht unter einer Haarlinie: anders als die Ansicht wirkt es erst mit Übernehmen */
	.felder {
		display: grid;
		gap: 0.625rem;
		padding-top: 0.75rem;
		border-top: 1px solid var(--rule);
	}
	.name {
		font-weight: 600;
	}
	.folge {
		display: flex;
		gap: 0.5rem;
		align-items: flex-start;
		margin: 0;
		padding: 0.5rem 0.75rem;
		border-radius: 0.5rem;
		background: var(--desk);
		font-size: 0.9375rem;
		line-height: 1.45;
		color: var(--ink-2);
	}
	.folge :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0.0625rem 0 0;
		flex: none;
		color: var(--ink-3);
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
	.ansicht .pille {
		padding-left: 0.75rem;
	}
	.pille:hover:not(:disabled) {
		background: var(--desk);
		border-color: var(--ink-3);
	}
	.pille:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.pille :global(svg) {
		width: 1.25rem;
		height: 1.25rem;
		margin: 0;
		flex: none;
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
</style>
