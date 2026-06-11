package expand

import "testing"

func TestNormalizeSpaces(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"git  status *", "git status *"},
		{"git   log   *", "git log *"},
		{"  git status  ", "git status"},
		{"git status *", "git status *"},
		{"", ""},
		// spaces inside double-quoted args preserved
		{`git commit -m "my  message"`, `git commit -m "my  message"`},
		// spaces inside single-quoted args preserved
		{"git commit -m 'my  message'", "git commit -m 'my  message'"},
		// normalize outside, preserve inside double quotes
		{`git  commit  -m  "my msg"`, `git commit -m "my msg"`},
		// normalize outside, preserve inside single quotes
		{"git  commit  -m  'my msg'", "git commit -m 'my msg'"},
		// escaped quote inside double-quoted string does not end the string
		{`git commit -m "it's  fine"`, `git commit -m "it's  fine"`},
		{`git  commit  -m  "say \"hi  there\""`, `git commit -m "say \"hi  there\""`},
	}
	for _, tc := range cases {
		got := NormalizeSpaces(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeSpaces(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}
