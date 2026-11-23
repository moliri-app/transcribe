// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"strconv"
	"strings"

	appmsg "github.com/moliri-app/transcribe/internal/msg"
)

// When a dictation fails, the window says why: an error of the recording or of the speech
// recognition service, which internal/live and internal/sttapi make as messages (internal/msg). The
// pages turn those into text with frontend/src/lib/meldungen.js and the catalogues of
// frontend/src/locales; the EXE has no JavaScript, so the few the dictation can end with have their
// texts here, the same as the frontend's: TestMessagesAgreeWithTheFrontend compares them with
// meldungen.js and the .po catalogues, TestDictationErrorsTranslated finds every message the
// recording and the services make. Anything else keeps the German text the error carries.

// message is the text of a message in English, its placeholders {name} filled from its arguments,
// and its translations by locale.
type message struct {
	en string
	// detail: what the service itself said (the argument detail) follows the text, as meldungen.js
	// does with mit()
	detail bool
	tr     map[string]string
}

// errorText is err as the dictation window shows it, in the interface language.
func errorText(err error) string {
	return errorTextIn(uiLocale(), err)
}

func errorTextIn(locale string, err error) string {
	if m := appmsg.Of(err); m != nil {
		return messageIn(locale, m)
	}
	return err.Error()
}

// messageIn is m in locale; the German text for a message the catalogue does not know.
func messageIn(locale string, m *appmsg.M) string {
	t, ok := messages[m.Key]
	if !ok {
		return m.Error()
	}
	s := t.en
	if tr := t.tr[locale]; tr != "" {
		s = tr
	}
	for name, v := range m.Args {
		if name == "detail" && t.detail {
			continue
		}
		s = strings.ReplaceAll(s, "{"+name+"}", argIn(locale, v))
	}
	if t.detail {
		s = strings.TrimSpace(s + " " + argIn(locale, m.Args["detail"]))
	}
	return s
}

// argIn is an argument of a message as its text in locale shows it.
func argIn(locale string, v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case *appmsg.M:
		return messageIn(locale, x)
	case float64:
		s := strconv.FormatFloat(x, 'f', -1, 64)
		if locale != "en" && locale != "ja" {
			s = strings.Replace(s, ".", ",", 1) // the others write a decimal comma
		}
		return s
	case float32:
		return argIn(locale, float64(x))
	}
	return fmt.Sprint(v)
}

// messages are the texts of the messages a dictation can end with, by key.
var messages = map[string]message{
	// the recording
	"capture.failed": {en: "Recording failed: {reason}", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: {reason}",
		"es": "La grabación falló: {reason}",
		"fr": "L’enregistrement a échoué : {reason}",
		"ja": "録音できませんでした: {reason}",
		"uk": "Запис не вдався: {reason}",
		"el": "Η εγγραφή απέτυχε: {reason}",
	}},
	"capture.format": {en: "Audio: unknown format ({bits} bit)", tr: map[string]string{
		"de": "Audio: unbekanntes Format ({bits} Bit)",
		"es": "Audio: formato desconocido ({bits} bits)",
		"fr": "Audio : format inconnu ({bits} bits)",
		"ja": "オーディオ: 不明な形式（{bits} ビット）",
		"uk": "Аудіо: невідомий формат ({bits} біт)",
		"el": "Ήχος: άγνωστη μορφή ({bits} bit)",
	}},
	"capture.micEnded": {en: "Recording failed: the microphone stopped delivering sound", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: das Mikrofon hat aufgehört zu liefern",
		"es": "La grabación falló: el micrófono dejó de enviar sonido",
		"fr": "L’enregistrement a échoué : le microphone ne fournit plus de son",
		"ja": "録音できませんでした: マイクからの音が途絶えました",
		"uk": "Запис не вдався: мікрофон перестав передавати звук",
		"el": "Η εγγραφή απέτυχε: το μικρόφωνο σταμάτησε να στέλνει ήχο",
	}},
	"capture.noChannels": {en: "Audio: format without channels", tr: map[string]string{
		"de": "Audio: Format ohne Kanäle",
		"es": "Audio: formato sin canales",
		"fr": "Audio : format sans canaux",
		"ja": "オーディオ: チャンネルのない形式",
		"uk": "Аудіо: формат без каналів",
		"el": "Ήχος: μορφή χωρίς κανάλια",
	}},
	"capture.noDevice": {en: "Recording failed: no microphone found", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: kein Mikrofon gefunden",
		"es": "La grabación falló: no se encontró ningún micrófono",
		"fr": "L’enregistrement a échoué : aucun microphone trouvé",
		"ja": "録音できませんでした: マイクが見つかりません",
		"uk": "Запис не вдався: мікрофон не знайдено",
		"el": "Η εγγραφή απέτυχε: δεν βρέθηκε μικρόφωνο",
	}},
	"capture.noOutput": {en: "Recording failed: no output device found for the system audio", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: kein Ausgabegerät für das Systemaudio gefunden",
		"es": "La grabación falló: no se encontró ningún dispositivo de salida para el audio del sistema",
		"fr": "L’enregistrement a échoué : aucun périphérique de sortie trouvé pour l’audio système",
		"ja": "録音できませんでした: システム音声の出力デバイスが見つかりません",
		"uk": "Запис не вдався: не знайдено пристрою виводу для системного звуку",
		"el": "Η εγγραφή απέτυχε: δεν βρέθηκε συσκευή εξόδου για τον ήχο συστήματος",
	}},
	"capture.noSound": {en: "No sound arrived before the recording ended, so nothing was saved.", tr: map[string]string{
		"de": "Es kam kein Ton an, bevor die Aufnahme beendet wurde, darum ist nichts gespeichert.",
		"es": "No llegó ningún sonido antes de que terminara la grabación, así que no se guardó nada.",
		"fr": "Aucun son n’est arrivé avant la fin de l’enregistrement, rien n’a donc été enregistré.",
		"ja": "録音が終わるまでに音が届かなかったため、何も保存されていません。",
		"uk": "До завершення запису не надійшло жодного звуку, тому нічого не збережено.",
		"el": "Δεν έφτασε ήχος πριν τελειώσει η εγγραφή, οπότε δεν αποθηκεύτηκε τίποτα.",
	}},
	"capture.noSystemOnMac": {en: "Recording failed: recording the system audio does not work on the Mac yet", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: Systemaudio aufzunehmen geht auf dem Mac noch nicht",
		"es": "La grabación falló: grabar el audio del sistema aún no funciona en Mac",
		"fr": "L’enregistrement a échoué : l’enregistrement de l’audio système ne fonctionne pas encore sur Mac",
		"ja": "録音できませんでした: Mac ではまだシステム音声を録音できません",
		"uk": "Запис не вдався: на Mac запис системного звуку ще не працює",
		"el": "Η εγγραφή απέτυχε: η εγγραφή του ήχου συστήματος δεν λειτουργεί ακόμη σε Mac",
	}},
	"capture.systemEnded": {en: "Recording failed: the system audio stopped delivering sound", tr: map[string]string{
		"de": "Aufnahme fehlgeschlagen: das Systemaudio hat aufgehört zu liefern",
		"es": "La grabación falló: el audio del sistema dejó de enviar sonido",
		"fr": "L’enregistrement a échoué : l’audio système ne fournit plus de son",
		"ja": "録音できませんでした: システム音声が途絶えました",
		"uk": "Запис не вдався: системний звук перестав надходити",
		"el": "Η εγγραφή απέτυχε: ο ήχος συστήματος σταμάτησε",
	}},
	"capture.systemOnly": {en: "A recording of the system audio alone is running. To dictate, first end that recording on the Live page.", tr: map[string]string{
		"de": "Gerade läuft eine Aufnahme nur vom Systemaudio. Zum Diktieren beende erst diese Aufnahme auf der Seite Live.",
		"es": "Hay en curso una grabación solo del audio del sistema. Para dictar, termina primero esa grabación en la página Live.",
		"fr": "Un enregistrement de l’audio système seul est en cours. Pour dicter, terminez d’abord cet enregistrement sur la page Live.",
		"ja": "システム音声のみの録音が進行中です。音声入力するには、先に Live ページでその録音を終了してください。",
		"uk": "Зараз триває запис лише системного звуку. Щоб диктувати, спершу завершіть цей запис на сторінці Live.",
		"el": "Γίνεται εγγραφή μόνο του ήχου συστήματος. Για υπαγόρευση, τερματίστε πρώτα αυτή την εγγραφή στη σελίδα Live.",
	}},

	// the speech recognition services
	"api.badAnswer": {en: "{service}: unexpected answer: {error}", tr: map[string]string{
		"de": "{service}: unerwartete Antwort: {error}",
		"es": "{service}: respuesta inesperada: {error}",
		"fr": "{service} : réponse inattendue : {error}",
		"ja": "{service}: 予期しない応答: {error}",
		"uk": "{service}: неочікувана відповідь: {error}",
		"el": "{service}: απρόσμενη απάντηση: {error}",
	}},
	"api.error": {en: "{service}: error {status}:", detail: true, tr: map[string]string{
		"de": "{service}: Fehler {status}:",
		"es": "{service}: error {status}:",
		"fr": "{service} : erreur {status} :",
		"ja": "{service}: エラー {status}:",
		"uk": "{service}: помилка {status}:",
		"el": "{service}: σφάλμα {status}:",
	}},
	"api.exhausted": {en: "{service} is not accepting anything right now (credit or requests used up).", detail: true, tr: map[string]string{
		"de": "{service} nimmt gerade nichts an (Guthaben oder Anfragen aufgebraucht).",
		"es": "{service} no acepta nada en este momento (crédito o solicitudes agotados).",
		"fr": "{service} n’accepte rien pour le moment (crédit ou requêtes épuisés).",
		"ja": "{service} は現在何も受け付けていません（残高またはリクエスト数の上限）。",
		"uk": "{service} зараз нічого не приймає (вичерпано кошти або запити).",
		"el": "Το {service} δεν δέχεται τίποτα αυτή τη στιγμή (εξαντλήθηκε το υπόλοιπο ή τα αιτήματα).",
	}},
	"api.keyNotAccepted": {en: "The API key was not accepted.", tr: map[string]string{
		"de": "Der API-Schlüssel wurde nicht angenommen.",
		"es": "La clave de API no fue aceptada.",
		"fr": "La clé d’API n’a pas été acceptée.",
		"ja": "API キーが受け付けられませんでした。",
		"uk": "Ключ API не прийнято.",
		"el": "Το κλειδί API δεν έγινε δεκτό.",
	}},
	"api.keyRejected": {en: "{service} rejected the API key.", detail: true, tr: map[string]string{
		"de": "{service} hat den API-Schlüssel abgelehnt.",
		"es": "{service} ha rechazado la clave de API.",
		"fr": "{service} a refusé la clé d’API.",
		"ja": "{service} が API キーを拒否しました。",
		"uk": "{service} відхилив ключ API.",
		"el": "Το {service} απέρριψε το κλειδί API.",
	}},
	"api.noAnswer": {en: "{service} does not answer: {error}", tr: map[string]string{
		"de": "{service} antwortet nicht: {error}",
		"es": "{service} no responde: {error}",
		"fr": "{service} ne répond pas : {error}",
		"ja": "{service} が応答しません: {error}",
		"uk": "{service} не відповідає: {error}",
		"el": "Το {service} δεν απαντά: {error}",
	}},
	"api.noKey": {en: "No key for the API has been entered yet (Settings, Recognition).", tr: map[string]string{
		"de": "Für die API ist noch kein Schlüssel hinterlegt (Einstellungen, Erkennung).",
		"es": "Todavía no se ha introducido ninguna clave para la API (Configuración, Reconocimiento).",
		"fr": "Aucune clé n’a encore été saisie pour l’API (Paramètres, Reconnaissance).",
		"ja": "API のキーがまだ入力されていません（設定 › 認識）。",
		"uk": "Ключ для API ще не вказано (Налаштування, Розпізнавання).",
		"el": "Δεν έχει καταχωριστεί ακόμη κλειδί για το API (Ρυθμίσεις, Αναγνώριση).",
	}},
	"api.noURL": {en: "The address of the API is missing (Settings, Recognition).", tr: map[string]string{
		"de": "Für die API fehlt die Adresse (Einstellungen, Erkennung).",
		"es": "Falta la dirección de la API (Configuración, Reconocimiento).",
		"fr": "L’adresse de l’API manque (Paramètres, Reconnaissance).",
		"ja": "API のアドレスがありません（設定 › 認識）。",
		"uk": "Не вказано адресу API (Налаштування, Розпізнавання).",
		"el": "Λείπει η διεύθυνση του API (Ρυθμίσεις, Αναγνώριση).",
	}},
	"api.quota": {en: "Credit or requests are used up.", tr: map[string]string{
		"de": "Guthaben oder Anfragen sind aufgebraucht.",
		"es": "El crédito o las solicitudes se han agotado.",
		"fr": "Le crédit ou les requêtes sont épuisés.",
		"ja": "残高またはリクエスト数が上限に達しました。",
		"uk": "Кошти або запити вичерпано.",
		"el": "Το υπόλοιπο ή τα αιτήματα εξαντλήθηκαν.",
	}},
	"api.refused": {en: "{service} refuses: {reason}", tr: map[string]string{
		"de": "{service} lehnt ab: {reason}",
		"es": "{service} lo rechaza: {reason}",
		"fr": "{service} refuse : {reason}",
		"ja": "{service} が拒否しました: {reason}",
		"uk": "{service} відмовляє: {reason}",
		"el": "Το {service} αρνείται: {reason}",
	}},
	"api.unreachable": {en: "{service} cannot be reached: {error}", tr: map[string]string{
		"de": "{service} ist nicht erreichbar: {error}",
		"es": "No se puede acceder a {service}: {error}",
		"fr": "{service} est injoignable : {error}",
		"ja": "{service} に接続できません: {error}",
		"uk": "{service} недоступний: {error}",
		"el": "Το {service} δεν είναι προσβάσιμο: {error}",
	}},
}
