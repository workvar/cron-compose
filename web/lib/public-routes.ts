/** Routes that render without requiring a signed-in session. */

export function isPublicPath(pathname: string): boolean {
  if (pathname === "/login" || pathname.startsWith("/login/")) return true;
  if (pathname === "/landing" || pathname.startsWith("/landing/")) return true;
  if (pathname === "/docs" || pathname.startsWith("/docs/")) return true;
  if (pathname === "/use-cases" || pathname.startsWith("/use-cases/")) return true;
  return false;
}

export function publicActiveNav(
  pathname: string,
): "home" | "use-cases" | "docs" | undefined {
  if (pathname === "/landing" || pathname.startsWith("/landing/")) return "home";
  if (pathname === "/use-cases" || pathname.startsWith("/use-cases/")) return "use-cases";
  if (pathname === "/docs" || pathname.startsWith("/docs/")) return "docs";
  return undefined;
}
