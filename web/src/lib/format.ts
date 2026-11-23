const dateFmt = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
export const dateTimeFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

export const formatDate = (iso: string) => dateFmt.format(new Date(iso));

/** "just now", "5m ago", "3h ago", "2d ago", then a date. */
export function timeAgo(iso: string) {
	const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
	if (s < 60) return 'just now';
	if (s < 3600) return `${Math.floor(s / 60)}m ago`;
	if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
	if (s < 86400 * 7) return `${Math.floor(s / 86400)}d ago`;
	return formatDate(iso);
}

export function formatBytes(n: number) {
	if (n < 1024) return `${n} B`;
	if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
	return `${(n / (1024 * 1024)).toFixed(1).replace(/\.0$/, '')} MB`;
}
