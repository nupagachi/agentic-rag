import { api } from "../client";
import { tokenStore } from "../token-store";

import type { LoginInput } from "@/lib/validations/auth";
import type { CurrentUser } from "@/types/user";

interface LoginResponse {
  access_token: string;
  user: CurrentUser;
}

export const authService = {
  async login(input: LoginInput) {
    const res = await api.post<LoginResponse>("/auth/login", input);
    tokenStore.set(res.access_token);
    return res.user;
  },
  async logout() {
    try {
      await api.post("/auth/logout");
    } finally {
      tokenStore.clear();
    }
  },
  me: () => api.get<CurrentUser>("/me"),
};
