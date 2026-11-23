<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import {
		ApiError,
		IMAGE_LIMITS,
		MAX_AVATAR_BYTES,
		PRESET_AVATARS,
		presetAvatarUrl,
		profile,
		type User
	} from '$lib/api';
	import { formatBytes } from '$lib/format';
	import Avatar from './Avatar.svelte';
	import Icon from './Icon.svelte';

	let { user, customDisabled }: { user: User; customDisabled: boolean } = $props();

	const presets = Array.from({ length: PRESET_AVATARS }, (_, i) => i + 1);
	const currentPreset = $derived(Number(user.avatarUrl.match(/^\/avatars\/(\d+)\.svg$/)?.[1] ?? 0));
	const hasCustom = $derived(user.avatarUrl.startsWith('/api/avatars/'));

	let busy = $state(false);
	let error = $state('');
	let saved = $state(false);
	let savedTimer: ReturnType<typeof setTimeout>;
	let fileInput: HTMLInputElement | undefined = $state();

	async function apply(action: () => Promise<User>) {
		busy = true;
		error = '';
		saved = false;
		clearTimeout(savedTimer);
		try {
			await action();
			await invalidateAll(); // refresh the navbar and everywhere else showing you
			saved = true;
			savedTimer = setTimeout(() => (saved = false), 2000);
		} catch (err) {
			error =
				err instanceof ApiError && err.status < 500
					? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
					: 'Something went wrong. Please try again.';
		} finally {
			busy = false;
		}
	}

	function onFile(e: Event & { currentTarget: HTMLInputElement }) {
		const file = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!file) return;
		if (!(IMAGE_LIMITS.types as readonly string[]).includes(file.type)) {
			error = 'Choose a PNG, JPEG, GIF or WebP image.';
			return;
		}
		if (file.size > MAX_AVATAR_BYTES) {
			error = `That image is larger than ${formatBytes(MAX_AVATAR_BYTES)}.`;
			return;
		}
		apply(() => profile.uploadAvatar(file));
	}
</script>

<section class="mt-6 rounded-xl border border-ink-800 bg-ink-900/60 p-6">
	<div class="flex items-start justify-between gap-3">
		<div>
			<h2 class="text-sm font-semibold">Profile picture</h2>
			<p class="mt-1 text-sm text-ink-400">Shown next to your name on tickets and replies.</p>
		</div>
		<span class="text-xs" aria-live="polite">
			{#if busy}
				<span class="text-ink-500">Saving…</span>
			{:else if saved}
				<span class="flex items-center gap-1 text-ink-400"><Icon name="check" class="size-3.5 text-primary-300" /> Saved</span>
			{/if}
		</span>
	</div>

	<div class="mt-6 flex flex-col gap-6 sm:flex-row sm:items-start">
		<!-- Current -->
		<div class="flex shrink-0 flex-col items-center gap-3 sm:w-32">
			<Avatar name={user.name} src={user.avatarUrl} size={88} tone="primary" />
			{#if user.avatarUrl}
				<button
					type="button"
					disabled={busy}
					onclick={() => apply(() => profile.setPresetAvatar(0))}
					class="text-xs text-ink-400 transition hover:text-primary-300 disabled:opacity-50"
				>
					Remove picture
				</button>
			{:else}
				<span class="text-xs text-ink-500">Using your initials</span>
			{/if}
		</div>

		<div class="min-w-0 flex-1">
			<p id="preset-label" class="mb-2.5 text-[13px] font-medium text-ink-200">Choose one</p>
			<div role="radiogroup" aria-labelledby="preset-label" class="grid w-fit grid-cols-5 gap-3">
				{#each presets as n (n)}
					{@const selected = currentPreset === n}
					<button
						type="button"
						role="radio"
						aria-checked={selected}
						aria-label="Avatar {n}"
						disabled={busy}
						onclick={() => !selected && apply(() => profile.setPresetAvatar(n))}
						class={[
							'rounded-full transition focus-visible:ring-2 focus-visible:ring-primary-400/60 focus-visible:outline-none disabled:cursor-wait',
							selected
								? 'ring-2 ring-primary-400 ring-offset-2 ring-offset-ink-900'
								: 'opacity-85 hover:scale-105 hover:opacity-100'
						]}
					>
						<img src={presetAvatarUrl(n)} alt="" class="size-12 rounded-full" />
					</button>
				{/each}
			</div>

			<div class="mt-6 border-t border-ink-800 pt-5">
				{#if customDisabled}
					<p class="flex items-start gap-2 text-sm text-ink-400">
						<Icon name="image" class="mt-0.5 size-4 shrink-0 text-ink-500" />
						<span>
							Your admin has turned off uploading your own picture{hasCustom
								? ', so your uploaded one is hidden for now'
								: ''}. You can still pick one above.
						</span>
					</p>
				{:else}
					<div class="flex flex-wrap items-center gap-3">
						<button
							type="button"
							disabled={busy}
							onclick={() => fileInput?.click()}
							class="inline-flex h-9 items-center gap-1.5 rounded-lg border border-ink-700 px-3.5 text-sm text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200 disabled:opacity-50"
						>
							<Icon name="upload" class="size-4" />
							{hasCustom ? 'Upload a new picture' : 'Upload your own'}
						</button>
						<span class="text-xs text-ink-500">PNG, JPG, GIF or WebP · up to {formatBytes(MAX_AVATAR_BYTES)}</span>
					</div>
					<input
						bind:this={fileInput}
						type="file"
						accept={IMAGE_LIMITS.types.join(',')}
						onchange={onFile}
						class="sr-only"
						tabindex="-1"
						aria-hidden="true"
					/>
				{/if}
				{#if error}
					<p role="alert" class="mt-3 text-sm text-primary-300">{error}</p>
				{/if}
			</div>
		</div>
	</div>
</section>
