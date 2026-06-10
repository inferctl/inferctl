package render

import "testing"

func TestSelectModeExplicitJSONOnly(t *testing.T) {
	if got := SelectMode(Options{StdoutTTY: false}); got != ModeHuman {
		t.Fatalf("non-TTY mode = %s, want human", got)
	}
	if got := SelectMode(Options{JSONFlag: true}); got != ModeJSON {
		t.Fatalf("--json mode = %s", got)
	}
	if got := SelectMode(Options{Env: map[string]string{"INFERCTL_FORMAT": "json"}}); got != ModeJSON {
		t.Fatalf("INFERCTL_FORMAT=json mode = %s", got)
	}
}

func TestANSIAllowed(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		tty  bool
		want bool
	}{
		{name: "tty", env: map[string]string{}, tty: true, want: true},
		{name: "not tty", env: map[string]string{}, tty: false, want: false},
		{name: "no color", env: map[string]string{"NO_COLOR": "1"}, tty: true, want: false},
		{name: "ci", env: map[string]string{"CI": "1"}, tty: true, want: false},
		{name: "dumb", env: map[string]string{"TERM": "dumb"}, tty: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ANSIAllowed(tc.env, tc.tty); got != tc.want {
				t.Fatalf("ANSIAllowed() = %v, want %v", got, tc.want)
			}
		})
	}
}
