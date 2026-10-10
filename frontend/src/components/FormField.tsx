import { forwardRef, useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

interface FieldProps {
  label: string;
  error?: string | undefined;
  hint?: string | undefined;
  required?: boolean | undefined;
  children: (a: { id: string; describedBy: string | undefined; invalid: boolean }) => ReactNode;
}

/** ラベル・ヒント・エラー表示をまとめる。エラーは role="alert" で読み上げられる。 */
export function Field({ label, error, hint, required, children }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errId = error ? `${id}-err` : undefined;
  const describedBy = [hintId, errId].filter(Boolean).join(' ') || undefined;
  return (
    <div className="space-y-1">
      {/* 必須マークは CSS で付ける（ラベルの文字列を汚さず、アクセシブルネームは「氏名」のまま） */}
      <label htmlFor={id} className={cn('block text-sm font-medium text-neutral-800', required && "after:ml-1 after:text-red-600 after:content-['*']")}>
        {label}
      </label>
      {children({ id, describedBy, invalid: !!error })}
      {hint && <p id={hintId} className="text-xs text-neutral-600">{hint}</p>}
      {error && <p id={errId} role="alert" className="text-sm text-red-700">{error}</p>}
    </div>
  );
}

const control =
  'block w-full min-h-11 rounded-lg border bg-white px-3 py-2 text-base text-neutral-900 placeholder:text-neutral-400 ' +
  'focus-visible:outline-2 focus-visible:outline-primary-500 disabled:bg-neutral-100 disabled:text-neutral-500 ' +
  'aria-[invalid=true]:border-red-500 border-neutral-300';

type BaseProps = { label: string; error?: string | undefined; hint?: string | undefined };

export const TextInput = forwardRef<HTMLInputElement, BaseProps & InputHTMLAttributes<HTMLInputElement>>(function TextInput(
  { label, error, hint, className, required, ...rest },
  ref,
) {
  return (
    <Field label={label} error={error} hint={hint} required={required}>
      {({ id, describedBy, invalid }) => (
        <input ref={ref} id={id} aria-describedby={describedBy} aria-invalid={invalid || undefined} required={required} className={cn(control, className)} {...rest} />
      )}
    </Field>
  );
});

export const SelectInput = forwardRef<HTMLSelectElement, BaseProps & SelectHTMLAttributes<HTMLSelectElement>>(function SelectInput(
  { label, error, hint, className, children, required, ...rest },
  ref,
) {
  return (
    <Field label={label} error={error} hint={hint} required={required}>
      {({ id, describedBy, invalid }) => (
        <select ref={ref} id={id} aria-describedby={describedBy} aria-invalid={invalid || undefined} className={cn(control, className)} {...rest}>
          {children}
        </select>
      )}
    </Field>
  );
});

export const TextArea = forwardRef<HTMLTextAreaElement, BaseProps & TextareaHTMLAttributes<HTMLTextAreaElement>>(function TextArea(
  { label, error, hint, className, required, ...rest },
  ref,
) {
  return (
    <Field label={label} error={error} hint={hint} required={required}>
      {({ id, describedBy, invalid }) => (
        <textarea ref={ref} id={id} aria-describedby={describedBy} aria-invalid={invalid || undefined} className={cn(control, 'min-h-24', className)} {...rest} />
      )}
    </Field>
  );
});
