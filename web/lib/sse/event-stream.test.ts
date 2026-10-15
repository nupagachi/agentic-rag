import { afterEach, describe, expect, it, vi } from "vitest";

import { openEventStream, type SseEvent } from "./event-stream";

function sseResponse(chunks: string[]) {
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      const enc = new TextEncoder();
      chunks.forEach((c) => controller.enqueue(enc.encode(c)));
      controller.close();
    },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

afterEach(() => vi.unstubAllGlobals());

describe("openEventStream", () => {
  it("parses events split across chunks, multi-line data and comments", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          sseResponse([
            ": keep-alive\n",
            "id: 1\nevent: step\nda",
            'ta: {"a":1}\n\n',
            "data: line1\r\ndata: line2\r\n\r\n",
          ]),
        ),
    );

    const events: SseEvent[] = [];
    await openEventStream({ path: "/chat/stream", onEvent: (e) => events.push(e) });

    expect(events).toEqual([
      { id: "1", event: "step", data: '{"a":1}' },
      { id: undefined, event: "message", data: "line1\nline2" },
    ]);
  });

  it("reconnects with Last-Event-ID after a dropped connection", async () => {
    vi.useFakeTimers();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          new ReadableStream({
            sent: false,
            pull(c) {
              // First read delivers one event, the next read fails like a dropped socket.
              if (!this.sent) {
                this.sent = true;
                c.enqueue(new TextEncoder().encode("id: 7\ndata: x\n\n"));
              } else c.error(new Error("socket closed"));
            },
          } as UnderlyingDefaultSource<Uint8Array> & { sent: boolean }),
        ),
      )
      .mockResolvedValueOnce(sseResponse(["id: 8\ndata: y\n\n"]));
    vi.stubGlobal("fetch", fetchMock);

    const events: SseEvent[] = [];
    const done = openEventStream({ path: "/s", onEvent: (e) => events.push(e) });
    await vi.runAllTimersAsync();
    await done;
    vi.useRealTimers();

    expect(events.map((e) => e.data)).toEqual(["x", "y"]);
    expect(fetchMock.mock.calls[1][1].headers["Last-Event-ID"]).toBe("7");
  });
});
