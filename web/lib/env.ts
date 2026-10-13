/**
 * Noi DUY NHAT doc `process.env` trong web/ (SR-14). Chi doc bien cong khai
 * `NEXT_PUBLIC_API_URL`; Next.js thay gia tri luc build, nen phai truy cap
 * bang ten day du (khong destructure, khong tra theo khoa dong).
 */
const DEFAULT_API_URL = "http://127.0.0.1:8080";

export function parseApiUrl(raw: string | undefined): string {
  const value = raw === undefined || raw.trim() === "" ? DEFAULT_API_URL : raw.trim();
  const url = new URL(value);
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error("NEXT_PUBLIC_API_URL must use http or https");
  }
  return url.origin;
}

export const env = {
  apiUrl: parseApiUrl(process.env.NEXT_PUBLIC_API_URL),
} as const;
