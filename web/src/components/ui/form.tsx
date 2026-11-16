"use client";

import { useFormStatus } from "react-dom";
import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import type { ActionState } from "@/lib/action";

import { Button } from "./button";

const CONTROL =
  "block w-full rounded-lg border bg-white px-3 text-sm text-ink-900 placeholder:text-ink-400 " +
  "disabled:bg-ink-100 disabled:text-ink-500";

function border(error?: string) {
  return error ? "border-red-400" : "border-ink-300";
}

type FieldProps = {
  label: string;
  /** Nome do campo: liga o rótulo ao input e busca o erro em `state.errors`. */
  name: string;
  hint?: string;
  error?: string;
  children: ReactNode;
  className?: string;
};

/** Rótulo + controle + mensagem de erro/ajuda, com a acessibilidade ligada. */
export function Field({ label, name, hint, error, children, className = "" }: FieldProps) {
  return (
    <div className={className}>
      <label htmlFor={name} className="mb-1 block text-sm font-medium text-ink-700">
        {label}
      </label>
      {children}
      {error ? (
        <p id={`${name}-error`} className="mt-1 text-xs text-red-600">
          {error}
        </p>
      ) : hint ? (
        <p className="mt-1 text-xs text-ink-500">{hint}</p>
      ) : null}
    </div>
  );
}

type InputProps = InputHTMLAttributes<HTMLInputElement> & { error?: string };

export function Input({ error, className = "", id, name, ...props }: InputProps) {
  return (
    <input
      id={id ?? name}
      name={name}
      aria-invalid={error ? true : undefined}
      aria-describedby={error && name ? `${name}-error` : undefined}
      className={`${CONTROL} h-10 ${border(error)} ${className}`}
      {...props}
    />
  );
}

type SelectProps = SelectHTMLAttributes<HTMLSelectElement> & { error?: string };

export function Select({ error, className = "", id, name, children, ...props }: SelectProps) {
  return (
    <select id={id ?? name} name={name} className={`${CONTROL} h-10 ${border(error)} ${className}`} {...props}>
      {children}
    </select>
  );
}

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & { error?: string };

export function Textarea({ error, className = "", id, name, rows = 3, ...props }: TextareaProps) {
  return (
    <textarea
      id={id ?? name}
      name={name}
      rows={rows}
      className={`${CONTROL} py-2 ${border(error)} ${className}`}
      {...props}
    />
  );
}

/**
 * Botão de envio que se desabilita sozinho enquanto a Server Action roda
 * (useFormStatus lê o estado do <form> em que o botão está). Evita o clique
 * duplo — e o back-end ainda tem a Idempotency-Key como segunda defesa.
 */
export function SubmitButton({
  children,
  pendingText = "Salvando…",
  variant = "primary",
  size = "md",
  className = "",
}: {
  children: ReactNode;
  pendingText?: string;
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md";
  className?: string;
}) {
  const { pending } = useFormStatus();
  return (
    <Button type="submit" variant={variant} size={size} disabled={pending} className={className}>
      {pending ? pendingText : children}
    </Button>
  );
}

/** Mensagem de retorno de uma Server Action (sucesso ou erro). */
export function FormMessage({ state }: { state: ActionState }) {
  if (!state.message) return null;
  return (
    <p
      role={state.ok ? "status" : "alert"}
      className={`rounded-lg px-3 py-2 text-sm ${
        state.ok ? "bg-brand-50 text-brand-800" : "bg-red-50 text-red-700"
      }`}
    >
      {state.message}
    </p>
  );
}
