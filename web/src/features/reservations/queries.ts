import "server-only";

import { api, query } from "@/lib/api";
import type {
  AvailabilityGrid,
  Block,
  Dashboard,
  Folio,
  HistoryEntry,
  List,
  RegisteredGuest,
  Reservation,
  Room,
  RoomType,
} from "@/lib/types";

// Consultas usadas pelas telas de reservas. Cada função chama UM serviço; é a
// página (Server Component) que combina os resultados — o padrão "API
// Composition": como cada microserviço é dono dos seus dados, quem precisa de
// uma visão combinada busca em cada um e junta.

export function listReservations(params: {
  propertyId: string;
  status?: string;
  from?: string;
  to?: string;
  q?: string;
  limit?: number;
}) {
  return api<List<Reservation>>(
    `/v1/reservations${query({
      property_id: params.propertyId,
      status: params.status,
      from: params.from,
      to: params.to,
      q: params.q,
      limit: params.limit ?? 100,
    })}`,
  );
}

export function getReservation(id: string) {
  return api<Reservation>(`/v1/reservations/${id}`);
}

export async function getHistory(id: string) {
  return (await api<List<HistoryEntry>>(`/v1/reservations/${id}/history`)).items;
}

export function getDashboard(propertyId: string, date: string) {
  return api<Dashboard>(`/v1/reservations/dashboard${query({ property_id: propertyId, date })}`);
}

export async function listRoomTypes(propertyId: string) {
  return (await api<List<RoomType>>(`/v1/properties/${propertyId}/room-types`)).items;
}

export async function listRooms(propertyId: string) {
  return (await api<List<Room>>(`/v1/properties/${propertyId}/rooms`)).items;
}

export function getAvailability(propertyId: string, from: string, to: string) {
  return api<AvailabilityGrid>(`/v1/availability${query({ property_id: propertyId, from, to })}`);
}

export async function listBlocks(propertyId: string, from: string, to: string) {
  return (await api<List<Block>>(`/v1/blocks${query({ property_id: propertyId, from, to })}`)).items;
}

/** A conta pode ainda não existir (o financeiro a cria ao ver o evento). */
export async function getFolioByReservation(reservationId: string): Promise<Folio | null> {
  try {
    return await api<Folio>(`/v1/folios/by-reservation/${reservationId}`);
  } catch {
    return null;
  }
}

export async function getRegistration(reservationId: string): Promise<RegisteredGuest[]> {
  try {
    return (await api<List<RegisteredGuest>>(`/v1/registrations/${reservationId}`)).items;
  } catch {
    return [];
  }
}
