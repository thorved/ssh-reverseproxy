"use client";

import { LaptopMinimal, MoonStar, SunMedium } from "lucide-react";
import { useTheme } from "@/components/providers/theme-provider";
import { Button } from "@/components/ui/button";

const themeOrder = ["light", "dark", "system"] as const;

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();

  const nextTheme =
    themeOrder[(themeOrder.indexOf(theme) + 1) % themeOrder.length];

  const Icon =
    theme === "dark"
      ? MoonStar
      : theme === "system"
        ? LaptopMinimal
        : SunMedium;

  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      className="h-10 w-10 rounded-full border border-transparent p-0 text-muted-foreground hover:border-border hover:bg-accent"
      onClick={() => setTheme(nextTheme)}
      title={`Theme: ${theme}`}
    >
      <Icon className="h-4 w-4" />
      <span className="sr-only">Toggle theme</span>
    </Button>
  );
}
