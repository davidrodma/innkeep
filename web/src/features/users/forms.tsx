"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import { ROLE_LABELS } from "@/lib/labels";
import type { User } from "@/lib/types";

import { createUser, updateUser } from "./actions";

const ROLE_HINTS: Record<string, string> = {
  ADMIN: "Acesso total, inclusive usuários.",
  MANAGER: "Tudo, exceto gerenciar usuários.",
  RECEPTIONIST: "Reservas, hóspedes e recebimentos.",
  FINANCE: "Contas, despesas, caixa e relatórios.",
  HOUSEKEEPING: "Somente a situação de limpeza dos quartos.",
};

function PropertyChecks({ prefix, properties, selected }: { prefix: string; properties: { id: string; name: string }[]; selected: string[] }) {
  if (properties.length < 2) return null;
  return (
    <fieldset className="sm:col-span-6">
      <legend className="mb-1 text-sm font-medium text-ink-700">Propriedades (nenhuma marcada = todas)</legend>
      <div className="flex flex-wrap gap-2">
        {properties.map((property) => (
          <label key={property.id} className="flex items-center gap-1.5 rounded-lg border border-ink-200 px-2.5 py-1.5 text-sm">
            <input
              type="checkbox"
              name="property_ids"
              value={property.id}
              defaultChecked={selected.includes(property.id)}
              id={`${prefix}-${property.id}`}
              className="size-4 rounded border-ink-300"
            />
            {property.name}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function RoleSelect({ id, defaultValue }: { id: string; defaultValue: string }) {
  return (
    <Select id={id} name="role" defaultValue={defaultValue}>
      {Object.entries(ROLE_LABELS).map(([code, text]) => (
        <option key={code} value={code}>
          {text} — {ROLE_HINTS[code]}
        </option>
      ))}
    </Select>
  );
}

export function NewUserForm({ properties }: { properties: { id: string; name: string }[] }) {
  const [state, action] = useActionState(createUser, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6 sm:items-end">
      <Field label="Nome" name="name" error={errors.name} className="sm:col-span-2">
        <Input name="name" required />
      </Field>
      <Field label="E-mail" name="email" error={errors.email} className="sm:col-span-2">
        <Input name="email" type="email" autoComplete="off" required />
      </Field>
      <Field label="Senha inicial" name="password" error={errors.password} hint="Mínimo de 10 caracteres." className="sm:col-span-2">
        <Input name="password" type="password" autoComplete="new-password" minLength={10} required />
      </Field>
      <Field label="Papel" name="role" error={errors.role} className="sm:col-span-6">
        <RoleSelect id="role" defaultValue="RECEPTIONIST" />
      </Field>
      <PropertyChecks prefix="new" properties={properties} selected={[]} />
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton>Criar usuário</SubmitButton>
      </div>
    </form>
  );
}

export function EditUserForm({ user, properties }: { user: User; properties: { id: string; name: string }[] }) {
  const [state, action] = useActionState(updateUser, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="id" value={user.id} />
      <Field label="Nome" name={`name-${user.id}`} error={errors.name} className="sm:col-span-3">
        <Input id={`name-${user.id}`} name="name" defaultValue={user.name} required />
      </Field>
      <Field label="Papel" name={`role-${user.id}`} className="sm:col-span-3">
        <RoleSelect id={`role-${user.id}`} defaultValue={user.role} />
      </Field>
      <Field label="Nova senha" name={`new_password-${user.id}`} error={errors.password} hint="Em branco = manter. Encerra as sessões do usuário." className="sm:col-span-3">
        <Input id={`new_password-${user.id}`} name="new_password" type="password" autoComplete="new-password" />
      </Field>
      <div className="flex h-10 items-center gap-2 sm:col-span-3">
        <input id={`active-${user.id}`} name="active" type="checkbox" defaultChecked={user.active} className="size-4 rounded border-ink-300" />
        <label htmlFor={`active-${user.id}`} className="text-sm text-ink-700">
          Ativo (pode entrar no sistema)
        </label>
      </div>
      <PropertyChecks prefix={user.id} properties={properties} selected={user.property_ids} />
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton size="sm" variant="secondary">
          Salvar
        </SubmitButton>
      </div>
    </form>
  );
}
