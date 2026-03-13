import type { AuthUser } from "@/lib/auth-store";

const apiBaseURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function authURL(path: string) {
  return `${apiBaseURL}${path}`;
}

export async function fetchCurrentUser(token: string): Promise<AuthUser> {
  const response = await fetch(authURL("/api/v1/auth/me"), {
    headers: {
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });

  if (response.status === 401) {
    throw new Error("unauthorized");
  }
  if (!response.ok) {
    throw new Error(`auth_me_failed:${response.status}`);
  }

  const payload = (await response.json()) as { user: AuthUser };
  return payload.user;
}