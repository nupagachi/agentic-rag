"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useForm } from "react-hook-form";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useLogin } from "@/hooks/use-auth";
import { ApiError } from "@/lib/api/errors";
import { loginSchema, PASSWORD_MIN, type LoginInput } from "@/lib/validations/auth";

export function LoginForm() {
  const t = useTranslations("auth");
  const tv = useTranslations("validation");
  const te = useTranslations("errors");
  const router = useRouter();
  const login = useLogin();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginInput>({ resolver: zodResolver(loginSchema), defaultValues: { email: "", password: "" } });

  const onSubmit = handleSubmit((values) =>
    login.mutate(values, {
      onSuccess: () => router.replace("/chat"),
      onError: (err) => {
        if (err instanceof ApiError && err.isUnauthorized) toast.error(t("loginFailed"));
        else if (err instanceof ApiError && err.status === 0) toast.error(te("network"));
        else toast.error(te("unknown"));
      },
    }),
  );

  const fieldError = (key?: string) =>
    key === "passwordMin" ? tv("passwordMin", { min: PASSWORD_MIN }) : key === "email" ? tv("email") : undefined;

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="email">{t("email")}</Label>
        <Input id="email" type="email" autoComplete="email" aria-invalid={!!errors.email} {...register("email")} />
        {errors.email && <p className="text-sm text-destructive">{fieldError(errors.email.message)}</p>}
      </div>
      <div className="space-y-2">
        <Label htmlFor="password">{t("password")}</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          aria-invalid={!!errors.password}
          {...register("password")}
        />
        {errors.password && <p className="text-sm text-destructive">{fieldError(errors.password.message)}</p>}
      </div>
      <Button type="submit" className="w-full" disabled={login.isPending}>
        {t("submitLogin")}
      </Button>
    </form>
  );
}
