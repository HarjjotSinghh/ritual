"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CopyCommand } from "@/components/copy-command";

const options = [
  {
    id: "go",
    label: "Go",
    command: "go install github.com/HarjjotSinghh/ritual/cmd/ritual@latest",
    note: "Builds from source. Needs Go 1.26 or newer.",
  },
  {
    id: "binary",
    label: "Release",
    command: "open https://github.com/HarjjotSinghh/ritual/releases/latest",
    note: "Archives for macOS, Linux, and Windows on amd64 and arm64, with checksums.",
  },
  {
    id: "homebrew",
    label: "Homebrew",
    command: "brew install --cask HarjjotSinghh/tap/ritual",
    note: "macOS. Published by the release workflow once the tap token is configured.",
  },
  {
    id: "scoop",
    label: "Scoop",
    command: "scoop install ritual",
    note: "Windows. Add the bucket first: scoop bucket add harjjotsinghh https://github.com/HarjjotSinghh/scoop-bucket",
  },
];

export function InstallTabs() {
  return (
    <Tabs defaultValue="go" className="w-full">
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
