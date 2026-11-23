<script lang="ts">
	import type { TicketStatus } from '$lib/api';
	import { statusLabels } from '$lib/tickets';

	let { status, class: cls = '' }: { status: TicketStatus; class?: string } = $props();

	const styles: Record<TicketStatus, string> = {
		unassigned: 'border-primary-400/30 bg-primary-400/10 text-primary-200',
		in_progress: 'border-ink-500/60 bg-ink-850 text-ink-100',
		pending: 'border-dashed border-ink-500/70 bg-transparent text-ink-200',
		resolved: 'border-ink-700 bg-transparent text-ink-400',
		closed: 'border-ink-800 bg-transparent text-ink-500'
	};
</script>

<span class={['inline-flex shrink-0 items-center rounded-md border px-2 py-0.5 text-xs font-medium', styles[status], cls]}>
	{statusLabels[status]}
</span>
