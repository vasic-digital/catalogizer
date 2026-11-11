import type { KeyboardEvent } from 'react';

/**
 * Builds an onKeyDown handler that triggers `action` on Enter or Space, so
 * click-only elements with role="button" are keyboard operable (S1082).
 */
export function onActivateKey<T extends Element = Element>(action: (e: KeyboardEvent<T>) => void) {
  return (e: KeyboardEvent<T>) => {
    if (e.target !== e.currentTarget || e.repeat) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      action(e);
    }
  };
}
