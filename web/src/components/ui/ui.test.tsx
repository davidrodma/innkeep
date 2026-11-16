import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Field, FormMessage, Input } from "./form";
import { StatusBadge } from "./layout";

describe("StatusBadge", () => {
  it("traduz a situação da reserva", () => {
    render(<StatusBadge status="PENDING" />);
    expect(screen.getByText("Aguardando sinal")).toBeInTheDocument();
  });
});

describe("Field + Input", () => {
  it("liga o rótulo ao campo e anuncia o erro para leitores de tela", () => {
    render(
      <Field label="E-mail" name="email" error="Formato inválido">
        <Input name="email" error="Formato inválido" />
      </Field>,
    );
    const input = screen.getByLabelText("E-mail");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Formato inválido");
  });
});

describe("FormMessage", () => {
  it("usa role=alert para erros e role=status para sucesso", () => {
    const { rerender } = render(<FormMessage state={{ ok: false, message: "Falhou" }} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Falhou");
    rerender(<FormMessage state={{ ok: true, message: "Salvo" }} />);
    expect(screen.getByRole("status")).toHaveTextContent("Salvo");
  });

  it("não renderiza nada sem mensagem", () => {
    const { container } = render(<FormMessage state={{}} />);
    expect(container).toBeEmptyDOMElement();
  });
});
