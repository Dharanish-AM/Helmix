import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const authCookieName = "helmix_token";

export function middleware(request: NextRequest) {
  const { nextUrl, cookies } = request;
  const token = cookies.get(authCookieName)?.value;
  const hasTokenInQuery = nextUrl.searchParams.has("token");

  if (nextUrl.pathname.startsWith("/dashboard") && !token && !hasTokenInQuery) {
    const loginURL = new URL("/login", request.url);
    return NextResponse.redirect(loginURL);
  }

  if (nextUrl.pathname.startsWith("/login") && token) {
    const dashboardURL = new URL("/dashboard", request.url);
    return NextResponse.redirect(dashboardURL);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/login"],
};