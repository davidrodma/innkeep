"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";

import { changePassword } from "./actions";

export function ChangePasswordForm() {
  const [state, action] = useActionState(changePassword, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-4 sm:grid-cols-3 sm:items-end">
      <Field label="Senha atual" name="current_password" error={errors.current_password}>
        <Input name="current_password" type="password" autoComplete="current-password" required />
      </Field>
      <Field label="Nova senha" name="new_password" error={errors.password ?? errors.new_password} hint="Mínimo de 10 caracteres.">
        <Input name="new_password" type="password" autoComplete="new-password" minLength={10} required />
      </Field>
      <Field label="Repita a nova senha" name="confirm_password" error={errors.confirm_password}>
        <Input name="confirm_password" type="password" autoComplete="new-password" required />
      </Field>
      <div className="space-y-3 sm:col-span-3">
        <FormMessage state={state} />
        <SubmitButton variant="secondary">Trocar senha</SubmitButton>
      </div>
    </form>
  );
}
