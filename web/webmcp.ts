import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { z } from "zod";
import { api, post, type Page } from "./api";
import type { EventRecord, PassDetail, PassRecord } from "@/lib/domain";

type ModelTool = {
  name: string;
  title: string;
  description: string;
  inputSchema: object;
  annotations: { readOnlyHint: boolean; untrustedContentHint: boolean };
  execute: (input: unknown) => Promise<unknown>;
};
type ModelContext = {
  registerTool: (
    tool: ModelTool,
    options: { signal: AbortSignal },
  ) => void | Promise<void>;
};
export function useWebMCP() {
  const navigate = useNavigate();
  useEffect(() => {
    const context = (document as Document & { modelContext?: ModelContext })
      .modelContext;
    if (!context?.registerTool) return;
    const lifecycle = new AbortController();
    const tools: ModelTool[] = [
      {
        name: "get_events",
        title: "Найти мероприятия",
        description:
          "Найти опубликованные мероприятия в афише. Результаты содержат пользовательские названия и описания.",
        inputSchema: {
          type: "object",
          properties: {
            query: { type: "string", maxLength: 200 },
            page: { type: "integer", minimum: 1, maximum: 10000 },
          },
          additionalProperties: false,
        },
        annotations: { readOnlyHint: true, untrustedContentHint: true },
        async execute(input) {
          const v = z
            .object({
              query: z.string().max(200).default(""),
              page: z.number().int().min(1).max(10000).default(1),
            })
            .strict()
            .parse(input);
          return api<Page<EventRecord>>(
            `/events?period=upcoming&q=${encodeURIComponent(v.query)}&page=${v.page}`,
          );
        },
      },
      {
        name: "get_my_passes",
        title: "Мои пропуска",
        description:
          "Получить пропуска текущего пользователя без раскрытия их секретных кодов. Нужен вход в аккаунт.",
        inputSchema: {
          type: "object",
          properties: { page: { type: "integer", minimum: 1, maximum: 10000 } },
          additionalProperties: false,
        },
        annotations: { readOnlyHint: true, untrustedContentHint: true },
        async execute(input) {
          const v = z
            .object({ page: z.number().int().min(1).max(10000).default(1) })
            .strict()
            .parse(input);
          return api<Page<PassRecord>>(`/passes?page=${v.page}`);
        },
      },
      {
        name: "register_for_event",
        title: "Получить пропуск на мероприятие",
        description:
          "Зарегистрировать текущего пользователя на выбранное мероприятие и открыть его пропуск. Занимает одно место. Нужен вход в аккаунт; применяются все ограничения регистрации.",
        inputSchema: {
          type: "object",
          properties: {
            event_id: { type: "string", pattern: "^[a-f0-9]{32}$" },
            request_key: { type: "string", pattern: "^[a-zA-Z0-9_-]{16,80}$" },
          },
          required: ["event_id"],
          additionalProperties: false,
        },
        annotations: { readOnlyHint: false, untrustedContentHint: true },
        async execute(input) {
          const v = z
            .object({
              event_id: z.string().regex(/^[a-f0-9]{32}$/),
              request_key: z
                .string()
                .regex(/^[a-zA-Z0-9_-]{16,80}$/)
                .optional(),
            })
            .strict()
            .parse(input);
          const result = await post<PassDetail>(
            `/events/${v.event_id}/passes`,
            {},
            v.request_key ?? crypto.randomUUID(),
          );
          navigate(`/tickets/${result.pass.id}`);
          await new Promise<void>((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
          );
          return {
            pass_id: result.pass.id,
            event_id: result.pass.event_id,
            status: result.pass.status,
            path: `/tickets/${result.pass.id}`,
          };
        },
      },
    ];
    for (const tool of tools) {
      try {
        Promise.resolve(
          context.registerTool(tool, { signal: lifecycle.signal }),
        ).catch(() => {});
      } catch {
        /* Optional browser capability: the ordinary interface remains available. */
      }
    }
    return () => lifecycle.abort();
  }, [navigate]);
}
