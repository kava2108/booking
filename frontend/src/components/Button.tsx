import { Loader2 } from 'lucide-react';
import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

type Variant = 'primary' | 'cta' | 'secondary' | 'ghost' | 'danger';
type Size = 'sm' | 'md' | 'lg';

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
}

// design-system/MASTER.md: cta（予約確定などの主要操作）は Amber。ラベルは暗色（コントラスト比 AA）
const variants: Record<Variant, string> = {
  primary: 'bg-primary-500 text-white hover:bg-primary-600 focus-visible:outline-primary-500',
  cta: 'bg-accent-500 text-neutral-900 font-semibold hover:bg-accent-600 focus-visible:outline-accent-500',
  secondary: 'bg-white text-primary-700 border border-primary-300 hover:bg-primary-50',
  ghost: 'bg-transparent text-neutral-700 hover:bg-neutral-100',
  danger: 'bg-red-600 text-white hover:bg-red-700 focus-visible:outline-red-600',
};
const sizes: Record<Size, string> = {
  sm: 'min-h-9 px-3 text-sm',
  md: 'min-h-11 px-4 text-base', // タッチターゲット 44px
  lg: 'min-h-12 px-6 text-lg',
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = 'primary', size = 'md', loading = false, disabled, className, children, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50',
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    >
      {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
      {children}
    </button>
  );
});
