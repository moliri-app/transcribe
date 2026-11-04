// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Meldet seitenübergreifend, dass sich Aufnahmen oder Aufträge geändert haben (etwa nach dem
// Ablegen von Dateien im Fenster), damit App-Leiste und Bibliothek sofort neu laden statt
// erst beim nächsten Abfragen.
export const auftraege = $state({ stand: 0 });

export function geaendert() {
	auftraege.stand++;
}
