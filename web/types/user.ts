import type { Locale } from "@/i18n/config";

// TODO: replace with types generated from contracts/openapi.yaml.
export interface CurrentUser {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  locale: Locale;
  theme: "light" | "dark" | "system";
  is_system_admin: boolean;
}
