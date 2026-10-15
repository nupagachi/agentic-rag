import axios, { type AxiosRequestConfig, type InternalAxiosRequestConfig } from "axios";

import { env } from "@/config/env";

import { toApiError } from "./errors";
import { tokenStore } from "./token-store";

const REFRESH_PATH = "/auth/refresh";

export const http = axios.create({
  baseURL: env.NEXT_PUBLIC_API_BASE_URL,
  withCredentials: true, // send the httpOnly refresh cookie
  timeout: 30_000,
  headers: { "Content-Type": "application/json" },
});

http.interceptors.request.use((config) => {
  const token = tokenStore.get();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Single-flight refresh: concurrent 401s wait for one /auth/refresh call.
let refreshing: Promise<string | null> | null = null;

export function refreshAccessToken(): Promise<string | null> {
  refreshing ??= axios
    .post<{ access_token: string }>(`${env.NEXT_PUBLIC_API_BASE_URL}${REFRESH_PATH}`, null, {
      withCredentials: true,
    })
    .then((res) => {
      tokenStore.set(res.data.access_token);
      return res.data.access_token;
    })
    .catch(() => {
      tokenStore.clear();
      return null;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean };

http.interceptors.response.use(
  (res) => res,
  async (err) => {
    const config = err.config as RetriableConfig | undefined;
    const isRefreshCall = config?.url?.endsWith(REFRESH_PATH);

    if (err.response?.status === 401 && config && !config._retried && !isRefreshCall) {
      config._retried = true;
      const token = await refreshAccessToken();
      if (token) return http(config);
    }
    throw toApiError(err);
  },
);

/** Typed helpers that unwrap `response.data`. */
export const api = {
  get: <T>(url: string, config?: AxiosRequestConfig) => http.get<T>(url, config).then((r) => r.data),
  post: <T>(url: string, body?: unknown, config?: AxiosRequestConfig) =>
    http.post<T>(url, body, config).then((r) => r.data),
  put: <T>(url: string, body?: unknown, config?: AxiosRequestConfig) =>
    http.put<T>(url, body, config).then((r) => r.data),
  patch: <T>(url: string, body?: unknown, config?: AxiosRequestConfig) =>
    http.patch<T>(url, body, config).then((r) => r.data),
  delete: <T>(url: string, config?: AxiosRequestConfig) => http.delete<T>(url, config).then((r) => r.data),
};
