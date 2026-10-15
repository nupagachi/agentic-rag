import { isAxiosError } from "axios";

/**
 * Error body returned by the Go API. Codes come from contracts/errors.yaml;
 * keep this shape in sync with that file.
 */
export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  };
  trace_id?: string;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly details?: Record<string, unknown>,
    readonly traceId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }

  get isUnauthorized() {
    return this.status === 401;
  }
}

/** Status 0 means the request never got a response (network, CORS, abort). */
export function toApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err;
  if (isAxiosError<ApiErrorBody>(err)) {
    const status = err.response?.status ?? 0;
    const body = err.response?.data?.error;
    return new ApiError(
      status,
      body?.code ?? (status === 0 ? "network_error" : "unknown_error"),
      body?.message ?? err.message,
      body?.details,
      err.response?.data?.trace_id,
    );
  }
  return new ApiError(0, "unknown_error", err instanceof Error ? err.message : String(err));
}
