"use client";

import {
  ArrowRightLeft,
  Home,
  KeyRound,
  LogOut,
  MonitorCog,
  Shield,
  Users,
} from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/auth-context";
import { cn } from "@/lib/utils";

const baseItems = [
  { href: "/dashboard", label: "Dashboard", icon: Home },
  { href: "/instances", label: "Instances", icon: MonitorCog },
  { href: "/settings/ssh-keys", label: "Keys", icon: KeyRound },
];

const adminItems = [
  { href: "/admin/users", label: "Users", icon: Users },
  { href: "/admin/instances", label: "Admin", icon: Shield },
];

function initials(value: string) {
  return value
    .split(/[\s@._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();

  const items =
    user?.role === "admin" ? [...baseItems, ...adminItems] : baseItems;
  const avatarLabel = initials(user?.display_name || user?.email || "U");

  return (
    <div className="min-h-screen">
      <header className="border-b border-border bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/75">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-8">
            <Link href="/dashboard" className="flex items-center gap-3">
              <ArrowRightLeft className="h-5 w-5" />
              <span className="text-2xl font-semibold tracking-tight">
                SSH Reverse Proxy
              </span>
            </Link>

            <nav className="hidden items-center gap-2 lg:flex">
              {items.map((item) => {
                const Icon = item.icon;
                const active = pathname === item.href;

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors",
                      active && "bg-secondary text-foreground",
                      !active && "hover:bg-secondary hover:text-foreground",
                    )}
                  >
                    <Icon className="h-4 w-4" />
                    {item.label}
                  </Link>
                );
              })}
            </nav>
          </div>

          <div className="flex items-center gap-3">
            <ThemeToggle />
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary text-sm font-medium">
              {avatarLabel || "U"}
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="hidden md:inline-flex"
              onClick={async () => {
                await logout();
                router.push("/login");
              }}
            >
              <LogOut className="h-4 w-4" />
              Sign out
            </Button>
          </div>
        </div>

        <div className="mx-auto flex max-w-7xl gap-2 overflow-x-auto px-4 pb-3 lg:hidden sm:px-6 lg:px-8">
          {items.map((item) => {
            const Icon = item.icon;
            const active = pathname === item.href;

            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors",
                  active && "bg-secondary text-foreground",
                  !active && "hover:bg-secondary hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {children}
      </main>
    </div>
  );
}
