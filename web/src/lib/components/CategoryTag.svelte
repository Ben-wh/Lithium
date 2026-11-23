<script lang="ts">
	import type { CategoryRef } from '$lib/api';

	let { category, class: cls = '' }: { category: Pick<CategoryRef, 'name' | 'color'>; class?: string } = $props();
</script>

<span
	class={[
		`tag-${category.color} tag-chip inline-flex max-w-40 shrink-0 items-center gap-1.5 rounded-full px-2 py-px text-[11px] font-medium`,
		cls
	]}
>
	<span class="tag-dot size-1.5 shrink-0 rounded-full"></span>
	<span class="truncate">{category.name}</span>
</span>
