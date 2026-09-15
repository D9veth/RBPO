import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowRight, CalendarDays, MapPin, Palette } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { categories, type EventInput, type EventRecord } from "@/lib/domain";
import { APIError, api, message, post } from "./api";
import { Back, ErrorBox, Field, Heading } from "./shared";
import { toast } from "sonner";
const toInput = (v: string) =>
  new Date(new Date(v).getTime() + 3 * 3600000).toISOString().slice(0, 16);
const colors = [
  { value: "blue", label: "Синий" },
  { value: "violet", label: "Фиолетовый" },
  { value: "orange", label: "Оранжевый" },
  { value: "green", label: "Зелёный" },
  { value: "pink", label: "Розовый" },
];
export function EventForm({
  event,
  onSaved,
}: {
  event?: EventRecord;
  onSaved?: (e: EventRecord) => void;
}) {
  const navigate = useNavigate();
  const tomorrow = new Date();
  tomorrow.setUTCDate(tomorrow.getUTCDate() + 1);
  tomorrow.setUTCHours(15, 0, 0, 0);
  const initial = event ?? {
    title: "",
    description: "",
    category: "Сообщества",
    location: "",
    starts_at: tomorrow.toISOString(),
    ends_at: new Date(+tomorrow + 7200000).toISOString(),
    doors_at: new Date(+tomorrow - 1800000).toISOString(),
    registration_ends_at: tomorrow.toISOString(),
    capacity: 100,
    color: "blue",
  };
  const [form, setForm] = useState({
    ...initial,
    starts_at: toInput(initial.starts_at),
    ends_at: toInput(initial.ends_at),
    doors_at: toInput(initial.doors_at),
    registration_ends_at: toInput(initial.registration_ends_at),
  });
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  function change(name: string, value: string | number) {
    setForm((f) => ({ ...f, [name]: value }));
  }
  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setFields({});
    try {
      const payload: EventInput = {
        title: form.title,
        description: form.description,
        category: form.category as EventInput["category"],
        location: form.location,
        capacity: Number(form.capacity),
        color: form.color,
        starts_at: new Date(form.starts_at + "+03:00").toISOString(),
        ends_at: new Date(form.ends_at + "+03:00").toISOString(),
        doors_at: new Date(form.doors_at + "+03:00").toISOString(),
        registration_ends_at: new Date(
          form.registration_ends_at + "+03:00",
        ).toISOString(),
      };
      const result = event
        ? await api<EventRecord>(`/events/${event.id}`, {
            method: "PATCH",
            body: JSON.stringify({ ...payload, version: event.version }),
          })
        : await post<EventRecord>("/events", payload);
      toast.success(
        event ? "Изменения сохранены" : "Черновик мероприятия создан",
      );
      if (onSaved) onSaved(result);
      else navigate(`/manage/${result.id}`);
    } catch (e) {
      setError(message(e));
      if (e instanceof APIError) setFields(e.fields);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      {!event && (
        <>
          <Back to="/manage" label="Мои мероприятия" />
          <Heading
            eyebrow="НОВОЕ СОБЫТИЕ"
            title="Соберём всех вместе"
            description="Сначала сохраним черновик. Регистрацию можно открыть, когда всё будет готово."
          />
        </>
      )}
      <form onSubmit={save} className="event-form">
        <section className="panel">
          <h2>
            <CalendarDays size={22} />О мероприятии
          </h2>
          <Field label="Название" error={fields.title}>
            <Input
              value={form.title}
              onChange={(e) => change("title", e.target.value)}
              required
              minLength={4}
              maxLength={120}
              placeholder="Например, вечер студенческих проектов"
            />
          </Field>
          <Field
            label="Описание"
            error={fields.description}
            hint="Что будет происходить, для кого это событие и что взять с собой."
          >
            <Textarea
              value={form.description}
              onChange={(e) => change("description", e.target.value)}
              rows={6}
              required
              minLength={20}
              maxLength={5000}
              placeholder="Расскажите участникам о мероприятии…"
            />
          </Field>
          <div className="form-grid">
            <Field label="Категория" error={fields.category}>
              <Select
                value={form.category}
                onValueChange={(v) => change("category", v)}
              >
                <SelectTrigger aria-label="Категория мероприятия">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {categories.map((c) => (
                    <SelectItem value={c} key={c}>
                      {c}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Field label="Количество мест" error={fields.capacity}>
              <Input
                type="number"
                min={1}
                max={10000}
                value={form.capacity}
                required
                onChange={(e) => change("capacity", Number(e.target.value))}
              />
            </Field>
          </div>
        </section>
        <section className="panel">
          <h2>
            <MapPin size={22} />
            Место и время
          </h2>
          <Field label="Площадка" error={fields.location}>
            <Input
              value={form.location}
              required
              minLength={3}
              maxLength={240}
              onChange={(e) => change("location", e.target.value)}
              placeholder="Корпус, адрес и аудитория"
            />
          </Field>
          <p className="form-note">Все даты и время — по Москве (UTC+3).</p>
          <div className="form-grid">
            {(
              [
                { key: "starts_at", label: "Начало мероприятия" },
                { key: "ends_at", label: "Окончание" },
                { key: "doors_at", label: "Открытие входа" },
                { key: "registration_ends_at", label: "Регистрация до" },
              ] as const
            ).map((f) => (
              <Field key={f.key} label={f.label} error={fields[f.key]}>
                <Input
                  type="datetime-local"
                  value={form[f.key]}
                  required
                  onChange={(e) => change(f.key, e.target.value)}
                />
              </Field>
            ))}
          </div>
        </section>
        <section className="panel">
          <h2>
            <Palette size={22} />
            Цвет мероприятия
          </h2>
          <RadioGroup
            value={form.color}
            onValueChange={(v) => change("color", v)}
            className="color-options"
            aria-label="Цвет мероприятия"
          >
            {colors.map((c) => (
              <label key={c.value} className={`color-option color-${c.value}`}>
                <RadioGroupItem value={c.value} />
                <span className="color-swatch" />
                <span>{c.label}</span>
              </label>
            ))}
          </RadioGroup>
        </section>
        {error && <ErrorBox text={error} />}
        <div className="form-actions">
          <span>
            {event
              ? "Изменения будут видны участникам."
              : "Черновик виден только команде мероприятия."}
          </span>
          <Button type="submit" size="lg" disabled={busy}>
            {busy
              ? "Сохраняем…"
              : event
                ? "Сохранить изменения"
                : "Создать черновик"}
            <ArrowRight size={18} />
          </Button>
        </div>
      </form>
    </>
  );
}
