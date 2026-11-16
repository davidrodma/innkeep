import "server-only";

import { api, query } from "@/lib/api";
import type { List, OfferSearch, PublicProperty } from "@/lib/types";

// Consultas do site público: todas anônimas (sem token).

export async function publishedProperties() {
  return (await api<List<PublicProperty>>("/v1/public/properties", { anonymous: true })).items;
}

export function publicProperty(slug: string) {
  return api<PublicProperty>(`/v1/public/properties/${encodeURIComponent(slug)}`, { anonymous: true });
}

export function searchOffers(propertyId: string, checkIn: string, checkOut: string, adults: number, children: number) {
  return api<OfferSearch>(
    `/v1/public/availability${query({ property_id: propertyId, check_in: checkIn, check_out: checkOut, adults, children })}`,
    { anonymous: true },
  );
}
