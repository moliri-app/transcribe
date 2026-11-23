// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Texts the EXE sends, in the interface language. The server says its messages as a stable key with
// arguments (internal/msg: `msg`, `errorMsg`, `partialMsg` … beside the German `error` and the like,
// which programs calling the API keep getting); here they become text through wuchale, like every
// other text of the interface. A key this file does not know shows the server's German text, so a
// new message in Go needs its entry here (TestFrontendKnowsEveryMessage in internal/msg checks).
// The same goes for the names of components and the steps of a transcription.

import { getLocale } from '$lib/moliri/i18n/locale.js';

/** @typedef {{ key: string, args?: Record<string, unknown> }} Meldung */

/**
 * A message of the server in the interface language; without one, or for one this file does not
 * know, sonst (the server's own text).
 * @param {Meldung | null | undefined} m
 * @param {string} [sonst]
 * @returns {string}
 */
export function meldung(m, sonst = '') {
	if (!m || typeof m !== 'object' || typeof m.key !== 'string') return sonst;
	return text(m) ?? sonst;
}

/** @param {unknown} v */
function istMeldung(v) {
	return !!v && typeof v === 'object' && typeof (/** @type {any} */ (v).key) === 'string';
}

/**
 * An argument as the text shows it: a nested message translated, a number in the interface's way.
 * @param {Meldung} m @param {string} name
 * @returns {string}
 */
export function arg(m, name) {
	const v = m.args?.[name];
	if (istMeldung(v)) return text(/** @type {Meldung} */ (v)) ?? '';
	if (typeof v === 'number') return v.toLocaleString(getLocale());
	return v == null ? '' : String(v);
}

/**
 * The arguments of m as they show (arg), read as a.name: a name in quotes inside a text would be
 * taken by wuchale as a text to translate.
 * @param {Meldung} m
 * @returns {Record<string, string>}
 */
export const argumente = (m) => new Proxy({}, { get: (_, name) => arg(m, String(name)) });

/**
 * @param {Meldung} m
 * @returns {string | null}
 */
function text(m) {
	const a = argumente(m);
	/** with what the service itself said, if anything */
	const mit = (/** @type {string} */ s) => `${s} ${a.detail}`.trim();
	switch (m.key) {
		case 'raw':
			return a.text;

		// the data folder (settings)
		case 'dataDir.relative':
			return /* @wc-include */ 'Please enter a full path, such as D:\\Transcribe.';
		case 'dataDir.same':
			return /* @wc-include */ 'The data is already there.';
		case 'dataDir.inside':
			return /* @wc-include */ 'The new location must not be inside the current data folder.';
		case 'dataDir.notEmpty':
			return /* @wc-include */ 'There are other files there already. Please choose an empty folder.';
		case 'dataDir.notDir':
			return /* @wc-include */ 'That is a file, not a folder.';
		case 'dataDir.busyJob':
			return /* @wc-include */ 'A job is running. The data folder can be moved once all jobs are done.';
		case 'dataDir.busyRecording':
			return /* @wc-include */ 'A recording is running. The data folder can be moved once it has ended.';
		case 'dataDir.busyModels':
			return /* @wc-include */ 'The app is downloading models. The data folder can be moved once the download is done.';
		case 'dataDir.busyComponents':
			return /* @wc-include */ 'The app is downloading components. The data folder can be moved once the download is done.';
		case 'dataDir.fixed':
			return /* @wc-include */ 'The data folder was set at startup (--data or TRANSCRIBE_DATA) and cannot be changed here.';
		case 'dataDir.moving':
			return /* @wc-include */ 'The data folder is already being moved.';
		case 'dataDir.moveFailed':
			return /* @wc-include */ `Moving did not work; the data is still in its old place. (${a.error})`;
		case 'pick.open':
			return /* @wc-include */ 'The folder dialog is already open.';

		// connected folders, the library
		case 'folders.relative':
			return /* @wc-include */ 'Please enter the full path of the folder, such as C:\\Nextcloud\\Interviews.';
		case 'folders.missing':
			return /* @wc-include */ `This folder does not exist: ${a.path}`;
		case 'library.linkedNoDelete':
			return /* @wc-include */ 'Transcribe does not delete anything in a connected folder.';
		case 'library.deleteRunning':
			return /* @wc-include */ 'This recording is being transcribed. Cancel that first.';

		// transcriptions
		case 'job.interrupted':
			return /* @wc-include */ 'Transcribe ended unexpectedly during this transcription. “Retry” starts it again.';
		case 'job.requeued':
			return /* @wc-include */ 'queued again after a restart';
		case 'job.empty':
			return /* @wc-include */ 'The recording is empty: there is no sound in it.';
		case 'job.tooShort':
			return /* @wc-include */ `The recording is too short (${a.seconds} s) to recognize anything in it.`;
		case 'job.localOff':
			return /* @wc-include */ 'Recognition on this computer is turned off (Settings, Recognition: external API only).';

		// speech recognition services
		case 'api.noURL':
			return /* @wc-include */ 'The address of the API is missing (Settings, Recognition).';
		case 'api.noKey':
			return /* @wc-include */ 'No key for the API has been entered yet (Settings, Recognition).';
		case 'api.unreachable':
			return /* @wc-include */ `${a.service} cannot be reached: ${a.error}`;
		case 'api.noAnswer':
			return /* @wc-include */ `${a.service} does not answer: ${a.error}`;
		case 'api.badAnswer':
			return /* @wc-include */ `${a.service}: unexpected answer: ${a.error}`;
		case 'api.keyRejected':
			return mit(/* @wc-include */ `${a.service} rejected the API key.`);
		case 'api.exhausted':
			return mit(/* @wc-include */ `${a.service} is not accepting anything right now (credit or requests used up).`);
		case 'api.error':
			return mit(/* @wc-include */ `${a.service}: error ${a.status}:`);
		case 'api.refused':
			return /* @wc-include */ `${a.service} refuses: ${a.reason}`;
		case 'api.keyNotAccepted':
			return /* @wc-include */ 'The API key was not accepted.';
		case 'api.quota':
			return /* @wc-include */ 'Credit or requests are used up.';

		// text recognition
		case 'ocr.unavailable':
			return /* @wc-include */ 'Text recognition is not available';
		case 'ocr.tooBig':
			return /* @wc-include */ 'Image larger than 64 MB';
		case 'ocr.noWindowsLanguage':
			return /* @wc-include */ 'Windows has no language installed for text recognition (Settings › Time & language › Language)';
		case 'ocr.tesseractLater':
			return /* @wc-include */ 'Tesseract is downloaded the first time it reads';
		case 'ocrApi.noURL':
			return /* @wc-include */ 'The address of the AI service is missing (Settings, Text recognition)';
		case 'ocrApi.noModel':
			return /* @wc-include */ 'The model of the AI service is missing (Settings, Text recognition)';
		case 'ocrApi.noKey':
			return /* @wc-include */ 'No key for the AI service has been entered yet (Settings, Text recognition)';
		case 'ocrApi.unreachable':
			return mit(/* @wc-include */ `${a.service} cannot be reached (${a.url}).`);
		case 'ocrApi.timeout':
			return /* @wc-include */ `${a.service} did not answer in time (${a.detail}).`;
		case 'ocrApi.keyRejected':
			return mit(/* @wc-include */ `${a.service} rejected the API key.`);
		case 'ocrApi.exhausted':
			return mit(/* @wc-include */ `${a.service} is not accepting anything right now (credit or requests used up).`);
		case 'ocrApi.unknownModel':
			return mit(/* @wc-include */ `${a.service} does not know the model “${a.model}”, or it cannot read images.`);
		case 'ocrApi.noEndpointStatus':
			return mit(/* @wc-include */ `No matching interface answers at ${a.url} (error ${a.status}). Is the address right?`);
		case 'ocrApi.noEndpoint':
			return mit(/* @wc-include */ `No matching interface answers at ${a.url}. Is the address right?`);
		case 'ocrApi.serverError':
			return mit(/* @wc-include */ `${a.service} reported an error (${a.status}).`);
		case 'ocrApi.imageRefused':
			return mit(/* @wc-include */ `${a.service} refused the image:`);
		case 'ocrApi.error':
			return mit(/* @wc-include */ `${a.service}: error ${a.status}:`);

		// requests of programs, on the Live page
		case 'call.gone':
			return /* @wc-include */ 'The program stopped waiting for the answer';
		case 'call.loadingModel':
			return /* @wc-include */ 'Loading the model …';
		case 'call.modelMissing':
			return /* @wc-include */ 'The model has not been downloaded yet';
		case 'call.llmMissing':
			return /* @wc-include */ 'The language model has not been downloaded yet';
		case 'call.llmNotLoaded':
			return /* @wc-include */ 'The language model is not loaded in Transcribe';
		case 'call.catalogTooLong':
			return /* @wc-include */ 'The field catalog is too long for the language model';
		case 'call.tooMany':
			return /* @wc-include */ 'Too many questions are waiting already';
		case 'call.waiting':
			return /* @wc-include */ 'Waiting for the questions before it to be answered …';
		case 'call.computing':
			return /* @wc-include */ 'Computing …';
		case 'call.loadingLLM':
			return /* @wc-include */ 'Loading the language model …';
		case 'call.reading':
			return /* @wc-include */ 'Reading the question …';
		case 'call.writing':
			return /* @wc-include */ 'Writing the answer …';
		case 'call.cancelled':
			return /* @wc-include */ 'Cancelled';
		case 'call.cancelledBefore':
			return /* @wc-include */ 'Cancelled before it was asked';
		case 'call.noJSON':
			return /* @wc-include */ 'The answer of the language model contains no JSON';

		// recording, live transcription
		case 'capture.failed':
			return /* @wc-include */ `Recording failed: ${a.reason}`;
		case 'capture.noDevice':
			return /* @wc-include */ 'Recording failed: no microphone found';
		case 'capture.micEnded':
			return /* @wc-include */ 'Recording failed: the microphone stopped delivering sound';
		case 'capture.noOutput':
			return /* @wc-include */ 'Recording failed: no output device found for the system audio';
		case 'capture.systemEnded':
			return /* @wc-include */ 'Recording failed: the system audio stopped delivering sound';
		case 'capture.noSystemOnMac':
			return /* @wc-include */ 'Recording failed: recording the system audio does not work on the Mac yet';
		case 'capture.systemOnly':
			return /* @wc-include */ 'A recording of the system audio alone is running. To dictate, first end that recording on the Live page.';
		case 'capture.noSound':
			return /* @wc-include */ 'No sound arrived before the recording ended, so nothing was saved.';
		case 'capture.format':
			return /* @wc-include */ `Audio: unknown format (${a.bits} bit)`;
		case 'capture.noChannels':
			return /* @wc-include */ 'Audio: format without channels';
		case 'live.noSuchNote':
			return /* @wc-include */ 'This note does not exist.';
		case 'live.noteRunning':
			return /* @wc-include */ 'The note is still running.';
		case 'live.alreadyRecording':
			return /* @wc-include */ 'The recording is already running.';
		case 'live.noOrigin':
			return /* @wc-include */ 'The address is missing.';
		case 'live.off':
			return /* @wc-include */ 'Transcribe is not recording the microphone for other programs right now: “Ready in the background” is off. It still processes audio and images that are sent to it. To record, turn it on in Transcribe under Settings › Apps.';
		case 'dictate.noTray':
			return /* @wc-include */ 'The shortcut only works while Transcribe runs with its icon in the notification area.';
		case 'dictate.keyTaken':
			return /* @wc-include */ 'Another program already uses this shortcut. Choose a different one.';

		// settings
		case 'autostart.failed':
			return /* @wc-include */ `Starting with Windows could not be changed: ${a.error}`;
		case 'prefs.cudaMissing':
			return /* @wc-include */ 'The GPU runtime has not been downloaded yet.';
		case 'prefs.language':
			return /* @wc-include */ 'Language: automatic, or codes such as de or de,en';
		case 'prefs.unknownService':
			return /* @wc-include */ `Unknown service: ${a.id}`;
		case 'prefs.unknownOCR':
			return /* @wc-include */ `Unknown text recognition: ${a.id}`;
		case 'gpu.none':
			return /* @wc-include */ 'No supported NVIDIA GPU found.';
		case 'gpu.missingLibs':
			return /* @wc-include */ `NVIDIA libraries are missing that the app cannot download itself: ${a.libs}`;
		case 'gpu.noAutoLibs':
			return /* @wc-include */ 'The NVIDIA libraries cannot be downloaded automatically on this system.';
		case 'update.devBuild':
			return /* @wc-include */ 'A development build does not look for updates.';
		case 'update.unreachable':
			return /* @wc-include */ 'GitHub could not be reached. Please try again later.';
		case 'update.none':
			return /* @wc-include */ 'There is no newer version.';
		case 'update.noPermission':
			return /* @wc-include */ 'Transcribe may not change its program folder. Download the new version from the releases page and replace the EXE by hand.';
		case 'update.tooSlow':
			return /* @wc-include */ 'The download took too long. Please try again later.';
		case 'update.failed':
			return /* @wc-include */ `The update could not be installed: ${a.error}`;
	}
	return null;
}

/**
 * The name of a component the app downloads (runtime, ffmpeg, a model), as the settings page and
 * the first-time setup show it; sonst (the server's German label) for one this file does not know.
 * @param {string} name @param {string} [sonst]
 * @returns {string}
 */
export function komponente(name, sonst = name) {
	switch (name) {
		case 'runtime':
			return /* @wc-include */ 'Recognition runtime (sherpa-onnx)';
		case 'silero':
			return /* @wc-include */ 'Voice activity (Silero VAD)';
		case 'segmentation':
			return /* @wc-include */ 'Speaker separation (pyannote)';
		case 'campplus':
			return /* @wc-include */ 'Speaker embeddings (CAM++)';
		case 'ced':
			return /* @wc-include */ 'Sound recognition (CED)';
		case 'parakeet':
			return /* @wc-include */ 'Speech recognition (Parakeet v3)';
		case 'german':
			return /* @wc-include */ 'German model (FastConformer)';
		case 'kroko':
			return /* @wc-include */ 'German live (Kroko)';
		case 'whisper-base':
			return /* @wc-include */ 'Speech recognition (Whisper base)';
		case 'whisper-turbo':
			return /* @wc-include */ 'Other languages (Whisper turbo)';
		case 'runtime-cuda':
			return /* @wc-include */ 'GPU runtime (CUDA)';
		case 'nvidia':
			return /* @wc-include */ 'NVIDIA libraries (CUDA, cuDNN)';
		case 'ffmpeg':
			return /* @wc-include */ 'ffmpeg (audio/video)';
		case 'tesseract':
			return /* @wc-include */ 'Text recognition (Tesseract)';
		case 'tesseract-deu':
			return /* @wc-include */ 'Text recognition German';
		case 'tesseract-eng':
			return /* @wc-include */ 'Text recognition English';
		case 'tesseract-frk':
			return /* @wc-include */ 'Text recognition Fraktur (frk)';
		case 'tesseract-Fraktur':
			return /* @wc-include */ 'Text recognition Fraktur script';
		case 'tesseract-frak2021':
			return /* @wc-include */ 'Text recognition Fraktur (frak2021)';
		case 'ppocr':
			return /* @wc-include */ 'Text recognition (PP-OCRv5)';
		case 'embed':
			return /* @wc-include */ 'Search by meaning (EmbeddingGemma)';
		case 'llm':
			return /* @wc-include */ 'Filter in your own words (Qwen 2.5 7B)';
	}
	return sonst;
}

/**
 * The step a transcription is at (jobs.Progress: "analyze events" …), for the library.
 * @param {string} step
 * @returns {string}
 */
export function schritt(step) {
	const api = /^transcribe \((.+)\)$/.exec(step);
	if (api) return /* @wc-include */ `transcribing (${api[1]})`;
	switch (step) {
		case 'Start':
			return /* @wc-include */ 'starting';
		case 'analyze events':
			return /* @wc-include */ 'finding sounds';
		case 'analyze speakers':
			return /* @wc-include */ 'telling speakers apart';
		case 'transcribe':
			return /* @wc-include */ 'transcribing';
		case 'annotate':
			return /* @wc-include */ 'putting it together';
	}
	return step;
}

/**
 * The name of a service one can choose in the settings (internal/sttapi and internal/ocrapi:
 * Providers): a brand as it is, one's own server in the interface language.
 * @param {{ id: string, label: string }} d @param {'stt' | 'ocr'} wofuer
 * @returns {string}
 */
export function dienstName(d, wofuer) {
	if (d.id !== 'custom') return d.label;
	return wofuer === 'ocr'
		? /* @wc-include */ 'Your own server (OpenAI-compatible, such as Ollama)'
		: /* @wc-include */ 'Your own server (OpenAI-compatible)';
}

/**
 * The name of a text recognition engine (GET /api/ocr/engines, and the model of a text recognition
 * request on the Live page), with the server's German names in the interface language: the
 * Windows engine (internal/osocr) and the AI service (internal/ocrapi: "KI-Dienst (OpenAI)").
 * @param {string} name
 * @returns {string}
 */
export function leserName(name) {
	const win = /^Windows-Texterkennung(.*)$/.exec(name);
	if (win) return /* @wc-include */ `Windows text recognition${win[1]}`;
	const ki = /^KI-Dienst \((.*)\)(.*)$/.exec(name);
	if (!ki) return name;
	const dienst = /^Eigener Server/.test(ki[1]) ? dienstName({ id: 'custom', label: ki[1] }, 'ocr') : ki[1];
	return /* @wc-include */ `AI service (${dienst})${ki[2]}`;
}

/**
 * The name of a shortcut for dictating (prefs.DictateKeys), as the keyboard of the interface
 * language labels the keys; sonst for one this file does not know.
 * @param {string} key @param {string} [sonst]
 * @returns {string}
 */
export function tastenkuerzel(key, sonst = key) {
	switch (key) {
		case 'ctrl+alt+space':
			return /* @wc-include */ 'Ctrl + Alt + Space';
		case 'ctrl+shift+space':
			return /* @wc-include */ 'Ctrl + Shift + Space';
		case 'ctrl+alt+d':
			return /* @wc-include */ 'Ctrl + Alt + D';
	}
	return sonst;
}
