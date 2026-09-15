import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ArrowUpRight,
  Check,
  Copy,
  Download,
  ExternalLink,
  Link2,
  Plus,
  Search,
  ShieldCheck,
  Ticket,
  Users,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type Activity,
  type EventRecord,
  type PassDetail,
  type PassRecord,
  type Staff,
  eventState,
  formatDate,
  formatTime,
  passStates,
} from "@/lib/domain";
import { api, message, post, type Page } from "./api";
import {
  Back,
  Empty,
  ErrorBox,
  Field,
  Heading,
  Loading,
  Pager,
  useData,
} from "./shared";
import { useSession } from "./session";
import { EventForm } from "./event-form";
import { toast } from "sonner";
export async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    toast.success("Скопировано");
  } catch {
    toast.error("Не удалось скопировать. Выделите и скопируйте текст вручную.");
  }
}
export function Confirm({
  trigger,
  title,
  description,
  onConfirm,
}: {
  trigger: React.ReactNode;
  title: string;
  description: string;
  onConfirm: () => Promise<void>;
}) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Оставить</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => void onConfirm()}
            className="danger-button"
          >
            Подтвердить
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
export function Manage() {
  const { id } = useParams();
  const {
    data: e,
    error,
    loading,
    reload,
    setData,
  } = useData<EventRecord>(`/events/${id}`);
  const [busy, setBusy] = useState(false);
  async function status(next: string) {
    if (!e) return;
    setBusy(true);
    try {
      const result = await post<EventRecord>(`/events/${id}/status`, {
        status: next,
        version: e.version,
      });
      setData(result);
      toast.success(
        next === "published"
          ? "Регистрация открыта"
          : next === "cancelled"
            ? "Мероприятие отменено"
            : "Мероприятие снято с публикации",
      );
    } catch (e) {
      toast.error(message(e));
    } finally {
      setBusy(false);
    }
  }
  if (loading && !e) return <Loading />;
  if (error || !e)
    return <ErrorBox text={error || "Мероприятие не найдено"} retry={reload} />;
  if (e.role !== "organizer")
    return (
      <ErrorBox text="Для управления мероприятием нужны права организатора." />
    );
  return (
    <>
      <Back to="/manage" label="Мои мероприятия" />
      <Heading
        eyebrow="УПРАВЛЕНИЕ МЕРОПРИЯТИЕМ"
        title={e.title}
        action={
          <div className="button-row">
            <Button asChild variant="outline">
              <Link to={`/events/${id}`}>
                <ExternalLink size={17} />
                Страница события
              </Link>
            </Button>
            {e.status === "draft" && (
              <Button disabled={busy} onClick={() => status("published")}>
                Открыть регистрацию
                <ArrowUpRight size={18} />
              </Button>
            )}
          </div>
        }
      />
      <div className="manage-meta">
        <span className={`status-pill ${e.status}`}>{eventState(e)}</span>
        <span>
          {formatDate(e.starts_at)} · {formatTime(e.starts_at)} МСК
        </span>
        <span>{e.location}</span>
      </div>
      <div className="stats-row">
        <div className="stat-card">
          <span>
            Выдано пропусков
            <Ticket />
          </span>
          <strong>
            {e.issued}
            <small> / {e.capacity}</small>
          </strong>
          <div className="capacity-bar">
            <span
              style={{
                width: `${Math.min(100, (e.issued / e.capacity) * 100)}%`,
              }}
            />
          </div>
        </div>
        <div className="stat-card">
          <span>
            Уже на мероприятии
            <Check />
          </span>
          <strong>{e.checked_in}</strong>
          <small>
            {e.issued ? Math.round((e.checked_in / e.issued) * 100) : 0}% от
            выданных пропусков
          </small>
        </div>
        <div className="stat-card">
          <span>
            Осталось мест
            <Users />
          </span>
          <strong>{Math.max(0, e.capacity - e.issued)}</strong>
          <small>Вместимость площадки: {e.capacity}</small>
        </div>
      </div>
      <Tabs defaultValue="participants" className="manage-tabs">
        <TabsList variant="line">
          <TabsTrigger value="participants">Участники</TabsTrigger>
          <TabsTrigger value="staff">Команда</TabsTrigger>
          <TabsTrigger value="activity">Журнал</TabsTrigger>
          <TabsTrigger value="settings">Настройки</TabsTrigger>
        </TabsList>
        <TabsContent value="participants">
          <Participants event={e} refresh={reload} />
        </TabsContent>
        <TabsContent value="staff">
          <Team event={e} />
        </TabsContent>
        <TabsContent value="activity">
          <ActivityView id={e.id} />
        </TabsContent>
        <TabsContent value="settings">
          {e.status !== "cancelled" ? (
            <EventForm key={e.version} event={e} onSaved={setData} />
          ) : (
            <p className="form-note">
              Отменённое мероприятие доступно для просмотра. Участники видят
              отмену на своих пропусках.
            </p>
          )}
          <section className="panel danger-zone">
            <div>
              <h2>Управление публикацией</h2>
              <p>
                При отмене все пропуска на это событие перестают давать право
                входа.
              </p>
            </div>
            {e.status === "published" && e.issued === 0 && (
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => status("draft")}
              >
                Вернуть в черновики
              </Button>
            )}
            {e.status !== "cancelled" && (
              <Confirm
                trigger={
                  <Button
                    variant="outline"
                    disabled={busy}
                    className="danger-text"
                  >
                    Отменить мероприятие
                  </Button>
                }
                title="Отменить мероприятие?"
                description="Регистрация и вход будут закрыты. Вернуть отменённое мероприятие нельзя."
                onConfirm={() => status("cancelled")}
              />
            )}
          </section>
        </TabsContent>
      </Tabs>
    </>
  );
}
function Participants({
  event: e,
  refresh,
}: {
  event: EventRecord;
  refresh: () => void;
}) {
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("all");
  const [page, setPage] = useState(1);
  const { data, error, loading, reload } = useData<Page<PassRecord>>(
    `/events/${e.id}/passes?q=${encodeURIComponent(q)}&status=${status === "all" ? "" : status}&page=${page}`,
  );
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [issueError, setIssueError] = useState("");
  const [key, setKey] = useState(() => crypto.randomUUID());
  const [guest, setGuest] = useState("");
  const [issued, setIssued] = useState<PassDetail | null>(null);
  async function issue(f: React.FormEvent) {
    f.preventDefault();
    setBusy(true);
    setIssueError("");
    try {
      const result = await post<PassDetail>(
        `/events/${e.id}/passes`,
        { guest_name: guest },
        key,
      );
      setIssued(result);
      reload();
      refresh();
    } catch (err) {
      setIssueError(message(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <div className="section-toolbar">
        <div className="button-row">
          <div className="search-input">
            <Search size={18} />
            <Input
              placeholder="Найти участника"
              aria-label="Поиск участников"
              value={q}
              onChange={(v) => {
                setQ(v.target.value);
                setPage(1);
              }}
            />
          </div>
          <Select
            value={status}
            onValueChange={(v) => {
              setStatus(v);
              setPage(1);
            }}
          >
            <SelectTrigger aria-label="Статус пропуска">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Все статусы</SelectItem>
              {Object.entries(passStates).map(([v, l]) => (
                <SelectItem value={v} key={v}>
                  {l}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="button-row">
          <Button asChild variant="outline">
            <a href={`/api/v1/events/${e.id}/export`}>
              <Download size={17} />
              CSV
            </a>
          </Button>
          <Button asChild variant="outline">
            <Link to={`/control/${e.id}`}>
              <ShieldCheck size={17} />
              Открыть вход
            </Link>
          </Button>
          <Dialog
            open={open}
            onOpenChange={(v) => {
              setOpen(v);
              if (v) {
                setKey(crypto.randomUUID());
                setIssued(null);
                setGuest("");
                setIssueError("");
              }
            }}
          >
            <DialogTrigger asChild>
              <Button disabled={e.status !== "published"}>
                <Plus size={17} />
                Выдать пропуск
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>
                  {issued ? "Пропуск готов" : "Пропуск для гостя"}
                </DialogTitle>
                <DialogDescription>
                  {issued
                    ? "Откройте пропуск, чтобы скачать QR-код или распечатать его."
                    : "Для гостевого пропуска аккаунт участника не нужен."}
                </DialogDescription>
              </DialogHeader>
              {issued ? (
                <div className="success-panel">
                  <Check size={30} />
                  <h3>{issued.pass.holder_name}</h3>
                  <Button asChild>
                    <Link to={`/tickets/${issued.pass.id}`}>
                      Открыть пропуск
                      <ArrowUpRight size={17} />
                    </Link>
                  </Button>
                </div>
              ) : (
                <form onSubmit={issue}>
                  <Field label="Имя участника">
                    <Input
                      value={guest}
                      onChange={(v) => {
                        setGuest(v.target.value);
                        setKey(crypto.randomUUID());
                      }}
                      required
                      minLength={2}
                      maxLength={120}
                      placeholder="Имя и фамилия"
                    />
                  </Field>
                  {issueError && <ErrorBox text={issueError} />}
                  <Button disabled={busy} type="submit">
                    {busy ? "Выдаём…" : "Выдать пропуск"}
                  </Button>
                </form>
              )}
            </DialogContent>
          </Dialog>
        </div>
      </div>
      {error ? (
        <ErrorBox text={error} retry={reload} />
      ) : loading ? (
        <Loading />
      ) : !data?.items.length ? (
        <Empty
          title="Участников пока нет"
          description="Поделитесь страницей опубликованного события или выдайте гостевой пропуск."
        >
          <Button
            variant="outline"
            onClick={() => copy(`${window.location.origin}/events/${e.id}`)}
          >
            <Link2 size={17} />
            Скопировать адрес события
          </Button>
        </Empty>
      ) : (
        <div className="table-panel">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Участник</TableHead>
                <TableHead>Пропуск</TableHead>
                <TableHead>Выдан</TableHead>
                <TableHead>Время входа</TableHead>
                <TableHead>
                  <span className="sr-only">Действие</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.items.map((p) => (
                <TableRow key={p.id}>
                  <TableCell>
                    <strong>{p.holder_name}</strong>
                    <span className="table-sub">
                      {p.holder_id
                        ? "Регистрация участника"
                        : "Гостевой пропуск"}
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className={`status-pill ${p.status}`}>
                      {passStates[p.status]}
                    </span>
                  </TableCell>
                  <TableCell>
                    {formatDate(p.issued_at, { month: "short" })},{" "}
                    {formatTime(p.issued_at)}
                  </TableCell>
                  <TableCell>
                    {p.redeemed_at ? formatTime(p.redeemed_at) : "—"}
                  </TableCell>
                  <TableCell>
                    <Button asChild variant="ghost" size="sm">
                      <Link to={`/tickets/${p.id}`}>
                        Открыть
                        <ArrowUpRight size={16} />
                      </Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {data && <Pager data={data} onPage={setPage} />}
    </>
  );
}
function Team({ event: e }: { event: EventRecord }) {
  const { session } = useSession();
  const { data, error, loading, reload } = useData<{
    items: Staff[];
    owner_id: string;
    owner_name: string;
  }>(`/events/${e.id}/staff`);
  const [open, setOpen] = useState(false);
  const [role, setRole] = useState("controller");
  const [label, setLabel] = useState("");
  const [url, setURL] = useState("");
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState("");
  async function invite(f: React.FormEvent) {
    f.preventDefault();
    setBusy(true);
    setFormError("");
    try {
      const result = await post<{ token: string }>(`/events/${e.id}/staff`, {
        label,
        role,
      });
      setURL(`${location.origin}/join#${result.token}`);
      reload();
    } catch (e) {
      setFormError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    try {
      await api(`/events/${e.id}/staff/${id}`, { method: "DELETE" });
      toast.success("Доступ отозван");
      reload();
    } catch (e) {
      toast.error(message(e));
    }
  }
  return (
    <>
      <div className="section-toolbar">
        <div>
          <h2>Команда мероприятия</h2>
          <p>
            Контролёры проверяют пропуска. Соорганизаторы также управляют
            событием и участниками.
          </p>
        </div>
        {session.user?.id === e.owner_id && (
          <Dialog
            open={open}
            onOpenChange={(v) => {
              setOpen(v);
              if (v) {
                setURL("");
                setLabel("");
                setFormError("");
              }
            }}
          >
            <DialogTrigger asChild>
              <Button>
                <Plus size={17} />
                Пригласить
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>
                  {url ? "Приглашение создано" : "Пригласить в команду"}
                </DialogTitle>
                <DialogDescription>
                  {url
                    ? "Ссылка действует 7 дней и может быть использована один раз. Скопируйте её сейчас."
                    : "Отправьте полученную ссылку человеку, которому доверяете работу на мероприятии."}
                </DialogDescription>
              </DialogHeader>
              {url ? (
                <>
                  <Input
                    readOnly
                    value={url}
                    aria-label="Ссылка-приглашение"
                    onFocus={(e) => e.target.select()}
                  />
                  <Button onClick={() => copy(url)}>
                    <Copy size={17} />
                    Скопировать ссылку
                  </Button>
                </>
              ) : (
                <form onSubmit={invite}>
                  <Field label="Для кого приглашение">
                    <Input
                      required
                      minLength={2}
                      maxLength={120}
                      value={label}
                      onChange={(e) => setLabel(e.target.value)}
                      placeholder="Например, контролёр главного входа"
                    />
                  </Field>
                  <Field label="Роль">
                    <Select value={role} onValueChange={setRole}>
                      <SelectTrigger aria-label="Роль сотрудника">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="controller">Контролёр</SelectItem>
                        <SelectItem value="organizer">Соорганизатор</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                  {formError && <ErrorBox text={formError} />}
                  <Button disabled={busy}>Создать приглашение</Button>
                </form>
              )}
            </DialogContent>
          </Dialog>
        )}
      </div>
      {error ? (
        <ErrorBox text={error} retry={reload} />
      ) : loading ? (
        <Loading />
      ) : (
        <div className="staff-list">
          <div className="staff-row">
            <span className="avatar">{data?.owner_name.slice(0, 1)}</span>
            <div>
              <strong>{data?.owner_name}</strong>
              <small>Владелец мероприятия</small>
            </div>
            <span className="status-pill active">В команде</span>
          </div>
          {data?.items.map((s) => (
            <div className="staff-row" key={s.id}>
              <span className="avatar secondary-avatar">
                {(s.name ?? s.label).slice(0, 1)}
              </span>
              <div>
                <strong>{s.name ?? s.label}</strong>
                <small>
                  {s.role === "controller" ? "Контролёр" : "Соорганизатор"} ·{" "}
                  {s.name ? s.label : "Приглашение"}
                </small>
              </div>
              <span
                className={`status-pill ${s.revoked_at ? "revoked" : s.accepted_at ? "active" : "draft"}`}
              >
                {s.revoked_at
                  ? "Отозвано"
                  : s.accepted_at
                    ? "В команде"
                    : Date.now() > new Date(s.expires_at).getTime()
                      ? "Истекло"
                      : "Ожидает принятия"}
              </span>
              {!s.revoked_at && session.user?.id === e.owner_id && (
                <Confirm
                  trigger={
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Отозвать доступ: ${s.name ?? s.label}`}
                    >
                      <X size={17} />
                    </Button>
                  }
                  title="Отозвать доступ?"
                  description="Сотрудник сразу потеряет права на это мероприятие. Неиспользованное приглашение также перестанет действовать."
                  onConfirm={() => revoke(s.id)}
                />
              )}
            </div>
          ))}
        </div>
      )}
    </>
  );
}
const actionLabels: Record<string, string> = {
  "event.created": "Создано мероприятие",
  "event.updated": "Изменены настройки",
  "event.published": "Открыта регистрация",
  "event.draft": "Снято с публикации",
  "event.cancelled": "Мероприятие отменено",
  "pass.issued": "Выдан пропуск",
  "pass.revoked": "Отменён пропуск",
  "pass.redeemed": "Подтверждён проход",
  "staff.invited": "Создано приглашение",
  "staff.accepted": "Участник присоединился к команде",
  "staff.revoked": "Отозван доступ",
  "participants.exported": "Выгружен список участников",
};
export function ActivityView({ id }: { id: string }) {
  const { data, error, loading, reload } = useData<Activity>(
    `/events/${id}/activity`,
  );
  return (
    <section className="panel">
      <div className="section-toolbar">
        <h2>Последние действия</h2>
        <Button variant="outline" onClick={reload}>
          Обновить
        </Button>
      </div>
      {error ? (
        <ErrorBox text={error} retry={reload} />
      ) : loading ? (
        <Loading />
      ) : !data?.audit.length ? (
        <p className="muted">Действий пока нет.</p>
      ) : (
        <div className="audit-list">
          {data.audit.map((a) => (
            <div className="audit-row" key={a.id}>
              <span className="audit-symbol">
                <Check size={15} />
              </span>
              <div>
                <strong>{actionLabels[a.action] ?? a.action}</strong>
                <small>{a.actor}</small>
              </div>
              <time>
                {formatDate(a.created_at, { month: "short" })},{" "}
                {formatTime(a.created_at)}
              </time>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
export function RevokePass({
  pass,
  onDone,
}: {
  pass: PassRecord;
  onDone: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function revoke(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await post(`/passes/${pass.id}/revoke`, { reason });
      setOpen(false);
      onDone();
      toast.success("Пропуск отменён");
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" className="danger-text">
          Отменить пропуск
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Отменить пропуск?</DialogTitle>
          <DialogDescription>
            QR-код перестанет действовать, а место освободится.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={revoke}>
          <Field label="Причина отмены">
            <Textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              required
              minLength={3}
              maxLength={240}
              placeholder="Например, планы изменились"
            />
          </Field>
          {error && <ErrorBox text={error} />}
          <Button disabled={busy} className="danger-button">
            Подтвердить отмену
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
