import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  ArrowRight,
  Eye,
  EyeOff,
  KeyRound,
  ShieldCheck,
  Ticket,
  UserRound,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { APIError, api, message, post, type Session, type User } from "./api";
import { ErrorBox, Field, Heading, Loading } from "./shared";
import { useSession } from "./session";
import { toast } from "sonner";
export function Auth({ register = false }: { register?: boolean }) {
  const { accept, session } = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [visible, setVisible] = useState(false);
  const next = location.state?.returnTo;
  const returnTo =
    typeof next === "string" && next.startsWith("/") && !next.startsWith("//")
      ? next
      : "/manage";
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setFields({});
    const form = new FormData(e.currentTarget);
    const data = {
      email: form.get("email"),
      password: form.get("password"),
      ...(register ? { name: form.get("name") } : {}),
    };
    try {
      const result = await post<Session>(
        register ? "/auth/register" : "/auth/login",
        data,
      );
      accept(result);
      navigate(returnTo, { replace: true });
      toast.success(register ? "Аккаунт создан" : "Вы вошли в аккаунт");
    } catch (e) {
      setError(message(e));
      if (e instanceof APIError) setFields(e.fields);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-layout">
      <div className="auth-intro">
        <span className="brand-mark">
          <Ticket size={30} />
        </span>
        <p className="eyebrow">ВАШ КАМПУС</p>
        <h1>
          {register
            ? "События, в которых хочется участвовать."
            : "С возвращением."}
        </h1>
        <p>Организуйте встречи, сохраняйте пропуска и встречайте участников.</p>
        <div className="auth-perks">
          <span>
            <Ticket />
            Пропуска на любом устройстве
          </span>
          <span>
            <UserRound />
            Один аккаунт для всех ролей
          </span>
          <span>
            <ShieldCheck />
            Контроль входа без повторных проходов
          </span>
        </div>
      </div>
      <section className="auth-card">
        <h2>{register ? "Создать аккаунт" : "Войти в аккаунт"}</h2>
        <p>
          {register
            ? "Email используется для входа."
            : "Введите email и пароль."}
        </p>
        {session.user && (
          <p className="form-note">Сейчас вы вошли как {session.user.name}.</p>
        )}
        <form onSubmit={submit}>
          {register && (
            <Field label="Ваше имя" error={fields.name}>
              <Input
                name="name"
                autoComplete="name"
                required
                minLength={2}
                maxLength={120}
                placeholder="Имя и фамилия"
              />
            </Field>
          )}
          <Field label="Email" error={fields.email}>
            <Input
              name="email"
              type="email"
              autoComplete="email"
              required
              maxLength={254}
              placeholder="you@example.com"
            />
          </Field>
          <Field
            label="Пароль"
            error={fields.password}
            hint={
              register
                ? "Не менее 12 символов. Подойдёт длинная запоминающаяся фраза."
                : undefined
            }
          >
            <span className="password-input">
              <Input
                name="password"
                type={visible ? "text" : "password"}
                autoComplete={register ? "new-password" : "current-password"}
                required
                minLength={register ? 12 : 1}
                maxLength={128}
              />
              <button
                type="button"
                aria-label={visible ? "Скрыть пароль" : "Показать пароль"}
                onClick={() => setVisible((v) => !v)}
              >
                {visible ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </span>
          </Field>
          {error && <ErrorBox text={error} />}
          <Button type="submit" size="lg" disabled={busy}>
            {busy ? "Подождите…" : register ? "Создать аккаунт" : "Войти"}
            <ArrowRight size={18} />
          </Button>
        </form>
        <p className="auth-switch">
          {register ? "Уже есть аккаунт?" : "Первый раз здесь?"}{" "}
          <Link to={register ? "/login" : "/register"} state={{ returnTo }}>
            {register ? "Войти" : "Зарегистрироваться"}
          </Link>
        </p>
      </section>
    </div>
  );
}
export function Profile() {
  const { session, refresh } = useSession();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [passwordError, setPasswordError] = useState("");
  const [changing, setChanging] = useState(false);
  async function save(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      await api<User>("/me", {
        method: "PATCH",
        body: JSON.stringify({ name: f.get("name") }),
      });
      await refresh();
      toast.success("Имя обновлено");
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function password(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    setChanging(true);
    setPasswordError("");
    const f = new FormData(form);
    try {
      await post("/auth/password", {
        current_password: f.get("current"),
        password: f.get("password"),
      });
      form.reset();
      toast.success("Пароль изменён. Другие сеансы завершены.");
    } catch (e) {
      setPasswordError(message(e));
    } finally {
      setChanging(false);
    }
  }
  return (
    <>
      <Heading eyebrow="АККАУНТ" title="Личные настройки" />
      <div className="settings-grid">
        <section className="panel">
          <h2>
            <UserRound size={22} />
            Профиль
          </h2>
          <form onSubmit={save}>
            <Field label="Имя">
              <Input
                name="name"
                defaultValue={session.user?.name}
                minLength={2}
                maxLength={120}
                required
              />
            </Field>
            <Field label="Email" hint="Используется для входа в аккаунт.">
              <Input value={session.user?.email ?? ""} disabled />
            </Field>
            {error && <ErrorBox text={error} />}
            <Button disabled={busy}>Сохранить изменения</Button>
          </form>
        </section>
        <section className="panel">
          <h2>
            <KeyRound size={22} />
            Сменить пароль
          </h2>
          <form onSubmit={password}>
            <Field label="Текущий пароль">
              <Input
                name="current"
                type="password"
                autoComplete="current-password"
                required
                maxLength={128}
              />
            </Field>
            <Field
              label="Новый пароль"
              hint="После смены пароля все остальные сеансы завершатся."
            >
              <Input
                name="password"
                type="password"
                autoComplete="new-password"
                minLength={12}
                maxLength={128}
                required
              />
            </Field>
            {passwordError && <ErrorBox text={passwordError} />}
            <Button disabled={changing}>Изменить пароль</Button>
          </form>
        </section>
      </div>
    </>
  );
}
export function Join() {
  const location = useLocation();
  const navigate = useNavigate();
  const { session } = useSession();
  const token = location.hash.slice(1);
  const [invite, setInvite] = useState<{
    event_title: string;
    role: string;
    label: string;
  } | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let active = true;
    post<{ event_title: string; role: string; label: string }>(
      "/invitations/inspect",
      { token },
    )
      .then((v) => {
        if (active) setInvite(v);
      })
      .catch((e) => {
        if (active) setError(message(e));
      });
    return () => {
      active = false;
    };
  }, [token]);
  async function accept() {
    setBusy(true);
    try {
      const result = await post<{ event_id: string; role: string }>(
        "/invitations/accept",
        { token },
      );
      toast.success("Вы присоединились к команде");
      navigate(
        result.role === "controller"
          ? `/control/${result.event_id}`
          : `/manage/${result.event_id}`,
        { replace: true },
      );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  if (error) return <ErrorBox text={error} />;
  if (!invite) return <Loading />;
  return (
    <section className="invite-card">
      <ShieldCheck size={44} />
      <p className="eyebrow">ПРИГЛАШЕНИЕ В КОМАНДУ</p>
      <h1>{invite.event_title}</h1>
      <p>
        Вас приглашают в роли{" "}
        <strong>
          {invite.role === "controller" ? "контролёра" : "соорганизатора"}
        </strong>
        .
      </p>
      <span>{invite.label}</span>
      {session.user ? (
        <Button size="lg" disabled={busy} onClick={accept}>
          Присоединиться к команде
          <ArrowRight size={18} />
        </Button>
      ) : (
        <Button asChild size="lg">
          <Link to="/login" state={{ returnTo: "/join#" + token }}>
            Войти и принять приглашение
          </Link>
        </Button>
      )}
    </section>
  );
}
