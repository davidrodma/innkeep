"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";

import { lookupBooking } from "./actions";

export function LookupForm({ code = "" }: { code?: string }) {
  const [state, action] = useActionState(lookupBooking, initialState);
  return (
    <form action={action} className="space-y-4">
      <Field label="Localizador" name="code" hint="Está no e-mail da reserva, no formato IK-XXXXXX.">
        <Input name="code" defaultValue={code} placeholder="IK-XXXXXX" autoCapitalize="characters" required />
      </Field>
      <Field label="E-mail usado na reserva" name="email">
        <Input name="email" type="email" autoComplete="email" required />
      </Field>
      <FormMessage state={state} />
      <SubmitButton pendingText="Buscando…" className="w-full">
        Ver reserva
      </SubmitButton>
    </form>
  );
}
