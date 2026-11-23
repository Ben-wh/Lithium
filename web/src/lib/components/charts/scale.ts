/** A "nice" axis maximum (1, 2 or 5 × 10ⁿ) at or above max, with tick values. */
export function niceTicks(max: number, count = 4): number[] {
	if (max <= 0) return [0, 1];
	const rough = max / count;
	const pow = 10 ** Math.floor(Math.log10(rough));
	const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= rough)!;
	const top = Math.ceil(max / step) * step;
	const ticks: number[] = [];
	for (let v = 0; v <= top + step / 2; v += step) ticks.push(Math.round(v * 1e6) / 1e6);
	return ticks;
}

export const compact = (n: number) =>
	n >= 10_000 ? `${Math.round(n / 1000)}K` : n.toLocaleString(undefined, { maximumFractionDigits: 1 });
