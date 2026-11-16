import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md";

const VARIANTS: Record<Variant, string> = {
  primary: "bg-brand-600 text-white shadow-[0_10px_20px_-10px] shadow-brand-600/70 hover:bg-brand-700 hover:shadow-none disabled:bg-brand-300 disabled:shadow-none",
  secondary: "border border-ink-200 bg-white text-ink-700 hover:border-brand-300 hover:text-brand-700 disabled:text-ink-400",
  ghost: "text-ink-700 hover:bg-ink-100 disabled:text-ink-400",
  danger: "bg-danger text-white shadow-[0_10px_20px_-10px] shadow-danger/70 hover:bg-danger-ink hover:shadow-none disabled:opacity-60",
};

const SIZES: Record<Size, string> = {
  sm: "h-8 px-3 text-sm",
  md: "h-10 px-4 text-sm",
};

/**
 * Classes de um botão. Exportada para estilizar também links (<Link>) com a
 * mesma aparência, sem duplicar a lista de classes.
 */
export function buttonClass(variant: Variant = "primary", size: Size = "md", extra = ""): string {
  return [
    "inline-flex items-center justify-center gap-2 rounded-md font-semibold whitespace-nowrap transition",
    "disabled:cursor-not-allowed",
    VARIANTS[variant],
    SIZES[size],
    extra,
  ].join(" ");
}

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: Size };

export function Button({ variant = "primary", size = "md", className = "", type = "button", ...props }: Props) {
  return <button type={type} className={buttonClass(variant, size, className)} {...props} />;
}
