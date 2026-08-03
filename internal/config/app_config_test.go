package config

import "testing"

func TestPublicWebOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		want    string
		wantErr bool
	}{
		{name: "normalizes trailing slash", origin: " https://hosthalla.example.com/ ", want: "https://hosthalla.example.com"},
		{name: "rejects path", origin: "https://hosthalla.example.com/app", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := NewDefaultAppConfig()
			cfg.WebOrigin = test.origin
			cfg.ApplyDefaults()

			origin, err := cfg.PublicWebOrigin()
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("PublicWebOrigin returned error: %v", err)
			}
			if origin != test.want {
				t.Fatalf("origin = %q", origin)
			}
		})
	}
}
