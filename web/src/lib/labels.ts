// Tradução dos códigos da API para os textos exibidos na interface.
// A API fala em códigos estáveis (em inglês); a tradução é assunto da tela.

import type { ReservationStatus } from "./types";

export const STATUS_LABELS: Record<ReservationStatus, string> = {
  REQUESTED: "Processando",
  PENDING: "Aguardando sinal",
  CONFIRMED: "Confirmada",
  CHECKED_IN: "Hospedado",
  CHECKED_OUT: "Finalizada",
  CANCELLED: "Cancelada",
  REJECTED: "Sem disponibilidade",
  EXPIRED: "Expirada",
  NO_SHOW: "Não compareceu",
};

export const SOURCE_LABELS: Record<string, string> = {
  DIRECT: "Direto",
  WEBSITE: "Site",
  BOOKING_COM: "Booking.com",
  AIRBNB: "Airbnb",
};

export const METHOD_LABELS: Record<string, string> = {
  PIX: "Pix",
  CASH: "Dinheiro",
  CREDIT_CARD: "Cartão de crédito",
  DEBIT_CARD: "Cartão de débito",
  BANK_TRANSFER: "Transferência",
  PAYMENT_LINK: "Link de pagamento",
  BOOKING_COM: "Pago via Booking.com",
  AIRBNB: "Pago via Airbnb",
  VOUCHER: "Crédito / cortesia",
  OTHER: "Outro",
};

export const EXPENSE_CATEGORY_LABELS: Record<string, string> = {
  UTILITIES: "Água, luz e internet",
  SUPPLIES: "Material de limpeza e consumo",
  MAINTENANCE: "Manutenção",
  PAYROLL: "Funcionários",
  RENT: "Aluguel",
  COMMISSION: "Comissões",
  TAXES: "Impostos e taxas",
  MARKETING: "Divulgação",
  OTHER: "Outras",
};

export const ROLE_LABELS: Record<string, string> = {
  ADMIN: "Administrador",
  MANAGER: "Gerente",
  RECEPTIONIST: "Recepção",
  FINANCE: "Financeiro",
  HOUSEKEEPING: "Governança",
};

export const REJECT_LABELS: Record<string, string> = {
  NO_AVAILABILITY: "Não há quartos livres deste tipo no período.",
  ROOM_TAKEN: "O quarto escolhido já está ocupado no período.",
  CLOSED: "As vendas estão fechadas em alguma das datas.",
  MIN_STAY: "O período é menor que a estadia mínima exigida.",
  UNKNOWN_ROOM_TYPE: "Tipo de acomodação indisponível.",
  OVER_CAPACITY: "O número de hóspedes excede a capacidade do quarto.",
  TIMEOUT: "Não foi possível verificar a disponibilidade. Tente novamente.",
};

export const EVENT_LABELS: Record<string, string> = {
  "reservation.requested": "Reserva solicitada",
  "reservation.room_allocated": "Quarto reservado no inventário",
  "reservation.rejected": "Reserva rejeitada",
  "reservation.confirmed": "Reserva confirmada",
  "reservation.cancelled": "Reserva cancelada",
  "reservation.expired": "Prazo do sinal expirou",
  "reservation.marked_no_show": "Hóspede não compareceu",
  "reservation.registration_completed": "Ficha de registro (FNRH) preenchida",
  "reservation.checked_in": "Check-in realizado",
  "reservation.checked_out": "Check-out realizado",
  "reservation.stay_change_requested": "Alteração de estadia solicitada",
  "reservation.stay_changed": "Estadia alterada",
  "reservation.stay_change_rejected": "Alteração de estadia recusada",
};

export const TRIP_PURPOSE_LABELS: Record<string, string> = {
  LEISURE: "Lazer / férias",
  BUSINESS: "Negócios",
  EVENT: "Congresso / feira / evento",
  FAMILY: "Visita a parentes ou amigos",
  STUDY: "Estudos / cursos",
  HEALTH: "Saúde",
  RELIGION: "Religião",
  SHOPPING: "Compras",
  OTHER: "Outro",
};

export const TRANSPORT_LABELS: Record<string, string> = {
  CAR: "Automóvel",
  BUS: "Ônibus",
  PLANE: "Avião",
  MOTORCYCLE: "Moto",
  SHIP: "Navio / barco",
  TRAIN: "Trem",
  OTHER: "Outro",
};

export const DOCUMENT_LABELS: Record<string, string> = {
  CPF: "CPF",
  RG: "RG",
  PASSPORT: "Passaporte",
  CNH: "CNH",
  RNE: "RNE / CRNM",
  OTHER: "Outro",
};

export function label(map: Record<string, string>, code: string): string {
  return map[code] ?? code;
}
