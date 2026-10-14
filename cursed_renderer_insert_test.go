package tea

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestCursedRendererInsertAboveFlushesPendingFrame(t *testing.T) {
	output, renderer := newInsertionTestRenderer(t, 12, 6, "old")
	renderer.render(NewView("new one\nnew two\nnew three"))

	if err := renderer.insertAbove("printed"); err != nil {
		t.Fatal(err)
	}

	if got := renderer.cellbuf.Height(); got != 3 {
		t.Fatalf("frame height = %d, want 3", got)
	}

	text := ansi.Strip(output.lastWrite())
	assertTextInOrder(t, text, []string{"printed", "new one", "new two", "new three"})

	if strings.Contains(text, "old") {
		t.Fatalf("insertion redrew stale frame: %q", text)
	}
}

func TestCursedRendererInsertAboveOversized(t *testing.T) {
	output, renderer := newInsertionTestRenderer(t, 16, 24, "managed one\nmanaged two\nfooter")
	inserted := make([]string, 30)

	for i := range inserted {
		inserted[i] = fmt.Sprintf("inserted-%02d", i)
	}

	if err := renderer.insertAbove(strings.Join(inserted, "\n")); err != nil {
		t.Fatal(err)
	}

	expected := append(inserted, "managed one", "managed two", "footer")
	text := ansi.Strip(output.lastWrite())
	assertTextInOrder(t, text, expected)

	for _, value := range expected {
		if got := strings.Count(text, value); got != 1 {
			t.Fatalf("%q occurs %d times, want 1 in %q", value, got, text)
		}
	}
}

func TestCursedRendererInsertAboveEmptyFrame(t *testing.T) {
	output, renderer := newInsertionTestRenderer(t, 12, 3, "")

	if err := renderer.insertAbove(""); err != nil {
		t.Fatal(err)
	}

	want := "\r\n" + ansi.ResetStyle + ansi.ResetHyperlink()
	if got := output.lastWrite(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCursedRendererInsertAboveSoftWrap(t *testing.T) {
	output, renderer := newInsertionTestRenderer(t, 4, 3, "")
	renderer.setSoftWrapPrint(true)

	if err := renderer.insertAbove("abcdefgh"); err != nil {
		t.Fatal(err)
	}

	want := "abcdefgh\r\n" + ansi.ResetStyle + ansi.ResetHyperlink()
	if got := output.lastWrite(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCursedRendererInsertAboveSynchronizesOutput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		enabled bool
	}{
		{name: "enabled", enabled: true},
		{name: "disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output, renderer := newInsertionTestRenderer(t, 12, 3, "managed")
			renderer.setSyncdUpdates(tc.enabled)

			if err := renderer.insertAbove("printed"); err != nil {
				t.Fatal(err)
			}

			got := output.lastWrite()
			assertTextInOrder(t, ansi.Strip(got), []string{"printed", "managed"})

			if !tc.enabled {
				if strings.Contains(got, ansi.SetModeSynchronizedOutput) || strings.Contains(got, ansi.ResetModeSynchronizedOutput) {
					t.Fatalf("insertion enabled unsupported synchronized output: %q", got)
				}

				return
			}

			if !strings.HasPrefix(got, ansi.SetModeSynchronizedOutput) || !strings.HasSuffix(got, ansi.ResetModeSynchronizedOutput) {
				t.Fatalf("insertion is not wrapped in synchronized output: %q", got)
			}
		})
	}
}

func TestCursedRendererInsertAboveAvoidsUnsafeErases(t *testing.T) {
	output, renderer := newInsertionTestRenderer(t, 4, 3, "wxyz")

	if err := renderer.insertAbove("abcdef"); err != nil {
		t.Fatal(err)
	}

	// Erasing to the line end after a full-width row removes its last cell.
	// The frame starts at the top, so erasing below clears the whole screen,
	// which tmux pushes into scrollback.
	got := output.lastWrite()
	for _, sequence := range []string{ansi.EraseLineRight, ansi.EraseScreenBelow, ansi.EraseEntireScreen} {
		if strings.Contains(got, sequence) {
			t.Fatalf("output erases with %q: %q", sequence, got)
		}
	}

	assertTextInOrder(t, ansi.Strip(got), []string{"abcd", "ef", "wxyz"})
}

func TestCursedRendererInsertAbovePreservesUnicodeWrapProtection(t *testing.T) {
	t.Parallel()

	// Wide cells must retain the renderer's wrap protection rather than being
	// emitted as a padded raw line.
	output, renderer := newInsertionTestRenderer(t, 6, 3, "🐇")
	x, y := renderer.scr.Position()

	if err := renderer.insertAbove("printed"); err != nil {
		t.Fatal(err)
	}

	got := output.lastWrite()
	assertTextInOrder(t, got, []string{ansi.ResetModeAutoWrap, "🐇", ansi.SetModeAutoWrap})

	if strings.Contains(got, "🐇    ") {
		t.Fatalf("redrawn frame pads a width-sensitive row: %q", got)
	}

	if gotX, gotY := renderer.scr.Position(); gotX != x || gotY != y {
		t.Fatalf("cursor = (%d, %d), want (%d, %d)", gotX, gotY, x, y)
	}
}

func TestCursedRendererInsertAbovePreservesViewColors(t *testing.T) {
	output := new(insertionTestWriter)
	renderer := newCursedRenderer(output, []string{"TERM=xterm-256color"}, 12, 3)
	view := NewView(ansi.SetForegroundColor("#abcdef") + "managed" + ansi.ResetStyle)
	view.ForegroundColor = color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}
	view.BackgroundColor = color.RGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff}
	renderer.render(view)

	if err := renderer.flush(false); err != nil {
		t.Fatal(err)
	}

	output.writes = nil

	if err := renderer.insertAbove("printed"); err != nil {
		t.Fatal(err)
	}

	got := output.lastWrite()
	inserted := strings.Index(got, "printed")

	if inserted < 0 {
		t.Fatalf("output does not contain inserted text: %q", got)
	}

	restored := got[inserted+len("printed"):]
	for _, sequence := range []string{
		ansi.SetForegroundColor("#112233"),
		ansi.SetBackgroundColor("#445566"),
	} {
		if !strings.Contains(restored, sequence) {
			t.Fatalf("output does not restore %q after the inserted text: %q", sequence, got)
		}
	}
}

func TestCursedRendererInsertAboveUsesColorProfile(t *testing.T) {
	output := new(insertionTestWriter)
	renderer := newCursedRenderer(output, []string{"TERM=xterm-256color"}, 12, 3)
	renderer.setColorProfile(colorprofile.Ascii)

	foreground := ansi.SetForegroundColor("#123456")
	renderer.render(NewView(foreground + "managed" + ansi.ResetStyle))

	if err := renderer.flush(false); err != nil {
		t.Fatal(err)
	}

	output.writes = nil

	if err := renderer.insertAbove("printed"); err != nil {
		t.Fatal(err)
	}

	if got := output.lastWrite(); strings.Contains(got, foreground) {
		t.Fatalf("redrawn frame bypassed the color profile: %q", got)
	}
}

type insertionTestWriter struct {
	writes []string
}

func (w *insertionTestWriter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func (w *insertionTestWriter) lastWrite() string {
	if len(w.writes) == 0 {
		return ""
	}

	return w.writes[len(w.writes)-1]
}

func newInsertionTestRenderer(t *testing.T, width, height int, content string) (*insertionTestWriter, *cursedRenderer) {
	t.Helper()

	output := new(insertionTestWriter)
	renderer := newCursedRenderer(output, []string{"TERM=xterm-256color"}, width, height)
	renderer.render(NewView(content))

	if err := renderer.flush(false); err != nil {
		t.Fatal(err)
	}

	output.writes = nil

	return output, renderer
}

func assertTextInOrder(t *testing.T, text string, values []string) {
	t.Helper()

	for _, value := range values {
		index := strings.Index(text, value)
		if index < 0 {
			t.Fatalf("output does not contain %q: %q", value, text)
		}

		text = text[index+len(value):]
	}
}
