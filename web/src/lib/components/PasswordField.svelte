<script lang="ts" module>
	// Check before submitting a form that uses `confirm`.
	export function passwordsMatch(password: string, confirmation: string) {
		return password === confirmation;
	}
</script>

<script lang="ts">
	import Icon from './Icon.svelte';
	import { copyText } from '$lib/clipboard';

	let {
		value = $bindable(''),
		confirmValue = $bindable(''),
		id,
		label = 'Password',
		confirm = false,
		generator = false,
		autocomplete = 'new-password',
		placeholder = 'At least 8 characters'
	}: {
		value?: string;
		confirmValue?: string;
		id: string;
		label?: string;
		/** Show a second "confirm" box. */
		confirm?: boolean;
		/** Show Generate and Copy (for admins setting someone else's password). */
		generator?: boolean;
		autocomplete?: AutoFill;
		placeholder?: string;
	} = $props();

	let show = $state(false);
	let copied = $state(false);

	const mismatch = $derived(confirm && confirmValue.length > 0 && confirmValue !== value);
	const matches = $derived(confirm && confirmValue.length > 0 && confirmValue === value);

	// No look-alike characters (0/O, 1/l/I) so it's easy to read out or type.
	const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789';

	function generate() {
		const bytes = crypto.getRandomValues(new Uint32Array(16));
		value = Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('');
		confirmValue = value;
		show = true;
	}

	async function copy() {
		if (await copyText(value)) {
			copied = true;
			setTimeout(() => (copied = false), 1500);
		}
	}

	const iconBtn =
		'flex size-8 items-center justify-center rounded-md text-ink-500 transition hover:bg-ink-850 hover:text-ink-100 disabled:pointer-events-none disabled:opacity-40';
	const labelClass = 'text-[13px] font-medium text-ink-200';
</script>

<div class="space-y-4">
	<div>
		<div class="mb-1.5 flex items-center justify-between">
			<label for={id} class={labelClass}>{label}</label>
			{#if generator}
				<button
					type="button"
					onclick={generate}
					class="flex items-center gap-1 text-xs font-medium text-primary-300 transition hover:text-primary-200"
				>
					<Icon name="refresh" class="size-3.5" /> Generate
				</button>
			{/if}
		</div>
		<div class="relative">
			<input
				{id}
				type={show ? 'text' : 'password'}
				{autocomplete}
				required
				minlength="8"
				bind:value
				{placeholder}
				class={['field', generator ? 'pr-20 font-mono' : 'pr-11']}
			/>
			<div class="absolute inset-y-0 right-1 flex items-center">
				{#if generator}
					<button
						type="button"
						onclick={copy}
						disabled={!value}
						class={iconBtn}
						aria-label={copied ? 'Copied' : 'Copy password'}
					>
						<Icon name={copied ? 'check' : 'copy'} class={['size-4', copied ? 'text-primary-300' : ''].join(' ')} />
					</button>
				{/if}
				<button
					type="button"
					onclick={() => (show = !show)}
					class={iconBtn}
					aria-label={show ? 'Hide passwords' : 'Show passwords'}
				>
					<Icon name={show ? 'eyeOff' : 'eye'} class="size-4" />
				</button>
			</div>
		</div>
	</div>

	{#if confirm}
		<div>
			<label for="{id}-confirm" class="mb-1.5 block {labelClass}">Confirm {label.toLowerCase()}</label>
			<div class="relative">
				<input
					id="{id}-confirm"
					type={show ? 'text' : 'password'}
					autocomplete="new-password"
					required
					bind:value={confirmValue}
					placeholder="Type it again"
					aria-invalid={mismatch}
					aria-describedby="{id}-confirm-hint"
					class={[
						'field pr-11',
						generator && 'font-mono',
						mismatch && 'border-primary-400/70! focus:ring-primary-400/25!'
					]}
				/>
				{#if matches}
					<span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-primary-300">
						<Icon name="check" class="size-4" />
					</span>
				{/if}
			</div>
			<p id="{id}-confirm-hint" class="mt-1.5 min-h-4 text-xs text-primary-300" aria-live="polite">
				{mismatch ? "Passwords don't match." : ''}
			</p>
		</div>
	{/if}
</div>
