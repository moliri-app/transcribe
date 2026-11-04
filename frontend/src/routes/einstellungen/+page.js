// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// /einstellungen ist ein zweiter Name der Einstellungen (internal/server/pages.go).
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(307, '/settings');
}
