<script lang="ts">
	import type { TicketStatus } from '$lib/api';
	import { statusLabels, statusTags } from '$lib/tickets';

	let { status, class: cls = '' }: { status: TicketStatus; class?: string } = $props();
</script>

<span
	class={[
		'tag-chip inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium',
		statusTags[status],
		cls
	]}
>
	<span class="tag-dot size-1.5 rounded-full" aria-hidden="true"></span>
	{statusLabels[status]}
</span>
