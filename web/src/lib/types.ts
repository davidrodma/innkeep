// Tipos das respostas da API do InnKeep (espelham os DTOs dos serviços em Go).
//
// Convenções da API:
//   - valores monetários vêm SEMPRE em centavos (inteiros), nunca decimais;
//   - datas de estadia são strings "AAAA-MM-DD", sem horário nem fuso;
//   - listas vêm no envelope { items, total }.

export type List<T> = { items: T[]; total: number };

export type User = {
  id: string;
  email: string;
  name: string;
  role: Role;
  property_ids: string[];
  permissions: string[];
  active: boolean;
  last_login_at: string | null;
};

export type Role = "ADMIN" | "MANAGER" | "RECEPTIONIST" | "FINANCE" | "HOUSEKEEPING";

export type Tokens = {
  access_token: string;
  expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
  user: User;
};

export type Address = {
  line: string;
  district: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
};

export type Property = {
  id: string;
  slug: string;
  name: string;
  legal_name: string;
  tax_id: string;
  cadastur: string;
  kind: string;
  description: string;
  email: string;
  phone: string;
  address: Address;
  timezone: string;
  currency: string;
  check_in_time: string;
  check_out_time: string;
  deposit_percent: number;
  hold_minutes: number;
  cancellation_policy: string;
  pix_key: string;
  published: boolean;
};

export type RoomType = {
  id: string;
  property_id: string;
  name: string;
  description: string;
  kind: "PRIVATE" | "SHARED";
  max_occupancy: number;
  bed_configuration: string;
  base_price_cents: number;
  amenities: string[];
  photo_urls: string[];
  active: boolean;
  sort_order: number;
};

export type PublicProperty = Omit<Property, "legal_name" | "tax_id" | "cadastur" | "published"> & {
  room_types?: RoomType[];
};

export type Room = {
  id: string;
  property_id: string;
  room_type_id: string;
  name: string;
  floor: string;
  notes: string;
  active: boolean;
  housekeeping: "CLEAN" | "DIRTY";
};

export type NightlyRate = { date: string; price_cents: number };

export type ReservationStatus =
  | "REQUESTED"
  | "PENDING"
  | "CONFIRMED"
  | "CHECKED_IN"
  | "CHECKED_OUT"
  | "CANCELLED"
  | "REJECTED"
  | "EXPIRED"
  | "NO_SHOW";

export type Reservation = {
  id: string;
  code: string;
  property_id: string;
  room_type_id: string;
  room_id: string;
  room_name: string;
  check_in: string;
  check_out: string;
  nights: number;
  adults: number;
  children: number;
  holder: { name: string; email?: string; phone?: string };
  source: string;
  external_ref: string;
  group_code: string;
  notes: string;
  status: ReservationStatus;
  nightly_rates: NightlyRate[];
  total_cents: number;
  deposit_due_cents: number;
  currency: string;
  hold_expires_at: string | null;
  payment_collect: string;
  overbooked: boolean;
  registration_completed: boolean;
  primary_guest_id: string;
  pending_change: { new_room_id?: string; new_check_in: string; new_check_out: string } | null;
  cancel_reason: string;
  reject_reason: string;
  reject_detail: string;
  requested_at: string;
  confirmed_at: string | null;
  checked_in_at: string | null;
  checked_out_at: string | null;
};

export type PublicReservation = {
  id: string;
  code: string;
  property_id: string;
  status: ReservationStatus;
  room_type_id: string;
  check_in: string;
  check_out: string;
  nights: number;
  adults: number;
  children: number;
  holder_name: string;
  nightly_rates: NightlyRate[];
  total_cents: number;
  deposit_due_cents: number;
  currency: string;
  hold_expires_at: string | null;
  reject_reason: string;
};

export type HistoryEntry = {
  version: number;
  type: string;
  actor: string;
  occurred_at: string;
  data: Record<string, unknown>;
};

export type Dashboard = {
  date: string;
  arrivals: Reservation[];
  departures: Reservation[];
  in_house: Reservation[];
  pending: Reservation[];
};

export type DayStatus = {
  date: string;
  total: number;
  available: number;
  price_cents: number;
  min_stay: number;
  closed: boolean;
};

export type RoomTypeCalendar = {
  room_type_id: string;
  name: string;
  max_occupancy: number;
  base_price_cents: number;
  currency: string;
  days: DayStatus[];
};

export type AvailabilityGrid = { from: string; to: string; room_types: RoomTypeCalendar[] };

export type Offer = {
  room_type_id: string;
  name: string;
  max_occupancy: number;
  available: number;
  nightly_rates: NightlyRate[];
  total_cents: number;
  currency: string;
  bookable: boolean;
  reason?: string;
};

export type OfferSearch = { check_in: string; check_out: string; nights: number; offers: Offer[] };

export type Block = {
  id: string;
  room_id: string;
  room_name: string;
  room_type_id: string;
  from: string;
  to: string;
  reason: string;
};

export type FolioEntry = {
  id: string;
  kind: "CHARGE" | "DISCOUNT" | "PAYMENT" | "REFUND";
  category: string;
  description: string;
  amount_cents: number;
  method: string;
  fee_cents: number;
  reference: string;
  business_date: string;
  created_at: string;
  created_by: string;
  voided: boolean;
  void_reason: string;
};

export type Folio = {
  id: string;
  reservation_id: string;
  reservation_code: string;
  property_id: string;
  guest_name: string;
  source: string;
  status: "OPEN" | "CLOSED";
  currency: string;
  check_in: string;
  check_out: string;
  deposit_due_cents: number;
  deposit_settled_at: string | null;
  charges_cents: number;
  discounts_cents: number;
  total_due_cents: number;
  paid_cents: number;
  fees_cents: number;
  balance_cents: number;
  entries: FolioEntry[];
};

export type FolioSummary = {
  id: string;
  reservation_id: string;
  reservation_code: string;
  guest_name: string;
  source: string;
  status: string;
  check_in: string;
  check_out: string;
  total_due_cents: number;
  paid_cents: number;
  balance_cents: number;
};

export type Expense = {
  id: string;
  property_id: string;
  category: string;
  description: string;
  supplier: string;
  amount_cents: number;
  due_date: string;
  status: "PENDING" | "PAID";
  paid_on: string;
  method: string;
};

export type CashMovement = {
  id: string;
  kind: "IN" | "OUT";
  amount_cents: number;
  reason: string;
  ref_type: string;
  occurred_on: string;
  created_by: string;
};

export type FeeRule = { method: string; bps: number };

export type FinancialReport = {
  from: string;
  to: string;
  gross_cents: number;
  fees_cents: number;
  refunds_cents: number;
  net_cents: number;
  expenses_paid_cents: number;
  expenses_pending_cents: number;
  result_cents: number;
  receivables_cents: number;
  by_method: { method: string; count: number; gross_cents: number; fees_cents: number; net_cents: number }[];
  by_source: { source: string; gross_cents: number; fees_cents: number; net_cents: number }[];
  expenses_by_category: { category: string; amount_cents: number }[];
  daily: { date: string; gross_cents: number; net_cents: number; expenses_cents: number }[];
  cash: { opening_cents: number; in_cents: number; out_cents: number; closing_cents: number };
};

export type Guest = {
  id: string;
  full_name: string;
  email: string;
  phone: string;
  birth_date: string;
  gender: string;
  nationality: string;
  profession: string;
  document_type: string;
  document_number: string;
  document_issuer: string;
  address: Address;
  notes: string;
  marketing_consent: boolean;
  anonymized: boolean;
};

export type RegistrationCard = {
  id: string;
  reservation_id: string;
  property_id: string;
  guest_id: string;
  primary: boolean;
  purpose_of_trip: string;
  transport: string;
  origin: { city: string; state: string; country: string };
  destination: { city: string; state: string; country: string };
  vehicle_plate: string;
  check_in: string;
  check_out: string;
  checked_in_at: string | null;
  checked_out_at: string | null;
};

export type RegisteredGuest = { guest: Guest; card: RegistrationCard };

export type ChannelConnection = {
  id: string;
  property_id: string;
  channel: "BOOKING_COM" | "AIRBNB";
  mode: "API" | "ICAL";
  name: string;
  external_property_id: string;
  base_url: string;
  has_credentials: boolean;
  active: boolean;
  last_push_at: string | null;
  last_pull_at: string | null;
  last_error: string;
};

export type ChannelMapping = {
  id: string;
  connection_id: string;
  room_type_id: string;
  external_room_id: string;
  external_rate_id: string;
  ical_import_url: string;
  ical_export_url: string;
};

export type SyncLog = {
  id: string;
  direction: "OUTBOUND" | "INBOUND";
  operation: string;
  success: boolean;
  detail: string;
  created_at: string;
};
