"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { BrandMark } from "@/components/site/brand-mark";
import { CopyCommand } from "@/components/site/copy-command";
import { installOptions } from "@/lib/content";

export function InstallTabs() {
  return (
    <Tabs defaultValue="go" className="w-full">
      <TabsList variant="line" className="mono h-auto p-0 text-[13px]">
        {installOptions.map((option) => (
          <TabsTrigger
            key={option.id}
            value={option.id}
            className="group gap-1.5 px-2 py-1 transition-colors duration-(--dur-fast)"
          >
            {option.brand ? (
              <BrandMark
                name={option.brand}
                className="size-3.5 transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-px"
              />
            ) : option.Icon ? (
              <option.Icon
                aria-hidden
                strokeWidth={1.5}
                className="size-3.5 shrink-0 transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-px"
              />
            ) : null}
            {option.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {installOptions.map((option) => (
        <TabsContent
          key={option.id}
          value={option.id}
          // Base UI reports which side the previous tab was on. The panel
          // slides in from that side, so the motion says where you came from
          // rather than just that something changed.
          className="mt-5 space-y-3 duration-(--dur-slow) data-[activation-direction=left]:animate-in data-[activation-direction=left]:slide-in-from-left-2 data-[activation-direction=left]:fade-in-50 data-[activation-direction=right]:animate-in data-[activation-direction=right]:slide-in-from-right-2 data-[activation-direction=right]:fade-in-50"
        >
          <CopyCommand command={option.command} />
          <p className="text-[13px] text-pretty text-muted-foreground">
            {option.note}
          </p>
        </TabsContent>
      ))}
    </Tabs>
  );
}
