import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  CalendarDays,
  MapPin,
  RefreshCw,
  Ticket,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api, message, type Page } from "./api";
import {
  eventState,
  formatDate,
  formatTime,
  type EventRecord,
} from "@/lib/domain";
export function useData<T>(path: string | null) {
  const [state, setState] = useState<{
    path: string | null;
    data: T | null;
    error: string;
    loading: boolean;
  }>({ path: null, data: null, error: "", loading: false });
  const [version, bump] = useState(0);
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController();
    setState((previous) => ({
      path,
      data: previous.path === path ? previous.data : null,
      error: "",
      loading: true,
    }));
    api<T>(path, { signal: controller.signal })
      .then((data) => {
        if (!controller.signal.aborted)
          setState({ path, data, error: "", loading: false });
      })
      .catch((error) => {
        if (!controller.signal.aborted)
          setState((previous) => ({
            ...previous,
            error: message(error),
            loading: false,
          }));
      });
    return () => controller.abort();
  }, [path, version]);
  const reload = useCallback(() => bump((v) => v + 1), []);
  const setData = useCallback(
    (data: T) => setState({ path, data, error: "", loading: false }),
    [path],
  );
  const current = path !== null && state.path === path;
  return {
    data: current ? state.data : null,
    error: current ? state.error : "",
    loading: !!path && (!current || state.loading),
    reload,
    setData,
  };
}
export function ErrorBox({
  text,
  retry,
}: {
  text: string;
  retry?: () => void;
}) {
  return (
    <div className="error-box" role="alert">
      <AlertCircle size={20} />
      <span>{text}</span>
      {retry && (
        <Button variant="outline" size="sm" onClick={retry}>
          <RefreshCw size={16} />
          Повторить
        </Button>
      )}
    </div>
  );
}
export function Loading() {
  return (
    <div aria-busy="true" aria-label="Загрузка" className="loading-grid">
      {[0, 1, 2].map((i) => (
        <Skeleton key={i} className="h-44 w-full rounded-2xl" />
      ))}
    </div>
  );
}
export function Empty({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="empty-state">
      <div className="empty-icon">
        <Ticket size={30} />
      </div>
      <h2>{title}</h2>
      <p>{description}</p>
      {children}
    </div>
  );
}
export function Heading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        {eyebrow && <p className="eyebrow">{eyebrow}</p>}
        <h1>{title}</h1>
        {description && <p className="page-description">{description}</p>}
      </div>
      {action}
    </div>
  );
}
export function EventCard({
  event: e,
  manage = false,
}: {
  event: EventRecord;
  manage?: boolean;
}) {
  const state = eventState(e),
    ratio = Math.min(100, Math.round((e.issued / e.capacity) * 100));
  return (
    <Link
      to={manage ? `/manage/${e.id}` : `/events/${e.id}`}
      className={`event-card color-${e.color}`}
    >
      <div className="event-color">
        <span className="category-label">{e.category}</span>
        <CalendarDays size={32} strokeWidth={1.5} />
        <div className="card-date">
          <strong>
            {formatDate(e.starts_at, { day: "2-digit", month: undefined })}
          </strong>
          <span>
            {formatDate(e.starts_at, { day: undefined, month: "short" })}
          </span>
        </div>
      </div>
      <div className="event-card-body">
        <div className="event-card-meta">
          <span className={`status-pill ${e.status}`}>{state}</span>
          <span>{formatTime(e.starts_at)} МСК</span>
        </div>
        <h2>{e.title}</h2>
        <p className="location">
          <MapPin size={16} />
          {e.location}
        </p>
        <div className="capacity-line">
          <span>
            <Users size={15} />
            {e.issued} из {e.capacity} мест
          </span>
          <ArrowRight size={20} />
        </div>
        <div className="capacity-bar">
          <span style={{ width: `${ratio}%` }} />
        </div>
      </div>
    </Link>
  );
}
export function Pager({
  data,
  onPage,
}: {
  data: Page<unknown>;
  onPage: (n: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(data.total / data.limit));
  if (pages < 2) return null;
  return (
    <nav className="pager" aria-label="Страницы">
      <Button
        variant="outline"
        disabled={data.page <= 1}
        onClick={() => onPage(data.page - 1)}
      >
        <ArrowLeft size={16} />
        Назад
      </Button>
      <span>
        {data.page} / {pages}
      </span>
      <Button
        variant="outline"
        disabled={data.page >= pages}
        onClick={() => onPage(data.page + 1)}
      >
        Далее
        <ArrowRight size={16} />
      </Button>
    </nav>
  );
}
export function Field({
  label,
  error,
  children,
  hint,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
  hint?: string;
}) {
  return (
    <label className={`form-field ${error ? "field-error" : ""}`}>
      <span>{label}</span>
      {children}
      {hint && <small>{hint}</small>}
      {error && <small role="alert">{error}</small>}
    </label>
  );
}
export function Back({ to, label = "Назад" }: { to: string; label?: string }) {
  return (
    <Link className="back-link" to={to}>
      <ArrowLeft size={16} />
      {label}
    </Link>
  );
}
