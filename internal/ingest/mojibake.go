package ingest

import "strings"

// Transcripts routinely carry text that was decoded as Latin-1 somewhere
// upstream: a file written with a byte-order mark, a tool that guessed an
// encoding, a shell that did not. The result is the familiar mis-decoded em
// dash, and it reaches ritual through no fault of its own — the bytes really
// are in the transcript.
//
// Repair is deliberately narrow. Only the handful of sequences that are
// unambiguous mis-decodings of common punctuation are rewritten, and only when
// a marker rune is present at all, so ordinary text never enters the
// replacement path. Text genuinely containing these sequences does not exist in
// practice; text containing a mis-decoded em dash is in half the skill files on
// disk.

// byteOrderMark is U+FEFF, written as an escape because a literal one in the
// source is itself a byte-order mark and Go rejects it.
const byteOrderMark = "\ufeff"

var mojibake = strings.NewReplacer(
	mis("—"), "—", // em dash
	mis("–"), "–", // en dash
	mis("‘"), "‘", // left single quote
	mis("’"), "’", // right single quote
	mis("“"), "“", // left double quote
	mis("”"), "”", // right double quote
	mis("…"), "…", // ellipsis
	mis("•"), "•", // bullet
	mis(" "), " ", // non-breaking space
	mis("·"), "·",
	mis("«"), "«",
	mis("»"), "»",
	byteOrderMark, "", // byte-order mark
)

// cp1252High maps the bytes 0x80-0x9F to the characters Windows-1252 assigns
// them. Those sixteen bytes are the whole reason this repair is needed: a
// decoder that treats UTF-8 as Windows-1252 turns the second byte of every
// punctuation mark into a curly quote or a euro sign, which is why the
// corruption is so recognizable.
var cp1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}

// mis returns what a string looks like after its UTF-8 bytes have been decoded
// as Windows-1252 and re-encoded as UTF-8 — the exact corruption this file
// repairs.
//
// Deriving the broken forms rather than writing them out keeps the table
// readable and keeps unprintable bytes out of the source.
func mis(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c >= 0x80 && c <= 0x9F {
			b.WriteRune(cp1252High[c-0x80])
			continue
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

// repairMojibake fixes the common Latin-1 mis-decodings and strips a stray
// byte-order mark. Text without a marker rune is returned untouched.
func repairMojibake(s string) string {
	if !strings.ContainsRune(s, 'â') && !strings.ContainsRune(s, 'Â') && !strings.Contains(s, byteOrderMark) {
		return s
	}
	return mojibake.Replace(s)
}
