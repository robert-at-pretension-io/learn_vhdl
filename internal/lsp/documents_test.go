package lsp

import "testing"

func TestDocumentStoreBasic(t *testing.T) {
	ds := NewDocumentStore()

	ds.Set("file:///test.vhd", "hello world")
	content, ok := ds.Get("file:///test.vhd")
	if !ok {
		t.Fatal("expected document to exist")
	}
	if content != "hello world" {
		t.Fatalf("unexpected content: %q", content)
	}

	ds.Delete("file:///test.vhd")
	_, ok = ds.Get("file:///test.vhd")
	if ok {
		t.Fatal("expected document to be deleted")
	}
}

func TestWordAtPosition(t *testing.T) {
	ds := NewDocumentStore()
	ds.Set("file:///test.vhd", `library ieee;
use ieee.std_logic_1164.all;

entity uart_tx is
  port(
    clk : in std_logic;
    data_out : out std_logic_vector(7 downto 0)
  );
end entity uart_tx;`)

	tests := []struct {
		line, char int
		expected   string
	}{
		{0, 0, "library"},
		{0, 3, "library"},
		{0, 8, "ieee"},
		{3, 7, "uart_tx"},
		{3, 10, "uart_tx"},
		{5, 4, "clk"},
		{5, 12, "in"},
		{5, 15, "std_logic"},
		// Out of bounds
		{100, 0, ""},
		{0, 100, ""},
	}

	for _, tt := range tests {
		got := ds.WordAtPosition("file:///test.vhd", tt.line, tt.char)
		if got != tt.expected {
			t.Errorf("WordAtPosition(line=%d, char=%d) = %q, want %q", tt.line, tt.char, got, tt.expected)
		}
	}
}

func TestWordAtPositionNonexistentDoc(t *testing.T) {
	ds := NewDocumentStore()
	word := ds.WordAtPosition("file:///missing.vhd", 0, 0)
	if word != "" {
		t.Errorf("expected empty word for missing doc, got %q", word)
	}
}

func TestWordAtPositionUnicodeIdentifier(t *testing.T) {
	ds := NewDocumentStore()
	ds.Set("file:///unicode.vhd", "signal σclk : integer;")

	// UTF-16 position 8 is on "c" in "σclk".
	word := ds.WordAtPosition("file:///unicode.vhd", 0, 8)
	if word != "σclk" {
		t.Fatalf("expected unicode identifier, got %q", word)
	}
}

func TestUTF16ColumnToRuneIndex(t *testing.T) {
	runes := []rune("a😀b")

	tests := []struct {
		column   int
		want     int
		wantOkay bool
	}{
		{0, 0, true},
		{1, 1, true}, // start of 😀
		{2, 1, true}, // inside surrogate pair snaps to 😀
		{3, 2, true}, // b
		{4, 3, true}, // end of line
		{5, 0, false},
	}

	for _, tt := range tests {
		got, ok := utf16ColumnToRuneIndex(runes, tt.column)
		if got != tt.want || ok != tt.wantOkay {
			t.Fatalf("utf16ColumnToRuneIndex(%d) = (%d,%v), want (%d,%v)",
				tt.column, got, ok, tt.want, tt.wantOkay)
		}
	}
}

func TestWordRangeAndPrefixAtPosition(t *testing.T) {
	ds := NewDocumentStore()
	uri := "file:///test.vhd"
	ds.Set(uri, "signal alpha_beta : std_logic;")

	word, start, end, ok := ds.WordRangeAtPosition(uri, 0, 10)
	if !ok {
		t.Fatal("expected word range")
	}
	if word != "alpha_beta" {
		t.Fatalf("unexpected word %q", word)
	}
	if start >= end {
		t.Fatalf("expected valid range, got start=%d end=%d", start, end)
	}

	prefix := ds.PrefixAtPosition(uri, 0, 10)
	if prefix != "alp" {
		t.Fatalf("expected prefix 'alp', got %q", prefix)
	}
}
