// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/moliri-app/transcribe/internal/prefs"
)

// What the EXE shows itself, apart from the pages (the tray menu, notifications, the dictation
// window, message boxes), speaks the interface language too: the one the pages last reported
// (prefs.UILocale, sent with their heartbeat), else the system's, else English. The pages
// translate with wuchale (frontend/src/locales); these few texts have their catalogue here.
// tx("English text", args...) is the text in the interface language; a new one needs its line in
// texts, which TestEveryTextTranslated checks.

var (
	localeMu   sync.Mutex
	localeData string // the data dir whose settings hold the interface language; "" before it is known
)

// useLocaleOf takes the interface language from the settings in dataDir from now on.
func useLocaleOf(dataDir string) {
	localeMu.Lock()
	localeData = dataDir
	localeMu.Unlock()
}

// uiLocale is the interface language: one of prefs.UILocales.
func uiLocale() string {
	localeMu.Lock()
	d := localeData
	localeMu.Unlock()
	if d != "" {
		if l := prefs.Load(d).UILocale; l != "" {
			return l
		}
	}
	if l := prefs.UILocale(systemLocale()); l != "" {
		return l
	}
	return "en"
}

// tx is en in the interface language, its verbs (%s, %d) filled from args like fmt.Sprintf.
func tx(en string, args ...any) string {
	return txIn(uiLocale(), en, args...)
}

func txIn(locale, en string, args ...any) string {
	s := en
	if t := texts[en][locale]; t != "" {
		s = t
	}
	if len(args) > 0 {
		s = fmt.Sprintf(s, args...)
	}
	return s
}

// keyLabel is a shortcut of prefs.DictateKeys ("ctrl+alt+space") as the keyboard of the interface
// language labels its keys ("Strg + Alt + Leertaste").
func keyLabel(key string) string {
	parts := strings.Split(key, "+")
	for i, p := range parts {
		switch p {
		case "ctrl":
			parts[i] = tx("Ctrl")
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = tx("Shift")
		case "space":
			parts[i] = tx("Space")
		default:
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, " + ")
}

// texts are the translations of what the EXE shows itself, by the English text.
var texts = map[string]map[string]string{
	// the tray menu
	"Open Transcribe": {
		"de": "Transcribe öffnen", "es": "Abrir Transcribe", "fr": "Ouvrir Transcribe",
		"ja": "Transcribe を開く", "uk": "Відкрити Transcribe", "el": "Άνοιγμα Transcribe",
	},
	"Settings": {
		"de": "Einstellungen", "es": "Configuración", "fr": "Paramètres",
		"ja": "設定", "uk": "Налаштування", "el": "Ρυθμίσεις",
	},
	"Quit": {
		"de": "Beenden", "es": "Salir", "fr": "Quitter",
		"ja": "終了", "uk": "Вийти", "el": "Έξοδος",
	},
	"Update to version %s": {
		"de": "Auf Version %s aktualisieren", "es": "Actualizar a la versión %s", "fr": "Mettre à jour vers la version %s",
		"ja": "バージョン %s に更新", "uk": "Оновити до версії %s", "el": "Ενημέρωση στην έκδοση %s",
	},
	"Downloading version %s … %d %%": {
		"de": "Version %s wird geladen … %d %%", "es": "Descargando la versión %s … %d %%",
		"fr": "Téléchargement de la version %s … %d %%", "ja": "バージョン %s をダウンロード中 … %d %%",
		"uk": "Завантаження версії %s … %d %%", "el": "Λήψη της έκδοσης %s … %d %%",
	},
	"The update starts once jobs and recordings are done": {
		"de": "Update startet, sobald Aufträge und Aufnahmen fertig sind",
		"es": "La actualización empieza cuando terminen los trabajos y las grabaciones",
		"fr": "La mise à jour démarre dès que les tâches et les enregistrements sont terminés",
		"ja": "ジョブと録音が終わると更新が始まります",
		"uk": "Оновлення почнеться, щойно завершаться завдання й записи",
		"el": "Η ενημέρωση ξεκινά μόλις ολοκληρωθούν οι εργασίες και οι ηχογραφήσεις",
	},

	// notifications
	"Transcribe %s is available": {
		"de": "Transcribe %s ist da", "es": "Ya está disponible Transcribe %s", "fr": "Transcribe %s est disponible",
		"ja": "Transcribe %s が利用可能です", "uk": "Доступна нова версія Transcribe %s", "el": "Το Transcribe %s είναι διαθέσιμο",
	},
	"A click on “Update” in the icon's menu or in the settings installs the new version.": {
		"de": "Ein Klick auf „Aktualisieren“ im Menü des Symbols oder in den Einstellungen installiert die neue Version.",
		"es": "Haz clic en «Actualizar» en el menú del icono o en la configuración para instalar la nueva versión.",
		"fr": "Cliquez sur « Mettre à jour » dans le menu de l'icône ou dans les paramètres pour installer la nouvelle version.",
		"ja": "アイコンのメニューまたは設定で「更新」をクリックすると、新しいバージョンがインストールされます。",
		"uk": "Натисніть «Оновити» в меню значка або в налаштуваннях, щоб установити нову версію.",
		"el": "Πατήστε «Ενημέρωση» στο μενού του εικονιδίου ή στις ρυθμίσεις για να εγκαταστήσετε τη νέα έκδοση.",
	},
	"May %s use Transcribe?": {
		"de": "Darf %s Transcribe nutzen?", "es": "¿Puede %s usar Transcribe?", "fr": "%s peut-il utiliser Transcribe ?",
		"ja": "%s が Transcribe を使ってもよいですか?", "uk": "Дозволити %s користуватися Transcribe?",
		"el": "Να επιτραπεί στο %s να χρησιμοποιεί το Transcribe;",
	},
	"May a file or an embedded page use Transcribe?": {
		"de": "Darf eine Datei oder eingebettete Seite Transcribe nutzen?",
		"es": "¿Puede un archivo o una página incrustada usar Transcribe?",
		"fr": "Un fichier ou une page intégrée peut-il utiliser Transcribe ?",
		"ja": "ファイルまたは埋め込みページが Transcribe を使ってもよいですか?",
		"uk": "Дозволити файлу або вбудованій сторінці користуватися Transcribe?",
		"el": "Να επιτραπεί σε ένα αρχείο ή μια ενσωματωμένη σελίδα να χρησιμοποιεί το Transcribe;",
	},
	"The page wants to turn speech into text or have text read from images. Click here to see the request.": {
		"de": "Die Seite möchte Sprache als Text holen oder Text aus Bildern lesen lassen. Ein Klick hier zeigt die Anfrage.",
		"es": "La página quiere convertir voz en texto o leer texto de imágenes. Haz clic aquí para ver la solicitud.",
		"fr": "La page souhaite convertir la parole en texte ou lire le texte d'images. Cliquez ici pour voir la demande.",
		"ja": "このページは音声を文字にするか、画像から文字を読み取ろうとしています。ここをクリックすると要求を表示します。",
		"uk": "Сторінка хоче перетворити мовлення на текст або розпізнати текст на зображеннях. Натисніть тут, щоб переглянути запит.",
		"el": "Η σελίδα θέλει να μετατρέψει ομιλία σε κείμενο ή να διαβάσει κείμενο από εικόνες. Πατήστε εδώ για να δείτε το αίτημα.",
	},
	"Transcribe keeps running in the background": {
		"de": "Transcribe läuft im Hintergrund weiter", "es": "Transcribe sigue ejecutándose en segundo plano",
		"fr": "Transcribe continue de fonctionner en arrière-plan", "ja": "Transcribe はバックグラウンドで動作し続けます",
		"uk": "Transcribe працює далі у фоновому режимі", "el": "Το Transcribe συνεχίζει να εκτελείται στο παρασκήνιο",
	},
	"Jobs and live requests carry on. Click the icon to open the window again.": {
		"de": "Aufträge und Live-Anfragen laufen weiter. Ein Klick auf das Symbol öffnet das Fenster wieder.",
		"es": "Los trabajos y las solicitudes en directo continúan. Haz clic en el icono para volver a abrir la ventana.",
		"fr": "Les tâches et les demandes en direct continuent. Cliquez sur l'icône pour rouvrir la fenêtre.",
		"ja": "ジョブとライブ要求は続行します。アイコンをクリックするとウィンドウが再び開きます。",
		"uk": "Завдання та живі запити тривають. Натисніть значок, щоб знову відкрити вікно.",
		"el": "Οι εργασίες και τα ζωντανά αιτήματα συνεχίζονται. Πατήστε το εικονίδιο για να ανοίξει ξανά το παράθυρο.",
	},

	// message boxes
	"The data folder %s cannot be reached. Is the drive connected?\n\n“Yes” starts Transcribe with the default folder %s, “No” quits it.": {
		"de": "Der Datenordner %s ist nicht erreichbar. Ist das Laufwerk angeschlossen?\n\n„Ja“ startet Transcribe mit dem Standardordner %s, „Nein“ beendet es.",
		"es": "No se puede acceder a la carpeta de datos %s. ¿Está conectada la unidad?\n\n«Sí» inicia Transcribe con la carpeta predeterminada %s; «No» lo cierra.",
		"fr": "Le dossier de données %s est inaccessible. Le lecteur est-il connecté ?\n\n« Oui » démarre Transcribe avec le dossier par défaut %s, « Non » le ferme.",
		"ja": "データフォルダー %s にアクセスできません。ドライブは接続されていますか?\n\n「はい」で既定のフォルダー %s を使って Transcribe を起動し、「いいえ」で終了します。",
		"uk": "Папка даних %s недоступна. Чи під’єднано диск?\n\n«Так» запускає Transcribe зі стандартною папкою %s, «Ні» завершує роботу.",
		"el": "Ο φάκελος δεδομένων %s δεν είναι προσβάσιμος. Είναι συνδεδεμένη η μονάδα δίσκου;\n\nΤο «Ναι» ξεκινά το Transcribe με τον προεπιλεγμένο φάκελο %s, το «Όχι» το τερματίζει.",
	},
	"Error: %s": {
		"de": "Fehler: %s", "es": "Error: %s", "fr": "Erreur : %s",
		"ja": "エラー: %s", "uk": "Помилка: %s", "el": "Σφάλμα: %s",
	},
	"The new version could not be started (%s)": {
		"de": "Die neue Version ließ sich nicht starten (%s)", "es": "No se pudo iniciar la nueva versión (%s)",
		"fr": "Impossible de démarrer la nouvelle version (%s)", "ja": "新しいバージョンを起動できませんでした (%s)",
		"uk": "Не вдалося запустити нову версію (%s)", "el": "Δεν ήταν δυνατή η εκκίνηση της νέας έκδοσης (%s)",
	},

	// the dictation window
	"Getting ready to dictate …": {
		"de": "Diktat wird vorbereitet …", "es": "Preparando el dictado …", "fr": "Préparation de la dictée …",
		"ja": "音声入力を準備しています …", "uk": "Підготовка диктування …", "el": "Προετοιμασία υπαγόρευσης …",
	},
	"One moment, you can speak shortly.": {
		"de": "Einen Moment, gleich kannst du sprechen.", "es": "Un momento, enseguida podrás hablar.",
		"fr": "Un instant, vous pourrez bientôt parler.", "ja": "少々お待ちください。まもなく話せます。",
		"uk": "Хвилинку, зараз можна буде говорити.", "el": "Μια στιγμή, σε λίγο μπορείτε να μιλήσετε.",
	},
	"Dictation did not work": {
		"de": "Diktieren ging nicht", "es": "El dictado no funcionó", "fr": "La dictée n'a pas fonctionné",
		"ja": "音声入力できませんでした", "uk": "Диктування не вдалося", "el": "Η υπαγόρευση απέτυχε",
	},
	"I'm listening": {
		"de": "Ich höre zu", "es": "Te escucho", "fr": "Je vous écoute",
		"ja": "聞いています", "uk": "Слухаю", "el": "Σας ακούω",
	},
	"%s: done   Esc: cancel": {
		"de": "%s: fertig   Esc: abbrechen", "es": "%s: terminar   Esc: cancelar", "fr": "%s : terminer   Échap : annuler",
		"ja": "%s: 完了   Esc: キャンセル", "uk": "%s: готово   Esc: скасувати", "el": "%s: τέλος   Esc: ακύρωση",
	},
	"Microphone starting …": {
		"de": "Mikrofon startet …", "es": "Iniciando el micrófono …", "fr": "Démarrage du microphone …",
		"ja": "マイクを起動しています …", "uk": "Запуск мікрофона …", "el": "Εκκίνηση μικροφώνου …",
	},
	"You can speak in a moment.": {
		"de": "Gleich kannst du sprechen.", "es": "Enseguida podrás hablar.", "fr": "Vous pourrez parler dans un instant.",
		"ja": "まもなく話せます。", "uk": "Зараз можна буде говорити.", "el": "Σε λίγο μπορείτε να μιλήσετε.",
	},
	"Speak now.": {
		"de": "Sprich jetzt.", "es": "Habla ahora.", "fr": "Parlez maintenant.",
		"ja": "話してください。", "uk": "Говоріть.", "el": "Μιλήστε τώρα.",
	},
	"One moment …": {
		"de": "Einen Moment …", "es": "Un momento …", "fr": "Un instant …",
		"ja": "少々お待ちください …", "uk": "Хвилинку …", "el": "Μια στιγμή …",
	},
	"The last words are still being recognised.": {
		"de": "Das zuletzt Gesagte wird noch erkannt.", "es": "Todavía se está reconociendo lo último que dijiste.",
		"fr": "Les derniers mots sont encore en cours de reconnaissance.", "ja": "最後に話した内容を認識しています。",
		"uk": "Останні слова ще розпізнаються.", "el": "Οι τελευταίες λέξεις αναγνωρίζονται ακόμη.",
	},
	"Dictation cancelled": {
		"de": "Diktat abgebrochen", "es": "Dictado cancelado", "fr": "Dictée annulée",
		"ja": "音声入力を中止しました", "uk": "Диктування скасовано", "el": "Η υπαγόρευση ακυρώθηκε",
	},
	"Nothing was inserted.": {
		"de": "Es wurde nichts eingefügt.", "es": "No se insertó nada.", "fr": "Rien n'a été inséré.",
		"ja": "何も挿入されませんでした。", "uk": "Нічого не вставлено.", "el": "Δεν εισήχθη τίποτα.",
	},
	"Nothing understood": {
		"de": "Nichts verstanden", "es": "No se entendió nada", "fr": "Rien n'a été compris",
		"ja": "聞き取れませんでした", "uk": "Нічого не розпізнано", "el": "Δεν αναγνωρίστηκε τίποτα",
	},
	"No text came through. %s starts a new dictation.": {
		"de": "Es kam kein Text an. %s startet ein neues Diktat.", "es": "No llegó ningún texto. %s inicia un nuevo dictado.",
		"fr": "Aucun texte n'est arrivé. %s lance une nouvelle dictée.", "ja": "テキストが届きませんでした。%s で新しい音声入力を始めます。",
		"uk": "Текст не надійшов. %s починає нове диктування.", "el": "Δεν ήρθε κείμενο. Το %s ξεκινά νέα υπαγόρευση.",
	},
	"Clipboard did not work": {
		"de": "Zwischenablage ging nicht", "es": "El portapapeles no funcionó", "fr": "Le presse-papiers n'a pas fonctionné",
		"ja": "クリップボードを使えませんでした", "uk": "Буфер обміну не спрацював", "el": "Το πρόχειρο δεν λειτούργησε",
	},
	"Another program is using the clipboard.": {
		"de": "Die Zwischenablage ist von einem anderen Programm belegt.", "es": "Otro programa está usando el portapapeles.",
		"fr": "Un autre programme utilise le presse-papiers.", "ja": "別のプログラムがクリップボードを使用しています。",
		"uk": "Буфер обміну зайнятий іншою програмою.", "el": "Ένα άλλο πρόγραμμα χρησιμοποιεί το πρόχειρο.",
	},
	"Inserted": {
		"de": "Eingefügt", "es": "Insertado", "fr": "Inséré",
		"ja": "挿入しました", "uk": "Вставлено", "el": "Εισήχθη",
	},
	"also on the clipboard": {
		"de": "liegt auch in der Zwischenablage", "es": "también está en el portapapeles", "fr": "aussi dans le presse-papiers",
		"ja": "クリップボードにもコピーしました", "uk": "також у буфері обміну", "el": "επίσης στο πρόχειρο",
	},
	"On the clipboard": {
		"de": "In der Zwischenablage", "es": "En el portapapeles", "fr": "Dans le presse-papiers",
		"ja": "クリップボードにコピーしました", "uk": "У буфері обміну", "el": "Στο πρόχειρο",
	},
	"%s: paste": {
		"de": "%s: einfügen", "es": "%s: pegar", "fr": "%s : coller",
		"ja": "%s: 貼り付け", "uk": "%s: вставити", "el": "%s: επικόλληση",
	},
	"Saved on the Live page": {
		"de": "Auf der Seite Live gespeichert", "es": "Guardado en la página Live", "fr": "Enregistré sur la page Live",
		"ja": "Live ページに保存しました", "uk": "Збережено на сторінці Live", "el": "Αποθηκεύτηκε στη σελίδα Live",
	},
	"Click: open it there": {
		"de": "Klick: dort öffnen", "es": "Clic: abrirlo allí", "fr": "Clic : l'ouvrir là-bas",
		"ja": "クリック: そこで開く", "uk": "Клік: відкрити там", "el": "Κλικ: άνοιγμα εκεί",
	},

	// keys, as keyboards of the language label them
	"Ctrl": {
		"de": "Strg", "es": "Ctrl", "fr": "Ctrl", "ja": "Ctrl", "uk": "Ctrl", "el": "Ctrl",
	},
	"Shift": {
		"de": "Umschalt", "es": "Mayús", "fr": "Maj", "ja": "Shift", "uk": "Shift", "el": "Shift",
	},
	"Space": {
		"de": "Leertaste", "es": "Espacio", "fr": "Espace", "ja": "スペース", "uk": "Пробіл", "el": "Διάστημα",
	},
}
