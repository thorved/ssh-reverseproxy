"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { AppShell } from "@/components/layout/app-shell";
import { useAuth } from "@/contexts/auth-context";

export function ProtectedLayout({
  children,
  requireAdmin = false,
}: {
  children: React.ReactNode;
  requireAdmin?: boolean;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const { isAuthenticated, isLoading, user } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [isAuthenticated, isLoading, pathname, router]);

  useEffect(() => {
    if (!isLoading && requireAdmin && user?.role !== "admin") {
      router.replace("/dashboard");
    }
  }, [isLoading, requireAdmin, router, user]);

  if (isLoading) {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <div className="rounded-3xl border border-border/80 bg-card/90 px-6 py-4 text-sm text-muted-foreground shadow-lg">
          Loading workspace...
        </div>
      </main>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  if (requireAdmin && user?.role !== "admin") {
    return null;
  }

  return <AppShell>{children}</AppShell>;
}
