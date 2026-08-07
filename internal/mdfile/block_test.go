package mdfile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/testutil"
)

func TestReplaceBlock_FreshFile(t *testing.T) {
	got, err := ReplaceBlock(nil, []byte("This directory holds PARA **projects**: work with a finish line…\n"))
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	want := BeginMarker + "\n" +
		"This directory holds PARA **projects**: work with a finish line…\n" +
		EndMarker + "\n"
	if string(got) != want {
		t.Errorf("ReplaceBlock() = %q, want %q", got, want)
	}
}

func TestReplaceBlock_PreservesSurroundingProse(t *testing.T) {
	data := []byte(BeginMarker + "\n" +
		"old generated text\n" +
		EndMarker + "\n" +
		"\n" +
		"In this repo every project links its Jira epic in the README frontmatter.\n")

	got, err := ReplaceBlock(data, []byte("new generated text\n"))
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	want := BeginMarker + "\n" +
		"new generated text\n" +
		EndMarker + "\n" +
		"\n" +
		"In this repo every project links its Jira epic in the README frontmatter.\n"
	if string(got) != want {
		t.Errorf("ReplaceBlock() = %q, want %q", got, want)
	}
}

func TestReplaceBlock_PreservesPrefix(t *testing.T) {
	data := []byte("# Projects\n\n" +
		BeginMarker + "\n" +
		"old\n" +
		EndMarker + "\n")

	got, err := ReplaceBlock(data, []byte("new\n"))
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	want := "# Projects\n\n" +
		BeginMarker + "\n" +
		"new\n" +
		EndMarker + "\n"
	if string(got) != want {
		t.Errorf("ReplaceBlock() = %q, want %q", got, want)
	}
}

func TestReplaceBlock_MissingMarkers(t *testing.T) {
	data := []byte("some prose with no para block at all\n")
	if _, err := ReplaceBlock(data, []byte("x\n")); err == nil {
		t.Error("ReplaceBlock on a file with no markers: want error, got nil")
	}
}

func TestReplaceBlock_Deterministic(t *testing.T) {
	data := []byte(BeginMarker + "\nold\n" + EndMarker + "\n")
	block := []byte("new\n")

	a, err := ReplaceBlock(data, block)
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	b, err := ReplaceBlock(data, block)
	if err != nil {
		t.Fatalf("ReplaceBlock: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two ReplaceBlock calls produced different bytes:\n%q\n%q", a, b)
	}
}

// TestReplaceBlock_ArbitraryProseSurvivesRewrites is Phase 3's headline
// property test (implementation-plan.md, Phase 3 "Done when"): arbitrary
// human prose outside the markers must survive an arbitrary number of
// rewrites, since §6 promises para "never touches a byte outside them."
func TestReplaceBlock_ArbitraryProseSurvivesRewrites(t *testing.T) {
	prefixes := []string{
		"",
		"# A title\n\n",
		"Some notes.\n\nMore notes with — em dashes — and 日本語.\n\n",
	}
	suffixes := []string{
		"",
		"\nHuman-authored rules follow.\n",
		"\n\nMultiple\n\nparagraphs\n\nof prose.\n",
	}
	generations := [][]byte{
		[]byte("first generated body\n"),
		[]byte("second, different generated body\n"),
		[]byte("third generated body, longer than the others were\n"),
	}

	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			data := []byte(prefix + BeginMarker + "\ninitial\n" + EndMarker + "\n" + suffix)

			for _, gen := range generations {
				out, err := ReplaceBlock(data, gen)
				if err != nil {
					t.Fatalf("ReplaceBlock: %v", err)
				}
				if !bytes.HasPrefix(out, []byte(prefix)) {
					t.Fatalf("prefix %q lost after rewrite; got %q", prefix, out)
				}
				if !bytes.HasSuffix(out, []byte(suffix)) {
					t.Fatalf("suffix %q lost after rewrite; got %q", suffix, out)
				}
				data = out
			}
		}
	}
}

// FuzzReplaceBlock_PreservesSurroundings is Phase 3's headline property
// test, fuzzed: arbitrary human prose outside the markers must survive a
// rewrite, since §6 promises para "never touches a byte outside them."
// Inputs where prefix/suffix already contain a marker are skipped — that's
// a file with a second, nested block, a shape ReplaceBlock isn't asked to
// handle: it always targets the first begin/end pair it finds.
func FuzzReplaceBlock_PreservesSurroundings(f *testing.F) {
	seeds := [][3]string{
		{"", "", "generated\n"},
		{"# A title\n\n", "\nHuman rules follow.\n", "new body\n"},
		{"", "", ""},
		{"prefix, no trailing newline", "no leading newline either", "x"},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1], s[2])
	}
	f.Fuzz(func(t *testing.T, prefix, suffix, gen string) {
		if strings.Contains(prefix, BeginMarker) || strings.Contains(prefix, EndMarker) ||
			strings.Contains(suffix, BeginMarker) || strings.Contains(suffix, EndMarker) {
			t.Skip()
		}
		data := []byte(prefix + BeginMarker + "\ninitial\n" + EndMarker + "\n" + suffix)

		out, err := ReplaceBlock(data, []byte(gen))
		if err != nil {
			t.Fatalf("ReplaceBlock: %v", err)
		}
		if !bytes.HasPrefix(out, []byte(prefix)) {
			t.Fatalf("prefix not preserved: prefix=%q out=%q", prefix, out)
		}
		if !bytes.HasSuffix(out, []byte(suffix)) {
			t.Fatalf("suffix not preserved: suffix=%q out=%q", suffix, out)
		}
	})
}

// TestRoundTrip_Block proves parse → render is a fixed point for the block
// codec too: extracting the current block's content and rewriting with
// that same content reproduces the source bytes exactly.
func TestRoundTrip_Block(t *testing.T) {
	data := []byte("# Projects\n\n" +
		BeginMarker + "\n" +
		"This directory holds PARA **projects**.\n" +
		EndMarker + "\n" +
		"\n" +
		"Human house rules.\n")

	decode := func(b []byte) ([]byte, error) { return ExtractBlock(b) }
	encode := func(block []byte) ([]byte, error) { return ReplaceBlock(data, block) }

	testutil.AssertRoundTrip(t, data, decode, encode)
}

func TestReplaceDelimited_HashMarkersPreserveHumanLines(t *testing.T) {
	// .gitattributes has no Markdown comment syntax, so the same block
	// mechanism has to work with `#` markers (§2.2's "append-only to an
	// existing file", §9).
	data := []byte("*.png binary\n" +
		HashMarkers.Begin + "\n" +
		"old generated lines\n" +
		HashMarkers.End + "\n" +
		"*.pdf binary\n")

	got, err := ReplaceDelimited(HashMarkers, data, []byte("**/logs/*.jsonl merge=union\n"))
	if err != nil {
		t.Fatalf("ReplaceDelimited: %v", err)
	}
	want := "*.png binary\n" +
		HashMarkers.Begin + "\n" +
		"**/logs/*.jsonl merge=union\n" +
		HashMarkers.End + "\n" +
		"*.pdf binary\n"
	if string(got) != want {
		t.Errorf("ReplaceDelimited() =\n%q\nwant\n%q", got, want)
	}
}

func TestExtractDelimited_HashMarkers(t *testing.T) {
	data, err := ReplaceDelimited(HashMarkers, nil, []byte("a\nb\n"))
	if err != nil {
		t.Fatalf("ReplaceDelimited: %v", err)
	}
	got, err := ExtractDelimited(HashMarkers, data)
	if err != nil {
		t.Fatalf("ExtractDelimited: %v", err)
	}
	if string(got) != "a\nb\n" {
		t.Errorf("ExtractDelimited() = %q, want %q", got, "a\nb\n")
	}
}

func TestRemoveDelimited(t *testing.T) {
	const begin = "# para:begin — generated, do not edit; run `para rebuild`"
	const end = "# para:end"

	tests := []struct {
		name  string
		data  string
		want  string
		found bool
	}{
		{
			// The whole file is para's block — a .gitattributes `init` wrote
			// into a repository that had none. What is left is what was there
			// before para: nothing.
			name:  "block is the whole file",
			data:  begin + "\nx merge=ours\n" + end + "\n",
			want:  "",
			found: true,
		},
		{
			name:  "keeps the repository's own lines",
			data:  "*.png binary\n" + begin + "\nx merge=ours\n" + end + "\n*.md text\n",
			want:  "*.png binary\n*.md text\n",
			found: true,
		},
		{
			name:  "no block is nothing to remove",
			data:  "*.png binary\n",
			want:  "*.png binary\n",
			found: false,
		},
		{
			name:  "empty file",
			data:  "",
			want:  "",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := RemoveDelimited(HashMarkers, []byte(tt.data))
			if err != nil {
				t.Fatalf("RemoveDelimited: %v", err)
			}
			if found != tt.found {
				t.Errorf("RemoveDelimited() found = %v, want %v", found, tt.found)
			}
			if string(got) != tt.want {
				t.Errorf("RemoveDelimited() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoveDelimited_IsReplaceDelimitedReversed(t *testing.T) {
	// The two together are a round trip: rewriting a file's block and then
	// taking the block out returns every line that was not para's. That is the
	// property `emit.gitattributes = false` rests on — §9's block is removable
	// precisely because it is delimited.
	block := HashMarkers.Begin + "\nold\n" + HashMarkers.End + "\n"
	for _, own := range []struct{ prefix, suffix string }{
		{"", ""},
		{"*.png binary\n", ""},
		{"", "*.md text\n"},
		{"# a comment\n\n", "\n*.md text\n"},
	} {
		original := own.prefix + own.suffix
		with, err := ReplaceDelimited(HashMarkers, []byte(own.prefix+block+own.suffix), []byte("x merge=ours\n"))
		if err != nil {
			t.Fatalf("ReplaceDelimited(%q): %v", original, err)
		}
		got, found, err := RemoveDelimited(HashMarkers, with)
		if err != nil {
			t.Fatalf("RemoveDelimited(%q): %v", with, err)
		}
		if !found {
			t.Errorf("RemoveDelimited(%q) found no block", with)
		}
		if string(got) != original {
			t.Errorf("round trip over %q = %q, want %q", original, got, original)
		}
	}
}

func TestAppendDelimited(t *testing.T) {
	begin, end := HashMarkers.Begin, HashMarkers.End

	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "no file yet",
			data: "",
			want: begin + "\nx merge=ours\n" + end + "\n",
		},
		{
			// §2.2's "append-only to an existing file": a repository that
			// already has a .gitattributes keeps every line and gains the block.
			name: "a file para has never touched",
			data: "*.png binary\n",
			want: "*.png binary\n" + begin + "\nx merge=ours\n" + end + "\n",
		},
		{
			name: "a file with no final newline",
			data: "*.png binary",
			want: "*.png binary\n" + begin + "\nx merge=ours\n" + end + "\n",
		},
		{
			name: "a block already there is replaced, not doubled",
			data: "*.png binary\n" + begin + "\nold\n" + end + "\n*.md text\n",
			want: "*.png binary\n" + begin + "\nx merge=ours\n" + end + "\n*.md text\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppendDelimited(HashMarkers, []byte(tt.data), []byte("x merge=ours\n"))
			if err != nil {
				t.Fatalf("AppendDelimited: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("AppendDelimited() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendDelimitedIsUndoneByRemoveDelimited(t *testing.T) {
	// The pair is what `emit.gitattributes` toggles between, so appending the
	// block and taking it out again has to give back the repository's file.
	for _, original := range []string{"", "*.png binary\n", "# a comment\n\n*.png binary\n"} {
		with, err := AppendDelimited(HashMarkers, []byte(original), []byte("x merge=ours\n"))
		if err != nil {
			t.Fatalf("AppendDelimited(%q): %v", original, err)
		}
		got, found, err := RemoveDelimited(HashMarkers, with)
		if err != nil {
			t.Fatalf("RemoveDelimited(%q): %v", with, err)
		}
		if !found {
			t.Errorf("RemoveDelimited(%q) found no block", with)
		}
		if string(got) != original {
			t.Errorf("round trip over %q = %q", original, got)
		}
	}
}

// TestDamagedBlockIsAnErrorInEveryDirection: a begin marker whose end marker was
// edited away is damage, and all three of the block operations have to say so.
//
// They did not, and the disagreement was the bug: AppendDelimited refused such a
// file — so `para init` in a repository holding one failed with no path in the
// message — while RemoveDelimited called it "no block", so `para config set
// emit.gitattributes false` reported success and left every one of para's lines
// in place. One rule, three functions, and the two that disagreed were the two
// that turn a config key on and off.
func TestDamagedBlockIsAnErrorInEveryDirection(t *testing.T) {
	damaged := [][]byte{
		[]byte(HashMarkers.Begin + "\nx merge=ours\n"),
		[]byte("*.png binary\n" + HashMarkers.Begin + "\nx merge=ours\n"),
		[]byte(HashMarkers.Begin),
		[]byte(HashMarkers.Begin + " trailing text on the marker's line\nx\n" + HashMarkers.End + "\n"),
	}
	for _, data := range damaged {
		if _, err := ReplaceDelimited(HashMarkers, data, []byte("y\n")); err == nil {
			t.Errorf("ReplaceDelimited(%q) accepted a damaged block", data)
		}
		if _, err := AppendDelimited(HashMarkers, data, []byte("y\n")); err == nil {
			t.Errorf("AppendDelimited(%q) accepted a damaged block", data)
		}
		if _, found, err := RemoveDelimited(HashMarkers, data); err == nil {
			t.Errorf("RemoveDelimited(%q) reported found=%v and no error over a damaged block", data, found)
		}
	}
}
