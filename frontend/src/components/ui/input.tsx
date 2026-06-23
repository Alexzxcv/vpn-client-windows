import { forwardRef } from 'react';
import { cn } from '@/lib/utils';

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, type = 'text', ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      type={type}
      className={cn(
        'flex h-9 w-full rounded-sm border border-hairline bg-slate px-2.5 text-base text-frost',
        'placeholder:text-mute transition-colors',
        'focus-visible:outline-none focus-visible:border-ion focus-visible:ring-2 focus-visible:ring-ion/40',
        'disabled:cursor-not-allowed disabled:opacity-60',
        className,
      )}
      {...props}
    />
  );
});
