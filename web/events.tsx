import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ArrowRight,
  CalendarDays,
  Clock3,
  MapPin,
  Plus,
  Search,
  ShieldCheck,
  Ticket,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  categories,
  eventState,
  formatDate,
  formatTime,
  type EventRecord,
  type PassDetail,
} from "@/lib/domain";
import { APIError, message, post, type Page } from "./api";
import {
  Back,
  Empty,
  ErrorBox,
  EventCard,
  Heading,
  Loading,
  Pager,
  useData,
} from "./shared";
import { useSession } from "./session";
import { toast } from "sonner";
export function Catalog({
  mode = "discover",
}: {
  mode?: "discover" | "manage" | "control";
}) {
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");
  const [period, setPeriod] = useState(mode === "manage" ? "all" : "upcoming");
  const [page, setPage] = useState(1);
  useEffect(() => {
    const t = setTimeout(() => {
      setQuery(search);
      setPage(1);
    }, 250);
    return () => clearTimeout(t);
  }, [search]);
  const { data, error, loading, reload } = useData<Page<EventRecord>>(
    `/events?scope=${mode}&q=${encodeURIComponent(query)}&category=${category === "all" ? "" : encodeURIComponent(category)}&period=${period}&page=${page}`,
  );
  return (
    <>
      <Heading
        eyebrow={
          mode === "manage"
            ? "КАБИНЕТ ОРГАНИЗАТОРА"
            : mode === "control"
              ? "РАБОТА НА ВХОДЕ"
              : "СТУДЕНЧЕСКАЯ ЖИЗНЬ"
        }
        title={
          mode === "manage"
            ? "Мои мероприятия"
            : mode === "control"
              ? "Контроль входа"
              : "Мероприятия"
        }
        description={
          mode === "control"
            ? "Выберите мероприятие, на котором вы работаете."
            : undefined
        }
        action={
          mode === "manage" ? (
            <Button asChild size="lg">
              <Link to="/manage/new">
                <Plus size={19} />
                Создать мероприятие
              </Link>
            </Button>
          ) : mode === "discover" ? (
            <Button asChild variant="outline">
              <Link to="/tickets">
                <Ticket size={18} />
                Мои пропуска
              </Link>
            </Button>
          ) : null
        }
      />
      <div className="filter-bar">
        <div className="search-input">
          <Search size={19} />
          <Input
            placeholder="Найти мероприятие или площадку"
            aria-label="Поиск мероприятий"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Select
          value={category}
          onValueChange={(v) => {
            setCategory(v);
            setPage(1);
          }}
        >
          <SelectTrigger aria-label="Категория">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Все категории</SelectItem>
            {categories.map((c) => (
              <SelectItem key={c} value={c}>
                {c}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={period}
          onValueChange={(v) => {
            setPeriod(v);
            setPage(1);
          }}
        >
          <SelectTrigger aria-label="Период">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="upcoming">Предстоящие</SelectItem>
            <SelectItem value="past">Прошедшие</SelectItem>
            <SelectItem value="all">За всё время</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="list-heading">
        <span>
          {mode === "discover"
            ? "АФИША"
            : mode === "manage"
              ? "ВАШИ СОБЫТИЯ"
              : "ДОСТУПНЫЕ МЕРОПРИЯТИЯ"}
        </span>
        {data && <span>{data.total} событий</span>}
      </div>
      {error ? (
        <ErrorBox text={error} retry={reload} />
      ) : loading ? (
        <Loading />
      ) : data && data.items.length === 0 ? (
        <Empty
          title={
            search || category !== "all"
              ? "Ничего не найдено"
              : mode === "manage"
                ? "Ваше первое мероприятие"
                : mode === "control"
                  ? "Пока нет доступных мероприятий"
                  : "Афиша пока пуста"
          }
          description={
            search || category !== "all"
              ? "Попробуйте изменить поиск или фильтры."
              : mode === "control"
                ? "Организатор может пригласить вас в команду по ссылке."
                : mode === "manage"
                  ? "Укажите место и время, откройте регистрацию и выдавайте пропуска."
                  : "Опубликованные мероприятия появятся здесь. Вы можете создать своё."
          }
        >
          {mode !== "control" && (
            <Button asChild>
              <Link to="/manage/new">
                <Plus size={17} />
                Создать мероприятие
              </Link>
            </Button>
          )}
        </Empty>
      ) : (
        <div className="events-grid">
          {data?.items.map((e) =>
            mode === "control" ? (
              <Link
                to={`/control/${e.id}`}
                key={e.id}
                className="control-event"
              >
                <span className={`event-dot color-${e.color}`}>
                  <ShieldCheck size={25} />
                </span>
                <div>
                  <span className="eyebrow">
                    {formatDate(e.starts_at)} · {formatTime(e.starts_at)}
                  </span>
                  <h2>{e.title}</h2>
                  <p>{e.location}</p>
                  <span>
                    {e.checked_in} прошли · {e.issued} пропусков
                  </span>
                </div>
                <ArrowRight />
              </Link>
            ) : (
              <EventCard key={e.id} event={e} manage={mode === "manage"} />
            ),
          )}
        </div>
      )}
      {data && <Pager data={data} onPage={setPage} />}
    </>
  );
}
export function EventDetails() {
  const { id } = useParams();
  const { session } = useSession();
  const navigate = useNavigate();
  const {
    data: e,
    error,
    loading,
    reload,
  } = useData<EventRecord>(`/events/${id}`);
  const [busy, setBusy] = useState(false);
  const [issueError, setIssueError] = useState("");
  const [requestKey, setRequestKey] = useState(() => crypto.randomUUID());
  async function register() {
    if (!session.user) {
      navigate("/login", { state: { returnTo: `/events/${id}` } });
      return;
    }
    setBusy(true);
    setIssueError("");
    try {
      const result = await post<PassDetail>(
        `/events/${id}/passes`,
        {},
        requestKey,
      );
      toast.success("Пропуск готов");
      navigate(`/tickets/${result.pass.id}`);
    } catch (e) {
      setIssueError(message(e));
      if (e instanceof APIError && e.status !== 0)
        setRequestKey(crypto.randomUUID());
    } finally {
      setBusy(false);
    }
  }
  if (loading) return <Loading />;
  if (error || !e)
    return <ErrorBox text={error || "Мероприятие не найдено"} retry={reload} />;
  const available =
    e.status === "published" &&
    Date.now() < new Date(e.registration_ends_at).getTime() &&
    Date.now() < new Date(e.ends_at).getTime();
  return (
    <>
      <Back to="/" label="Все мероприятия" />
      <div className="event-detail-grid">
        <section className="event-story">
          <div className={`event-banner color-${e.color}`}>
            <span className="category-label">{e.category}</span>
            <CalendarDays size={64} strokeWidth={1} />
            <span className="banner-number">
              {formatDate(e.starts_at, { day: "2-digit", month: undefined })}
            </span>
          </div>
          <div className="detail-copy">
            <span className={`status-pill ${e.status}`}>{eventState(e)}</span>
            <h1>{e.title}</h1>
            <div className="event-facts">
              <span>
                <CalendarDays />
                {formatDate(e.starts_at, { year: "numeric" })}
              </span>
              <span>
                <Clock3 />
                {formatTime(e.starts_at)}–{formatTime(e.ends_at)} МСК
              </span>
              <span>
                <MapPin />
                {e.location}
              </span>
            </div>
            <h2>О мероприятии</h2>
            <p className="description-text">{e.description}</p>
          </div>
        </section>
        <aside className="registration-card">
          <div className="ticket-heading">
            <Ticket size={26} />
            <span>ВАШ ПРОПУСК</span>
          </div>
          <h2>
            {e.status === "cancelled"
              ? "Мероприятие отменено"
              : available
                ? "Присоединяйтесь"
                : "Регистрация закрыта"}
          </h2>
          <p>Персональный QR-код для входа на мероприятие.</p>
          <dl>
            <div>
              <dt>Свободных мест</dt>
              <dd>
                {Math.max(0, e.capacity - e.issued)} <span>/ {e.capacity}</span>
              </dd>
            </div>
            <div>
              <dt>Вход с</dt>
              <dd>{formatTime(e.doors_at)} МСК</dd>
            </div>
            <div>
              <dt>Регистрация до</dt>
              <dd>
                {formatDate(e.registration_ends_at, { month: "short" })},{" "}
                {formatTime(e.registration_ends_at)}
              </dd>
            </div>
          </dl>
          {issueError && <ErrorBox text={issueError} />}
          <Button
            size="lg"
            disabled={busy || !available || e.issued >= e.capacity}
            onClick={register}
          >
            {busy
              ? "Выдаём пропуск…"
              : !available
                ? "Регистрация закрыта"
                : e.issued >= e.capacity
                  ? "Мест больше нет"
                  : session.user
                    ? "Получить пропуск"
                    : "Войти и получить пропуск"}
            <ArrowRight size={18} />
          </Button>
          <Link className="text-link" to="/tickets">
            Уже зарегистрировались? Мои пропуска
          </Link>
          <div className="registration-note">
            <ShieldCheck size={18} />
            <span>Один пропуск — один проход. Покажите QR-код контролёру.</span>
          </div>
          {e.role === "organizer" && (
            <Button asChild variant="outline">
              <Link to={`/manage/${e.id}`}>
                <Users size={18} />
                Управлять мероприятием
              </Link>
            </Button>
          )}
        </aside>
      </div>
    </>
  );
}
