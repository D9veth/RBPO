import { useEffect, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Camera,
  Check,
  CircleAlert,
  Clock3,
  RefreshCw,
  ScanLine,
  ShieldCheck,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  type Activity,
  type Admission,
  type EventRecord,
  formatTime,
  formatDate,
} from "@/lib/domain";
import { message, post } from "./api";
import { Back, ErrorBox, Heading, Loading, useData } from "./shared";
const results: Record<string, string> = {
  accepted: "Проход разрешён",
  already_used: "Повторный проход",
  revoked: "Пропуск отменён",
  wrong_event: "Другое мероприятие",
  not_found: "Пропуск не найден",
  invalid: "Некорректный код",
  not_open: "Вход ещё не открыт",
  closed: "Мероприятие завершено",
  cancelled: "Вход закрыт",
};
export function Control() {
  const { id } = useParams();
  const {
    data: e,
    error,
    loading,
    reload,
  } = useData<EventRecord>(`/events/${id}`);
  const activity = useData<Activity>(
    e && e.role ? `/events/${id}/activity` : null,
  );
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<Admission | null>(null);
  const [scanError, setScanError] = useState("");
  const [camera, setCamera] = useState(false);
  const [cameraError, setCameraError] = useState("");
  const [cameraReady, setCameraReady] = useState(false);
  const video = useRef<HTMLVideoElement>(null);
  const processing = useRef(false);
  const pending = useRef<{ code: string; key: string } | null>(null);
  const submitRef = useRef<(code: string) => void>(() => {});
  async function scan(value: string) {
    if (processing.current) return;
    processing.current = true;
    setBusy(true);
    setResult(null);
    setScanError("");
    setCode(value);
    setCamera(false);
    if (pending.current?.code !== value)
      pending.current = { code: value, key: crypto.randomUUID() };
    try {
      const response = await post<Admission>(
        `/events/${id}/check-in`,
        { code: value },
        pending.current.key,
      );
      setResult(response);
      pending.current = null;
      reload();
      activity.reload();
      if (response.result === "accepted" && navigator.vibrate)
        navigator.vibrate(120);
    } catch (e) {
      setScanError(message(e));
    } finally {
      setBusy(false);
      processing.current = false;
    }
  }
  submitRef.current = (value) => {
    void scan(value);
  };
  useEffect(() => {
    if (!camera) return;
    let cancelled = false;
    let controls: { stop: () => void } | undefined;
    setCameraReady(false);
    setCameraError("");
    import("@zxing/browser")
      .then(async ({ BrowserQRCodeReader }) => {
        if (cancelled || !video.current) return;
        const reader = new BrowserQRCodeReader(undefined, {
          delayBetweenScanAttempts: 200,
        });
        try {
          const running = await reader.decodeFromVideoDevice(
            undefined,
            video.current,
            (result, _error, c) => {
              if (result && !cancelled) {
                c.stop();
                submitRef.current(result.getText());
              }
            },
          );
          controls = running;
          if (cancelled) running.stop();
          else setCameraReady(true);
        } catch (e) {
          if (!cancelled) {
            setCamera(false);
            setCameraError(
              e instanceof DOMException && e.name === "NotAllowedError"
                ? "Доступ к камере запрещён. Разрешите его в браузере или введите код вручную."
                : "Камера недоступна. Попробуйте другую камеру или введите код вручную.",
            );
          }
        }
      })
      .catch(() => {
        if (!cancelled) {
          setCamera(false);
          setCameraError(
            "Не удалось загрузить сканер. Обновите страницу или введите код вручную.",
          );
        }
      });
    return () => {
      cancelled = true;
      controls?.stop();
    };
  }, [camera]);
  if (loading && !e) return <Loading />;
  if (error || !e)
    return <ErrorBox text={error || "Мероприятие не найдено"} retry={reload} />;
  if (!e.role)
    return (
      <ErrorBox text="Для проверки пропусков нужно приглашение в команду мероприятия." />
    );
  const stateEvent = activity.data?.event ?? e;
  return (
    <>
      <Back to="/control" label="Выбрать другое мероприятие" />
      <Heading
        eyebrow="КОНТРОЛЬ ВХОДА"
        title={e.title}
        description={`${formatDate(e.starts_at)} · вход с ${formatTime(e.doors_at)} до ${formatTime(e.ends_at)} МСК`}
        action={
          <div className="admission-count">
            <strong>{stateEvent.checked_in}</strong>
            <span>прошли из {stateEvent.issued}</span>
          </div>
        }
      />
      <div className="control-layout">
        <section className="scan-panel">
          <div className="scan-panel-title">
            <ScanLine size={23} />
            <h2>Проверить пропуск</h2>
            <span className={`status-pill ${scanError ? "revoked" : "active"}`}>
              {scanError ? "Нет связи" : "Онлайн"}
            </span>
          </div>
          {camera ? (
            <div className="camera-frame">
              <video ref={video} muted playsInline autoPlay />
              <div className="scan-target" />
              {!cameraReady && (
                <span className="camera-hint">Подключаем камеру…</span>
              )}
              <Button variant="secondary" onClick={() => setCamera(false)}>
                <X size={17} />
                Закрыть камеру
              </Button>
            </div>
          ) : (
            <button
              type="button"
              className="camera-start"
              disabled={busy}
              onClick={() => {
                setResult(null);
                setCamera(true);
              }}
            >
              <span className="camera-icon">
                <Camera size={36} />
              </span>
              <strong>Сканировать QR-код</strong>
              <span>Наведите камеру на пропуск участника</span>
            </button>
          )}
          {cameraError && <ErrorBox text={cameraError} />}
          <div className="scan-divider">
            <span />
            или введите код
            <span />
          </div>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void scan(code);
            }}
          >
            <label htmlFor="admission-code" className="sr-only">
              Код пропуска
            </label>
            <Input
              id="admission-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              placeholder="XXXX-XXXX-XXXX-XXXX-…"
              required
              maxLength={100}
              disabled={busy}
              className="code-input"
            />
            <Button size="lg" type="submit" disabled={busy || !code.trim()}>
              {busy ? "Проверяем…" : "Погасить пропуск"}
              <ShieldCheck size={19} />
            </Button>
          </form>
          <p className="scan-footnote">
            Успешная проверка погашает пропуск. Повторный проход будет отклонён.
          </p>
        </section>
        <section
          className="scan-feedback"
          aria-live="polite"
          aria-atomic="true"
        >
          {busy ? (
            <div className="feedback-idle">
              <RefreshCw size={40} className="animate-spin" />
              <h2>Проверяем пропуск</h2>
              <p>Дождитесь ответа сервера.</p>
            </div>
          ) : scanError ? (
            <div className="feedback-result result-network">
              <CircleAlert size={52} />
              <h2>Ответ не получен</h2>
              <p>{scanError}</p>
              <Button onClick={() => scan(pending.current?.code ?? code)}>
                <RefreshCw size={17} />
                Повторить запрос
              </Button>
              <small>Повтор не засчитает проход дважды.</small>
            </div>
          ) : result ? (
            <div className={`feedback-result result-${result.result}`}>
              <span className="feedback-symbol">
                {result.result === "accepted" ? (
                  <Check size={48} />
                ) : result.result === "already_used" ? (
                  <Clock3 size={48} />
                ) : (
                  <X size={48} />
                )}
              </span>
              <p className="eyebrow">
                {result.result === "accepted"
                  ? "ПРОПУСК ПОДТВЕРЖДЁН"
                  : "ПРОХОД НЕ РАЗРЕШЁН"}
              </p>
              <h2>{results[result.result] ?? result.message}</h2>
              {result.pass && <strong>{result.pass.holder_name}</strong>}
              <p>{result.message}</p>
              {result.pass?.redeemed_at && (
                <span>Вход: {formatTime(result.pass.redeemed_at)} МСК</span>
              )}
              <Button
                variant="outline"
                onClick={() => {
                  setResult(null);
                  setCode("");
                  pending.current = null;
                  document.getElementById("admission-code")?.focus();
                }}
              >
                Следующий участник
              </Button>
            </div>
          ) : (
            <div className="feedback-idle">
              <ShieldCheck size={52} />
              <h2>Готовы встречать гостей</h2>
              <p>Результат проверки появится здесь.</p>
            </div>
          )}
        </section>
      </div>
      <section className="panel recent-scans">
        <div className="section-toolbar">
          <h2>Последние проверки</h2>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              activity.reload();
              reload();
            }}
          >
            <RefreshCw size={16} />
            Обновить
          </Button>
        </div>
        {activity.error ? (
          <ErrorBox text={activity.error} retry={activity.reload} />
        ) : !activity.data?.attempts.length ? (
          <p className="muted">
            На этом мероприятии ещё не проверяли пропуска.
          </p>
        ) : (
          <div className="attempt-list">
            {activity.data.attempts.slice(0, 12).map((a) => (
              <div className="attempt-row" key={a.id}>
                <span
                  className={`attempt-symbol ${a.result === "accepted" ? "accepted" : "rejected"}`}
                >
                  {a.result === "accepted" ? (
                    <Check size={17} />
                  ) : (
                    <X size={17} />
                  )}
                </span>
                <div>
                  <strong>
                    {a.holder_name || results[a.result] || "Проверка пропуска"}
                  </strong>
                  <small>
                    {results[a.result]} · {a.actor}
                  </small>
                </div>
                <time>{formatTime(a.created_at)}</time>
              </div>
            ))}
          </div>
        )}
      </section>
    </>
  );
}
