import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Avatar } from '../Avatar';

describe('Avatar', () => {
  it('renders with initials from name', () => {
    render(<Avatar name="John Doe" />);
    expect(screen.getByText('JD')).toBeInTheDocument();
  });

  it('renders with image when imageUrl provided', () => {
    render(<Avatar name="John Doe" imageUrl="/avatar.jpg" />);
    const img = screen.getByRole('img', { name: /john doe/i });
    expect(img).toBeInTheDocument();
    expect(img).toHaveAttribute('src', '/avatar.jpg');
  });

  it('applies size classes', () => {
    const { container } = render(<Avatar name="John" size="lg" />);
    expect(container.firstChild).toHaveClass('h-12', 'w-12');
  });

  it('renders presence indicator when provided', () => {
    const { container } = render(<Avatar name="John" presence="online" />);
    const presenceDot = container.querySelector('[class*="bg-[var(--success)]"]');
    expect(presenceDot).toBeInTheDocument();
  });

  it('applies custom className', () => {
    const { container } = render(<Avatar name="John" className="custom-class" />);
    expect(container.firstChild).toHaveClass('custom-class');
  });
});
