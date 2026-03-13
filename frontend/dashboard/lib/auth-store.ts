"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

export type AuthUser = {
  id: string;
  github_id: number;
  username: string;
  email: string;
  avatar_url: string;
  org_id: string;
  org_name: string;
  role: string;
  created_at: string;
  token_updated_at: string;
};

type AuthState = {
  token: string | null;
  user: AuthUser | null;
  setSession: (token: string, user: AuthUser | null) => void;
  clearSession: () => void;
};

const authCookieName = "helmix_token";

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      setSession: (token, user) => {
        set({ token, user });
        setAuthCookie(token);
      },
      clearSession: () => {
        clearAuthCookie();
        set({ token: null, user: null });
      },
    }),
    {
      name: "helmix-auth",
      storage: createJSONStorage(() => localStorage),
      partialize: ({ token, user }) => ({ token, user }),
    },
  ),
);

export function setAuthCookie(token: string) {
  if (typeof document === "undefined") {
    return;
  }
  document.cookie = `${authCookieName}=${encodeURIComponent(token)}; path=/; max-age=${60 * 60 * 24}; samesite=lax`;
}

export function clearAuthCookie() {
  if (typeof document === "undefined") {
    return;
  }
  document.cookie = `${authCookieName}=; path=/; max-age=0; samesite=lax`;
}

export function authCookie() {
  return authCookieName;
}