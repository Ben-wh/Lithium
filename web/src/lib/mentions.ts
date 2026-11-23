// Mentions are stored in comment text as @[Display Name](userID) for a person
// or @[Group Name](g:groupID) for a group. In the reply box people see plain
// "@Name"; tokens are swapped in on send.

export type Segment = { text: string } | { mention: string; id: number; group: boolean };

const TOKEN = /@\[([^\]\n]{1,100})\]\((g:)?(\d+)\)/g;

/** Split comment text into plain text and mentions, for display. */
export function parseMentions(body: string): Segment[] {
	const out: Segment[] = [];
	let last = 0;
	for (const m of body.matchAll(TOKEN)) {
		if (m.index! > last) out.push({ text: body.slice(last, m.index) });
		out.push({ mention: m[1], id: Number(m[3]), group: m[2] === 'g:' });
		last = m.index! + m[0].length;
	}
	if (last < body.length) out.push({ text: body.slice(last) });
	return out;
}

/**
 * Turn "@Name" written in the reply box into stored mention tokens. `picked`
 * maps each chosen name to its target: a user id ("12") or a group ("g:3").
 */
export function encodeMentions(text: string, picked: Map<string, string>): string {
	// Longest names first, so "@Ann Lee" isn't half-matched by "@Ann".
	const names = [...picked.keys()].sort((a, b) => b.length - a.length);
	let out = text;
	for (const name of names) {
		const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
		out = out.replace(new RegExp(`@${escaped}(?![\\w])`, 'g'), `@[${name}](${picked.get(name)})`);
	}
	return out;
}
