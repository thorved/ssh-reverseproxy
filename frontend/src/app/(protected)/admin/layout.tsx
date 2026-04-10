"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useAuth } from "@/contexts/auth-context";

export default function AdminLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const router = useRouter();
  const { isLoading, user } = useAuth();

  useEffect(() => {
    if (!isLoading && user?.role !== "admin") {
      router.replace("/dashboard");
    }
  }, [isLoading, router, user]);

  if (isLoading) {
    return null;
  }

  if (user?.role !== "admin") {
    return null;
  }

  return <>{children}</>;
}
