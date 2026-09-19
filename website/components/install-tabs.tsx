"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CopyCommand } from "@/components/copy-command";

const options = [
  {
    id: "homebrew",
    label: "Homebrew",
    command: "brew install HarjjotSinghh/tap/ritual",
    note: "macOS and Linux.",
  },
  {
    id: "go",
    label: "Go",
    command: "go install github.com/HarjjotSinghh/ritual/cmd/ritual@latest",
    note: "Go 1.25 or newer. Builds from source.",
  },
  {
    id: "scoop",
    label: "Scoop",
    command: "scoop install ritual",
    note: "Add the bucket first: scoop bucket add harjjotsinghh https://github.com/HarjjotSinghh/scoop-bucket",
  },
  {
    id: "binary",
    label: "Binary",
    command: "curl -fsSL https://github.com/HarjjotSinghh/ritual/releases/latest",
    note: "Signed archives for macOS, Linux, and Windows on amd64 and arm64.",
  },
];

export function InstallTabs() {
  return (
    <Tabs defaultValue="homebrew" className="w-full">
      <TabsList className="ui">
        {options.map((option) => (
          <TabsTrigger key={option.id} value={option.id}>
            {option.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {options.map((option) => (
        <TabsContent key={option.id} value={option.id} className="mt-4 space-y-3">
          <CopyCommand command={option.command} />
          <p className="text-sm text-muted-foreground">{option.note}</p>
        </TabsContent>
      ))}
    </Tabs>
  );
}
