<script lang="ts">
	import { onDestroy } from 'svelte';
	import { SLA_WARNING_SHARE, type SLAStatus, type SLATarget } from '$lib/api';
	import { dateTimeFmt, formatMinutes } from '$lib/format';

	// A ticket's SLA targets in the staff sidebar, counting down live.
	// ownTicket: the viewer is the requester, whose replies don't count as a
	// first response, so say so instead of leaving them wondering.
	let { sla, ownTicket = false }: { sla: SLAStatus; ownTicket?: boolean } = $props();

	let now = $state(Date.now());
	const loadedAt = Date.now();
	const tick = setInterval(() => (now = Date.now()), 30_000);
	onDestroy(() => clearInterval(tick));

	type View = { tone: 'ok' | 'warn' | 'bad' | 'muted'; text: string; title?: string };

	function view(t: SLATarget): View {
		const target = formatMinutes(t.targetMinutes);
		switch (t.state) {
			case 'met':
				return { tone: 'ok', text: `Met in ${formatMinutes(t.elapsedSeconds / 60)} (target ${target})` };
			case 'missed':
				return { tone: 'bad', text: `Missed: took ${formatMinutes(t.elapsedSeconds / 60)} (target ${target})` };
			case 'paused':
				return {
					tone: 'muted',
					text: `Paused while pending: ${formatMinutes(t.targetMinutes - t.elapsedSeconds / 60)} left`
				};
			case 'overdue': {
				// Still counting: add the time since the page loaded.
				const over = t.elapsedSeconds / 60 + (now - loadedAt) / 60_000 - t.targetMinutes;
				return { tone: 'bad', text: `Overdue by ${formatMinutes(over)} (target ${target})` };
			}
			default: {
				const left = t.dueAt ? (new Date(t.dueAt).getTime() - now) / 60_000 : 0;
				if (left <= 0) return { tone: 'bad', text: `Overdue (target ${target})` };
				const used = 1 - left / t.targetMinutes;
				return {
					tone: used >= SLA_WARNING_SHARE ? 'warn' : 'ok',
					text: `Due in ${formatMinutes(left)}`,
					title: t.dueAt ? `Due ${dateTimeFmt.format(new Date(t.dueAt))} (target ${target})` : undefined
				};
			}
		}
	}

	const dot: Record<View['tone'], string> = {
		ok: 'tag-green',
		warn: 'tag-amber',
		bad: 'tag-red',
		muted: 'tag-gray'
	};
	const rows = $derived(
		[
			{ label: 'First response', t: sla.firstResponse },
			{ label: 'Resolution', t: sla.resolution }
		].filter((r): r is { label: string; t: SLATarget } => !!r.t)
	);
</script>

<div class="space-y-2.5">
	{#each rows as r (r.label)}
		{@const v = view(r.t)}
		<div>
			<p class="text-xs text-ink-500">{r.label}</p>
			<p class="mt-1 flex items-center gap-2 text-sm" title={v.title}>
				<span class="{dot[v.tone]} tag-dot size-2 shrink-0 rounded-full" aria-hidden="true"></span>
				<span class={v.tone === 'bad' ? 'text-danger-200' : v.tone === 'muted' ? 'text-ink-400' : 'text-ink-100'}>{v.text}</span>
			</p>
			{#if r.label === 'First response' && ownTicket && r.t.state !== 'met' && r.t.state !== 'missed'}
				<p class="mt-1 text-xs text-ink-500">You raised this ticket, so your replies don't count. Another agent or admin needs to reply.</p>
			{/if}
		</div>
	{/each}
</div>
