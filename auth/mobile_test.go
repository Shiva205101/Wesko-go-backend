package auth

import "testing"

func TestNormalizeIndianMobile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "local 10 digit", input: "9642560235", want: "+919642560235"},
		{name: "leading zero", input: "09642560235", want: "+919642560235"},
		{name: "country code digits", input: "919642560235", want: "+919642560235"},
		{name: "formatted input", input: "+91 96425-60235", want: "+919642560235"},
		{name: "invalid prefix", input: "5642560235", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeIndianMobile(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
