<script lang="ts">
	import type { Priority } from '$lib/api';

	// Signal bars for low/normal/high; a filled alert square for urgent.
	let { priority, class: cls = 'size-4' }: { priority: Priority; class?: string } = $props();
	const bars = $derived({ low: 1, normal: 2, high: 3, urgent: 3 }[priority]);
</script>

{#if priority === 'urgent'}
	<svg class={cls} viewBox="0 0 16 16" aria-hidden="true">
		<rect x="1" y="1" width="14" height="14" rx="3.5" class="fill-danger-400" />
		<path d="M8 4.5V9" stroke-width="2" stroke-linecap="round" class="stroke-danger-950" />
		<circle cx="8" cy="11.75" r="1.1" class="fill-danger-950" />
	</svg>
{:else}
	<svg class={cls} viewBox="0 0 16 16" aria-hidden="true">
		{#each [4, 7.5, 11] as h, i (i)}
			<rect x={1.5 + i * 4.75} y={14 - h} width="3.25" height={h} rx="1" class={['fill-current', i >= bars && 'opacity-25']} />
		{/each}
	</svg>
{/if}
