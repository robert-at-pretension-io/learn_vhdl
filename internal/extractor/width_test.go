package extractor

import "testing"

func TestCalculateWidth(t *testing.T) {
	tests := []struct {
		typ  string
		want int
	}{
		{"std_logic", 1},
		{"bit", 1},
		{"std_logic_vector(7 downto 0)", 8},
		{"unsigned(0 to 15)", 16},
		{"signed(15 downto 8)", 8},
		{"integer range 0 to 255", 8},
		{"natural range 0 to 7", 3},
		{"std_logic_vector(WIDTH-1 downto 0)", -1}, // unknown but likely multi-bit
		{"unsigned(data'length-1 downto 0)", -1},    // attribute-based range
		{"signed", -1},                               // unconstrained vector
		{"unsigned", -1},                             // unconstrained vector
		{"", 0},
		{"my_custom_record", 0},                      // unknown custom type
	}

	for _, tt := range tests {
		if got := CalculateWidth(tt.typ); got != tt.want {
			t.Fatalf("CalculateWidth(%q) = %d, want %d", tt.typ, got, tt.want)
		}
	}
}
