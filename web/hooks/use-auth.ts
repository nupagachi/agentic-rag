"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { queryKeys } from "@/lib/api/query-keys";
import { authService } from "@/lib/api/services/auth.service";

export function useCurrentUser() {
  return useQuery({ queryKey: queryKeys.me, queryFn: authService.me });
}

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: authService.login,
    onSuccess: (user) => queryClient.setQueryData(queryKeys.me, user),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: authService.logout,
    onSettled: () => queryClient.clear(),
  });
}
