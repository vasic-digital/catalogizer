// Regression guard (found by the LIVE playback spec): @tailwindcss/aspect-ratio replaces theme.aspectRatio, which silently drops the
// built-in `aspect-video` / `aspect-square` utilities. Every `aspect-video` box (media player, playlist and favourites tiles) then
// collapsed to height 0 and the <video> element was never visible. This compiles the REAL tailwind config and asserts the utilities exist.
import { describe, it, expect } from 'vitest';
import postcss from 'postcss';
import tailwind from 'tailwindcss';
import config from '../../tailwind.config.js';

async function css(): Promise<string> {
  const out = await postcss([tailwind({ ...config, content: [{ raw: '<div class="aspect-video aspect-square"></div>' }] })]).process('@tailwind utilities;', { from: undefined });
  return out.css;
}

describe('tailwind config aspect-ratio utilities', () => {
  it('emits aspect-video as 16 / 9', async () => {
    expect(await css()).toMatch(/\.aspect-video\s*{[^}]*aspect-ratio:\s*16\s*\/\s*9/);
  });
  it('emits aspect-square as 1 / 1', async () => {
    expect(await css()).toMatch(/\.aspect-square\s*{[^}]*aspect-ratio:\s*1\s*\/\s*1/);
  });
});
