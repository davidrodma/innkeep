#!/usr/bin/env python3
"""Gera a tabela de endpoints REST (docs/api.md) a partir do código-fonte.

A documentação da API é DERIVADA do código: cada `mux.Handle("MÉTODO /rota", ...)`
dos adaptadores REST vira uma linha, com a permissão exigida. Assim a doc não
fica desatualizada — basta rodar de novo:

    python3 scripts/list-endpoints.py > docs/api.md
"""
import collections
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent

HANDLE = re.compile(r'mux\.Handle(?:Func)?\("([A-Z]+) ([^"]+)",\s*(.+?)\)\n')
HELPER = re.compile(r'(\w+) := func\(fn httpx\.HandlerFunc\) http\.Handler \{ return guard\.Require\(access\.(\w+), fn\) \}')
PERMISSIONS = {
    "PermPropertyRead": "property:read", "PermPropertyWrite": "property:write",
    "PermHousekeepingWrite": "housekeeping:write",
    "PermInventoryRead": "inventory:read", "PermInventoryWrite": "inventory:write",
    "PermReservationRead": "reservation:read", "PermReservationWrite": "reservation:write",
    "PermGuestRead": "guest:read", "PermGuestWrite": "guest:write",
    "PermBillingRead": "billing:read", "PermBillingWrite": "billing:write",
    "PermFinanceRead": "finance:read", "PermFinanceWrite": "finance:write",
    "PermChannelRead": "channel:read", "PermChannelWrite": "channel:write",
    "PermUserManage": "user:manage",
}


def access_of(rest: str, helpers: dict) -> str:
    called = re.match(r"(\w+)\(", rest)
    if called and called.group(1) in helpers:
        return PERMISSIONS.get(helpers[called.group(1)], helpers[called.group(1)])
    required = re.search(r"guard\.Require\(access\.(\w+)", rest)
    if required:
        return PERMISSIONS.get(required.group(1), required.group(1))
    if "guard.Authenticated" in rest:
        return "autenticado"
    if "guard.Optional" in rest:
        return "público (token opcional)"
    if "write(" in rest or "read(" in rest:
        inner = re.search(r"(read|write|billingRead|billingWrite|financeRead|financeWrite)\(", rest)
        if inner and inner.group(1) in helpers:
            return PERMISSIONS.get(helpers[inner.group(1)], helpers[inner.group(1)])
    return "público"


def main() -> None:
    sys.stdout.reconfigure(encoding="utf-8")  # acentos corretos também no Windows
    services = collections.OrderedDict()
    for path in sorted((ROOT / "services").glob("*/internal/adapter/in/rest/handler.go")):
        service = path.relative_to(ROOT / "services").parts[0]
        source = path.read_text(encoding="utf-8")
        helpers = dict(HELPER.findall(source))
        for method, route, rest in HANDLE.findall(source):
            idempotent = rest.startswith("idempotent(")
            permission = access_of(rest.removeprefix("idempotent("), helpers)
            if idempotent:
                permission += " · aceita `Idempotency-Key`"
            services.setdefault(service, []).append((method, route, permission))

    total = sum(len(items) for items in services.values())
    print("# Referência da API REST\n")
    print("> Gerado por `scripts/list-endpoints.py` a partir do código — não edite à mão.\n")
    print(f"Todas as rotas passam pelo **gateway** (`/v1/...`). São {total} endpoints em {len(services)} serviços.")
    print("Rotas marcadas como *público* não exigem token; as demais exigem `Authorization: Bearer <token>`")
    print("e a permissão indicada (veja os papéis em `pkg/kernel/access/access.go`).\n")
    print("Erros seguem o formato **Problem Details (RFC 9457)**: `{type, title, status, detail, code, errors, trace_id}`.\n")
    for service, items in services.items():
        print(f"## {service}\n")
        print("| Método | Rota | Acesso |")
        print("|---|---|---|")
        for method, route, permission in items:
            print(f"| `{method}` | `{route}` | {permission} |")
        print()


if __name__ == "__main__":
    main()
