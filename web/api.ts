export type User = { id: string; email: string; name: string };
export type Session = { user: User | null; csrf_token: string };
let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
export class APIError extends Error {
  constructor(
    message: string,
    public code: string,
    public status: number,
    public fields: Record<string, string> = {},
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v1${path}`, {
      ...options,
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        ...(csrf ? { "X-CSRF-Token": csrf } : {}),
        ...options.headers,
      },
      signal: options.signal ?? AbortSignal.timeout(20000),
    });
  } catch (e) {
    if (options.signal?.aborted) throw e;
    throw new APIError(
      "Нет ответа от сервера. Проверьте соединение и повторите запрос.",
      "NETWORK",
      0,
    );
  }
  const payload = await response.json().catch(() => null);
  if (payload?.error)
    throw new APIError(
      payload.error.message,
      payload.error.code,
      response.status,
      payload.error.fields,
    );
  if (!payload || !Object.hasOwn(payload, "data"))
    throw new APIError(
      "Сервис временно недоступен. Попробуйте ещё раз.",
      "UNAVAILABLE",
      response.status,
    );
  return payload.data as T;
}
export function post<T>(path: string, data: unknown, requestKey?: string) {
  return api<T>(path, {
    method: "POST",
    body: JSON.stringify(data),
    headers: requestKey ? { "Idempotency-Key": requestKey } : {},
  });
}
export function message(error: unknown) {
  return error instanceof Error
    ? error.message
    : "Не удалось выполнить действие.";
}
export type Page<T> = {
  items: T[];
  total: number;
  page: number;
  limit: number;
};
