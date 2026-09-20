// Extracts the brand marks the site needs into a single checked-in module, so
// the page ships a handful of inline paths instead of depending on two icon
// packages at runtime. Re-run after `npm i --no-save simple-icons
// @lobehub/icons-static-svg` to refresh.
import fs from "node:fs";
import path from "node:path";
import * as simpleIcons from "simple-icons";

const ROOT = "/Users/harjjotsinghh/Documents/Projects/ritual/website";
const LOBE = path.join(ROOT, "node_modules/@lobehub/icons-static-svg/icons");
const OUT = path.join(ROOT, "lib/brand-marks.ts");

// key -> lobehub filename. These are the agent marks.
const fromLobe = {
  claude: "claudecode.svg",
  codex: "codex.svg",
  cursor: "cursor.svg",
  opencode: "opencode.svg",
  gemini: "geminicli.svg",
  grok: "grok.svg",
  qwen: "qwen.svg",
  kimi: "kimi.svg",
  copilot: "copilot.svg",
  cline: "cline.svg",
  pi: "pi.svg",
};

// key -> simple-icons slug. These are the distribution channels.
const fromSimple = {
  go: "go",
  homebrew: "homebrew",
  github: "github",
};

function lobe(file) {
  const raw = fs.readFileSync(path.join(LOBE, file), "utf8");
  const viewBox = raw.match(/viewBox="([^"]+)"/)[1];
  const fillRule = /fill-rule="evenodd"/.test(raw);
  let body = raw.replace(/^[\s\S]*?<\/title>/, "").replace(/<\/svg>\s*$/, "");
  body = body.replace(/\s(fill|height|width|style|xmlns)="[^"]*"/g, "");
  return { viewBox, fillRule, body: body.trim() };
}

function simple(slug) {
  const icon = Object.values(simpleIcons).find((i) => i && i.slug === slug);
  if (!icon) throw new Error("missing simple-icons slug: " + slug);
  return { viewBox: "0 0 24 24", fillRule: false, body: `<path d="${icon.path}"></path>` };
}

const marks = {};
for (const [key, file] of Object.entries(fromLobe)) marks[key] = lobe(file);
for (const [key, slug] of Object.entries(fromSimple)) marks[key] = simple(slug);

const header = `// Brand marks, inlined.
//
// Agent marks come from @lobehub/icons-static-svg (MIT), which is the only set
// that covers all eleven CLIs as one consistent monochrome family. The
// distribution marks come from simple-icons (CC0). Both are extracted at
// author time by scripts/gen-brand-marks.mjs so the bundle carries a dozen
// paths rather than two icon packages.
//
// The marks are trademarks of their owners and appear here only to identify
// the tools ritual reads from and the channels it ships through.

export type BrandMark = {
  viewBox: string;
  /** Some marks are drawn as cut-outs and need evenodd to render the holes. */
  fillRule: boolean;
  body: string;
};

export type BrandKey = keyof typeof brandMarks;

export const brandMarks = `;

const entries = Object.entries(marks)
  .map(
    ([key, m]) =>
      `  ${key}: {\n    viewBox: ${JSON.stringify(m.viewBox)},\n    fillRule: ${m.fillRule},\n    body: ${JSON.stringify(m.body)},\n  },`,
  )
  .join("\n");

fs.writeFileSync(OUT, `${header}{\n${entries}\n} satisfies Record<string, BrandMark>;\n`);

console.log("wrote", OUT);
for (const [k, v] of Object.entries(marks)) {
  console.log(`  ${k.padEnd(10)} ${v.viewBox.padEnd(12)} evenodd=${String(v.fillRule).padEnd(5)} ${v.body.length} chars`);
}
