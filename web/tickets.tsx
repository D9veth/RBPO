import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  ArrowUpRight,
  CalendarDays,
  Copy,
  Download,
  MapPin,
  Printer,
  Ticket,
  ShieldCheck,
} from "lucide-react";
import QRCode from "qrcode";
import { Button } from "@/components/ui/button";
import {
  type PassDetail,
  type PassRecord,
  formatDate,
  formatTime,
  passStates,
} from "@/lib/domain";
import { type Page } from "./api";
import {
  Back,
  Empty,
  ErrorBox,
  Heading,
  Loading,
  Pager,
  useData,
} from "./shared";
import { copy, RevokePass } from "./manage";
export function MyTickets() {
  const [page, setPage] = useState(1);
  const { data, error, loading, reload } = useData<Page<PassRecord>>(
    `/passes?page=${page}`,
  );
  return (
    <>
      <Heading
        eyebrow="ЛИЧНАЯ КОЛЛЕКЦИЯ"
        title="Мои пропуска"
        description="Откройте пропуск и покажите QR-код на входе."
      />
      {error ? (
        <ErrorBox text={error} retry={reload} />
      ) : loading ? (
        <Loading />
      ) : !data?.items.length ? (
        <Empty
          title="Планы на новые впечатления"
          description="Выберите событие на афише — ваш пропуск появится здесь."
        >
          <Button asChild>
            <Link to="/">
              Перейти к мероприятиям
              <ArrowUpRight size={18} />
            </Link>
          </Button>
        </Empty>
      ) : (
        <div className="tickets-grid">
          {data.items.map((p) => (
            <Link
              key={p.id}
              to={`/tickets/${p.id}`}
              className={`wallet-ticket color-${p.color}`}
            >
              <div className="wallet-ticket-top">
                <Ticket size={25} />
                <span
                  className={`status-pill ${p.event_status === "cancelled" ? "revoked" : p.status}`}
                >
                  {p.event_status === "cancelled"
                    ? "Событие отменено"
                    : passStates[p.status]}
                </span>
              </div>
              <h2>{p.event_title}</h2>
              <p>
                <CalendarDays size={17} />
                {formatDate(p.starts_at)} · {formatTime(p.starts_at)}
              </p>
              <p>
                <MapPin size={17} />
                {p.event_location}
              </p>
              <div className="ticket-perforation" />
              <div className="wallet-ticket-bottom">
                <span>{p.holder_name}</span>
                <ArrowUpRight size={22} />
              </div>
            </Link>
          ))}
        </div>
      )}
      {data && <Pager data={data} onPage={setPage} />}
    </>
  );
}
export function TicketDetail() {
  const { id } = useParams();
  const { data, error, loading, reload } = useData<PassDetail>(`/passes/${id}`);
  const [qr, setQR] = useState("");
  const [qrError, setQRError] = useState("");
  useEffect(() => {
    if (!data) return;
    let active = true;
    setQRError("");
    QRCode.toDataURL(data.qr_payload, {
      width: 600,
      margin: 3,
      errorCorrectionLevel: "M",
      color: { dark: "#101c37", light: "#ffffff" },
    })
      .then((value) => {
        if (active) setQR(value);
      })
      .catch(() => {
        if (active)
          setQRError(
            "Не удалось подготовить QR-код. Можно использовать текстовый код.",
          );
      });
    return () => {
      active = false;
    };
  }, [data]);
  if (loading) return <Loading />;
  if (error || !data)
    return <ErrorBox text={error || "Пропуск не найден"} retry={reload} />;
  const p = data.pass,
    active =
      p.status === "active" &&
      p.event_status === "published" &&
      Date.now() < new Date(p.ends_at).getTime();
  return (
    <>
      <div className="no-print">
        <Back to="/tickets" label="Мои пропуска" />
      </div>
      <div className="ticket-detail-layout">
        <section className={`full-ticket color-${p.color}`}>
          <div className="full-ticket-header">
            <span className="ticket-brand">
              <Ticket size={22} />
              кампус
            </span>
            <span className="ticket-type">ПЕРСОНАЛЬНЫЙ ПРОПУСК</span>
            <h1>{p.event_title}</h1>
            <div>
              <CalendarDays size={17} />
              {formatDate(p.starts_at, { year: "numeric" })} ·{" "}
              {formatTime(p.starts_at)} МСК
            </div>
            <div>
              <MapPin size={17} />
              {p.event_location}
            </div>
          </div>
          <div className="ticket-perforation" />
          <div className="full-ticket-body">
            <span
              className={`status-pill ${active ? "active" : p.status === "redeemed" ? "redeemed" : "revoked"}`}
            >
              {p.event_status === "cancelled"
                ? "Мероприятие отменено"
                : p.status === "active" && !active
                  ? "Срок действия истёк"
                  : passStates[p.status]}
            </span>
            <p className="holder-label">УЧАСТНИК</p>
            <h2>{p.holder_name}</h2>
            {active ? (
              <>
                <div className="qr-frame">
                  {qr ? (
                    <img
                      src={qr}
                      width={260}
                      height={260}
                      alt="QR-код пропуска для проверки контролёром"
                    />
                  ) : (
                    <div className="qr-placeholder">Готовим QR-код…</div>
                  )}
                </div>
                {qrError && <ErrorBox text={qrError} />}
                <code className="pass-code">{data.code}</code>
                <p className="ticket-help">
                  Покажите QR-код контролёру.
                  <br />
                  Код действует только для одного прохода.
                </p>
              </>
            ) : (
              <div className="invalid-ticket">
                <ShieldCheck size={48} />
                <h3>
                  {p.status === "redeemed"
                    ? "Вы на мероприятии"
                    : "Пропуск недействителен"}
                </h3>
                <p>
                  {p.redeemed_at
                    ? `Вход подтверждён ${formatDate(p.redeemed_at, { month: "short" })} в ${formatTime(p.redeemed_at)}`
                    : (p.revoke_reason ?? "По этому пропуску войти нельзя.")}
                </p>
              </div>
            )}
          </div>
        </section>
        <aside className="ticket-actions no-print">
          <h2>Готовы к событию</h2>
          <p>
            Сохраните QR-код заранее. Для проверки на входе контролёру
            потребуется интернет.
          </p>
          {active && (
            <>
              <Button variant="outline" onClick={() => copy(data.code)}>
                <Copy size={18} />
                Скопировать код
              </Button>
              <Button variant="outline" asChild disabled={!qr}>
                <a
                  href={qr || undefined}
                  download={`campus-pass-${p.id.slice(0, 8)}.png`}
                >
                  <Download size={18} />
                  Скачать QR-код
                </a>
              </Button>
              <Button variant="outline" onClick={() => window.print()}>
                <Printer size={18} />
                Распечатать пропуск
              </Button>
            </>
          )}
          <Button asChild>
            <Link to={`/events/${p.event_id}`}>
              О мероприятии
              <ArrowUpRight size={17} />
            </Link>
          </Button>
          {p.status === "active" && <RevokePass pass={p} onDone={reload} />}
          <div className="registration-note">
            <ShieldCheck size={19} />
            <span>
              Не публикуйте код: любой человек с его копией может предъявить
              пропуск первым.
            </span>
          </div>
        </aside>
      </div>
    </>
  );
}
