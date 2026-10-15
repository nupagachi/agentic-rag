import { z } from "zod";

/**
 * Public env, validated once at module load. Only NEXT_PUBLIC_* values reach
 * the browser; they are inlined at build time, so each key must be read
 * explicitly (no dynamic process.env access).
 */
const publicEnvSchema = z.object({
  NEXT_PUBLIC_API_BASE_URL: z.string().min(1).default("/api/v1"),
  NEXT_PUBLIC_APP_NAME: z.string().min(1).default("Agentic RAG"),
});

export const env = publicEnvSchema.parse({
  NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
  NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME,
});
