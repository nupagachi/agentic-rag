import { env } from "@/config/env";
import { refreshAccessToken } from "@/lib/api/client";
import { tokenStore } from "@/lib/api/token-store";

export interface SseEvent {
  id?: string;
  event: string;
  data: string;
}

export interface EventStreamOptions {
  path: string;
  method?: "GET" | "POST";
  body?: unknown;
  onEvent: (event: SseEvent) => void;
  onError?: (error: unknown) => void;
  signal?: AbortSignal;
  maxRetries?: number;
}

/**
 * fetch-based SSE reader. Native EventSource cannot send an Authorization
 * header or a POST body, so we parse the stream ourselves and resume with
 * `Last-Event-ID` after a dropped connection (exponential backoff).
 */
export async function openEventStream(opts: EventStreamOptions): Promise<void> {
  const { maxRetries = 5 } = opts;
  let lastEventId: string | undefined;
  let attempt = 0;
  let refreshed = false;

  while (!opts.signal?.aborted) {
    try {
      const res = await fetch(`${env.NEXT_PUBLIC_API_BASE_URL}${opts.path}`, {
        method: opts.method ?? "GET",
        credentials: "include",
        signal: opts.signal,
        headers: {
          Accept: "text/event-stream",
          ...(opts.body !== undefined && { "Content-Type": "application/json" }),
          ...(tokenStore.get() && { Authorization: `Bearer ${tokenStore.get()}` }),
          ...(lastEventId && { "Last-Event-ID": lastEventId }),
        },
        body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      });

      if (res.status === 401 && !refreshed) {
        refreshed = true;
        if (await refreshAccessToken()) continue;
      }
      if (!res.ok || !res.body) throw new Error(`SSE ${res.status}`);

      attempt = 0;
      for await (const event of parseSse(res.body)) {
        if (event.id) lastEventId = event.id;
        opts.onEvent(event);
      }
      return; // server closed the stream cleanly
    } catch (err) {
      if (opts.signal?.aborted) return;
      if (++attempt > maxRetries) {
        opts.onError?.(err);
        return;
      }
      await new Promise((r) => setTimeout(r, Math.min(1000 * 2 ** (attempt - 1), 15_000)));
    }
  }
}

async function* parseSse(stream: ReadableStream<Uint8Array>): AsyncGenerator<SseEvent> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let current: Partial<SseEvent> & { data?: string } = {};

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let newline: number;
    while ((newline = buffer.search(/\r?\n/)) !== -1) {
      const line = buffer.slice(0, newline);
      buffer = buffer.slice(buffer[newline] === "\r" ? newline + 2 : newline + 1);

      if (line === "") {
        if (current.data !== undefined) {
          yield { id: current.id, event: current.event ?? "message", data: current.data };
        }
        current = {};
        continue;
      }
      if (line.startsWith(":")) continue; // comment / keep-alive

      const colon = line.indexOf(":");
      const field = colon === -1 ? line : line.slice(0, colon);
      const val = colon === -1 ? "" : line.slice(colon + 1).replace(/^ /, "");
      if (field === "data") current.data = current.data === undefined ? val : `${current.data}\n${val}`;
      else if (field === "event") current.event = val;
      else if (field === "id") current.id = val;
    }
  }
}
