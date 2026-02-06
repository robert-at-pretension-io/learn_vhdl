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
