import type { Metadata } from "next";

import { Badge, Card, CardHeader, PageHeader } from "@/components/ui/layout";
import { ChangePasswordForm } from "@/features/auth/change-password-form";
import { EditUserForm, NewUserForm } from "@/features/users/forms";
import { api } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { ROLE_LABELS, label } from "@/lib/labels";
import { availableProperties, can, currentUser } from "@/lib/session";
import type { List, User } from "@/lib/types";

export const metadata: Metadata = { title: "Usuários" };

export default async function UsersPage() {
  const me = await currentUser();
  const properties = (await availableProperties()).map((property) => ({ id: property.id, name: property.name }));
  const users = can(me, "user:manage") ? (await api<List<User>>("/v1/users")).items : [];

  return (
    <>
      <PageHeader title="Usuários" description="A equipe com acesso ao painel e o que cada pessoa pode fazer." />
      <div className="space-y-6">
        {users.length > 0 ? (
          <Card>
            <CardHeader title="Equipe" />
            <ul className="divide-y divide-ink-100">
              {users.map((user) => (
                <li key={user.id}>
                  <details>
                    <summary className="flex cursor-pointer flex-wrap items-center justify-between gap-2 px-5 py-3 hover:bg-ink-50">
                      <span className="text-sm">
                        <span className="font-medium text-ink-900">{user.name}</span>{" "}
                        <span className="text-ink-500">
                          · {user.email} · último acesso {user.last_login_at ? formatDateTime(user.last_login_at) : "nunca"}
                        </span>
                      </span>
                      <span className="flex gap-1">
                        <Badge tone="brand">{label(ROLE_LABELS, user.role)}</Badge>
                        {!user.active ? <Badge tone="red">Inativo</Badge> : null}
                      </span>
                    </summary>
                    <div className="bg-ink-50/50 px-5 py-4">
                      <EditUserForm user={user} properties={properties} />
                    </div>
                  </details>
                </li>
              ))}
            </ul>
          </Card>
        ) : null}

        {can(me, "user:manage") ? (
          <Card>
            <CardHeader title="Novo usuário" />
            <div className="px-5 py-4">
              <NewUserForm properties={properties} />
            </div>
          </Card>
        ) : null}

        <Card>
          <CardHeader title="Minha senha" description={`Conectado como ${me.email}`} />
          <div className="px-5 py-4">
            <ChangePasswordForm />
          </div>
        </Card>
      </div>
    </>
  );
}
