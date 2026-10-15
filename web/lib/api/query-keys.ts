/**
 * Central query-key factory. Hierarchical keys let callers invalidate a
 * whole domain (e.g. `queryKeys.kb.all`) or a single entity.
 */
export const queryKeys = {
  me: ["me"] as const,
  kb: {
    all: ["kb"] as const,
    list: (workspaceId: string) => [...queryKeys.kb.all, "list", workspaceId] as const,
    detail: (kbId: string) => [...queryKeys.kb.all, "detail", kbId] as const,
    documents: (kbId: string) => [...queryKeys.kb.all, kbId, "documents"] as const,
  },
  chat: {
    all: ["chat"] as const,
    conversations: () => [...queryKeys.chat.all, "conversations"] as const,
    conversation: (id: string) => [...queryKeys.chat.all, "conversation", id] as const,
  },
};
