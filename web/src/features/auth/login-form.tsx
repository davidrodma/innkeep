"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";

import { login } from "./actions";

export function LoginForm({ next }: { next: string }) {
  // useActionState liga o formulário à Server Action: `state` é o que a
  // action devolveu por último (mensagem de erro, por exemplo).
  const [state, action] = useActionState(login, initialState);

  return (
    <form action={action} className="space-y-4">
      <input type="hidden" name="next" value={next} />
      <Field label="E-mail" name="email">
        <Input name="email" type="email" autoComplete="username" required autoFocus />
      </Field>
      <Field label="Senha" name="password">
        <Input name="password" type="password" autoComplete="current-password" required />
      </Field>
      <FormMessage state={state} />
      <SubmitButton pendingText="Entrando…" className="w-full">
        Entrar
      </SubmitButton>
    </form>
  );
}
