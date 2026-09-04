import type { InputHTMLAttributes, SelectHTMLAttributes } from "react";

interface FormFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  name: string;
}

export function FormField({ label, name, ...rest }: FormFieldProps) {
  return (
    <label htmlFor={name} className="flex flex-col gap-1.5">
      <span className="text-sm font-medium text-ink">{label}</span>
      <input
        id={name}
        name={name}
        className="rounded border border-ink-border/30 bg-paper px-3 py-2 text-sm text-ink placeholder:text-slate-light focus:border-verified"
        {...rest}
      />
    </label>
  );
}

interface FormSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label: string;
  name: string;
  options: { value: string; label: string }[];
}

export function FormSelect({ label, name, options, ...rest }: FormSelectProps) {
  return (
    <label htmlFor={name} className="flex flex-col gap-1.5">
      <span className="text-sm font-medium text-ink">{label}</span>
      <select
        id={name}
        name={name}
        className="rounded border border-ink-border/30 bg-paper px-3 py-2 text-sm text-ink focus:border-verified"
        {...rest}
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    </label>
  );
}
