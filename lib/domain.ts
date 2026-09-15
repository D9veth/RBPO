export const categories = [
  "Лекции",
  "Карьера",
  "Культура",
  "Спорт",
  "Сообщества",
] as const;
export type Category = (typeof categories)[number];
export type EventStatus = "draft" | "published" | "cancelled";
export type EventInput = {
  title: string;
  description: string;
  category: Category;
  location: string;
  starts_at: string;
  ends_at: string;
  doors_at: string;
  registration_ends_at: string;
  capacity: number;
  color: string;
};
export type EventRecord = EventInput & {
  id: string;
  owner_id: string;
  status: EventStatus;
  version: number;
  created_at: string;
  updated_at: string;
  issued: number;
  checked_in: number;
  role: "organizer" | "controller" | "";
};
export type PassRecord = {
  id: string;
  event_id: string;
  holder_id: string | null;
  holder_name: string;
  status: "active" | "redeemed" | "revoked";
  issued_at: string;
  issued_by: string;
  redeemed_at: string | null;
  redeemed_by: string | null;
  revoked_at: string | null;
  revoke_reason: string | null;
  event_title: string;
  event_location: string;
  starts_at: string;
  ends_at: string;
  event_status: EventStatus;
  color: string;
};
export type PassDetail = { pass: PassRecord; code: string; qr_payload: string };
export type Staff = {
  id: string;
  event_id: string;
  label: string;
  role: "controller" | "organizer";
  user_id: string | null;
  name: string | null;
  created_at: string;
  expires_at: string;
  accepted_at: string | null;
  revoked_at: string | null;
};
export type Attempt = {
  id: number;
  result: string;
  created_at: string;
  actor: string;
  holder_name: string;
};
export type Audit = {
  id: number;
  action: string;
  target_id: string;
  details: Record<string, unknown>;
  created_at: string;
  actor: string;
};
export type Activity = {
  attempts: Attempt[];
  audit: Audit[];
  event: EventRecord;
};
export type Admission = {
  result: string;
  message: string;
  pass?: PassRecord;
  replay: boolean;
};
export function formatDate(
  value: string | number,
  options: Intl.DateTimeFormatOptions = {},
) {
  return new Intl.DateTimeFormat("ru-RU", {
    day: "numeric",
    month: "long",
    timeZone: "Europe/Moscow",
    ...options,
  }).format(new Date(value));
}
export function formatTime(value: string | number) {
  return new Intl.DateTimeFormat("ru-RU", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Moscow",
  }).format(new Date(value));
}
export function eventState(
  e: Pick<EventRecord, "status" | "ends_at" | "starts_at">,
) {
  if (e.status === "draft") return "Черновик";
  if (e.status === "cancelled") return "Отменено";
  if (new Date(e.ends_at).getTime() < Date.now()) return "Завершено";
  if (new Date(e.starts_at).getTime() <= Date.now()) return "Идёт сейчас";
  return "Скоро";
}
export const passStates = {
  active: "Действует",
  redeemed: "Использован",
  revoked: "Отменён",
};
