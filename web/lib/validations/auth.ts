import { z } from "zod";

/**
 * Schemas hold i18n message keys (under `validation.*`), not text; the form
 * layer translates them. Keeps schemas usable on server and client.
 */
export const PASSWORD_MIN = 8;

export const loginSchema = z.object({
  email: z.email({ error: "email" }),
  password: z.string().min(PASSWORD_MIN, { error: "passwordMin" }),
});

export type LoginInput = z.infer<typeof loginSchema>;
