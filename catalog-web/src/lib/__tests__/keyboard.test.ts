import { describe, it, expect, vi } from 'vitest';
import { onActivateKey } from '../keyboard';

const ev = (key: string, same = true) => {
  const el = {};
  return { key, target: el, currentTarget: same ? el : {}, preventDefault: vi.fn() } as never;
};

describe('onActivateKey', () => {
  it.each(['Enter', ' '])('fires on %j and prevents default', (k) => {
    const action = vi.fn();
    const e = ev(k);
    onActivateKey(action)(e);
    expect(action).toHaveBeenCalledTimes(1);
    expect((e as { preventDefault: () => void }).preventDefault).toHaveBeenCalled();
  });
  it('ignores other keys and bubbled events from children', () => {
    const action = vi.fn();
    onActivateKey(action)(ev('a'));
    onActivateKey(action)(ev('Enter', false));
    expect(action).not.toHaveBeenCalled();
  });
});
