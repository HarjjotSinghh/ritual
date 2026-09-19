"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";

// The toggle renders its icon only after mount. Before that the server and the
// client disagree about which theme is active, and a flipped icon on first
// paint is more distracting than no icon at all.
export function ThemeToggle() {
  const [dark, setDark] = useState<boolean | null>(null);

  useEffect(() => {
    setDark(document.documentElement.classList.contains("dark"));
  }, []);

  function toggle() {
    const next = !document.documentElement.classList.contains("dark");
    document.documentElement.classList.toggle("dark", next);
    try {
      localStorage.setItem("ritual-theme", next ? "dark" : "light");
    } catch {
      // A browser with storage blocked still gets the toggle, just not the
      // memory of it.
    }
    setDark(next);
  }

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      onClick={toggle}
      aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
      className="text-muted-foreground hover:text-foreground"
    >
      {dark === null ? (
        <span className="size-4" />
      ) : dark ? (
        <Sun aria-hidden />
      ) : (
        <Moon aria-hidden />
      )}
    </Button>
  );
}
