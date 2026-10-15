import { AxiosError, AxiosHeaders } from "axios";
import { describe, expect, it } from "vitest";

import { ApiError, toApiError } from "./errors";

function axiosErr(status?: number, data?: unknown) {
  const config = { headers: new AxiosHeaders() };
  const response = status ? { status, data, statusText: "", headers: {}, config } : undefined;
  return new AxiosError("boom", "ERR", config, null, response);
}

describe("toApiError", () => {
  it("maps API error body", () => {
    const err = toApiError(axiosErr(403, { error: { code: "kb_forbidden", message: "no" }, trace_id: "t1" }));
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 403, code: "kb_forbidden", message: "no", traceId: "t1" });
  });

  it("treats missing response as network error", () => {
    expect(toApiError(axiosErr())).toMatchObject({ status: 0, code: "network_error" });
  });

  it("flags 401 as unauthorized", () => {
    expect(toApiError(axiosErr(401, {})).isUnauthorized).toBe(true);
  });
});
