import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";
type Size = "sm" | "md";

const VARIANTS: Record<Variant, string> = {
  primary: "bg-brand-600 text-white hover:bg-brand-700 disabled:bg-brand-300",
  secondary: "border border-ink-300 bg-white text-ink-800 hover:bg-ink-100 disabled:text-ink-400",
  ghost: "text-ink-700 hover:bg-ink-100 disabled:text-ink-400",
  danger: "bg-red-600 text-white hover:bg-red-700 disabled:bg-red-300",
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
    "inline-flex items-center justify-center gap-2 rounded-lg font-medium whitespace-nowrap transition-colors",
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
