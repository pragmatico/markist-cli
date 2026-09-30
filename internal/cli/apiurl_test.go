package cli

import "testing"

func TestResolveAPIURLPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		flagValue   string
		env         string
		configValue string
		want        string
	}{
		{
			name:        "flag wins over everything",
			flagValue:   "https://flag.test",
			env:         "https://env.test",
			configValue: "https://config.test",
			want:        "https://flag.test",
		},
		{
			name:        "env wins over config when flag unset",
			flagValue:   "",
			env:         "https://env.test",
			configValue: "https://config.test",
			want:        "https://env.test",
		},
		{
			name:        "config wins over default when flag and env unset",
			flagValue:   "",
			env:         "",
			configValue: "https://config.test",
			want:        "https://config.test",
		},
		{
			name:        "default when nothing else is set",
			flagValue:   "",
			env:         "",
			configValue: "",
			want:        DefaultAPIURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvAPIURL, tc.env)
			got := resolveAPIURL(tc.flagValue, tc.configValue)
			if got != tc.want {
				t.Fatalf("resolveAPIURL(%q, %q) = %q, want %q", tc.flagValue, tc.configValue, got, tc.want)
			}
		})
	}
}
