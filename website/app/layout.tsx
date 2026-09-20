import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const sans = Geist({
  variable: "--font-sans-ui",
  subsets: ["latin"],
  display: "swap",
});

const mono = Geist_Mono({
  variable: "--font-mono-ui",
  subsets: ["latin"],
  display: "swap",
});

const description =
  "ritual reads the session history your coding agents already write to disk, finds the work you repeat, and turns it into skills, rules, commands, and hooks. Local-first: nothing is uploaded.";

export const metadata: Metadata = {
  metadataBase: new URL("https://ritual-harjjot.vercel.app"),
  title: {
    default: "ritual - cross-agent process mining for developers",
    template: "%s - ritual",
  },
  description,
  keywords: [
    "agent skills",
    "claude code",
    "codex cli",
    "cursor",
    "opencode",
    "process mining",
    "developer workflows",
    "SKILL.md",
  ],
  authors: [
    { name: "Harjot Singh Rana", url: "https://github.com/HarjjotSinghh" },
  ],
  openGraph: {
    type: "website",
    title: "ritual - cross-agent process mining for developers",
    description,
    siteName: "ritual",
  },
  twitter: {
    card: "summary_large_image",
    title: "ritual - cross-agent process mining for developers",
    description,
  },
  robots: { index: true, follow: true },
};

// The theme is resolved before first paint. Without this the page flashes
// light, which on a dark-mode terminal tool is the first thing it gets wrong.
const themeScript = `
(function () {
  try {
    var stored = localStorage.getItem("ritual-theme");
    var dark = stored ? stored === "dark"
      : window.matchMedia("(prefers-color-scheme: dark)").matches;
    document.documentElement.classList.toggle("dark", dark);
  } catch (e) {}
})();
`;

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${sans.variable} ${mono.variable} h-full antialiased`}
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="flex min-h-full flex-col">
        {/* The ruled column the whole page hangs off. Fixed, behind everything,
            never interactive. */}
        <div aria-hidden className="shell-rules" />
        <div className="relative z-1 flex min-h-full flex-1 flex-col">
          {children}
        </div>
      </body>
    </html>
  );
}
