package template

import "testing"

func TestEscapePlanField(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"a|b", "a%7Cb"},
		{"50%", "50%25"},
		{"line1\nline2", "line1%0Aline2"},
		// % must be escaped first; raw "%7C" must become "%257C".
		{"raw %7C", "raw %257C"},
		// Combined.
		{"a|b\nc%d", "a%7Cb%0Ac%25d"},
	}
	for _, tc := range cases {
		got := EscapePlanField(tc.in)
		if got != tc.want {
			t.Errorf("EscapePlanField(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
