package common

import (
	"reflect"
	"testing"
)

func TestNormalizePasswordRegisterCodes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "json array",
			raw:  `["alpha"," beta ","alpha",""]`,
			want: []string{"alpha", "beta"},
		},
		{
			name: "comma and newline",
			raw:  "alpha,\n beta \r\ngamma",
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "empty",
			raw:  " \n ",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePasswordRegisterCodes(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected codes: got %v want %v", got, tt.want)
			}
		})
	}
}

func TestIsPasswordRegisterCodeValid(t *testing.T) {
	UpdatePasswordRegisterCodes(`["first","second"]`)
	t.Cleanup(func() {
		UpdatePasswordRegisterCodes("")
	})

	if !IsPasswordRegisterCodeValid(" second ") {
		t.Fatal("expected trimmed code to be accepted")
	}
	if IsPasswordRegisterCodeValid("") {
		t.Fatal("expected empty code to be rejected")
	}
	if IsPasswordRegisterCodeValid("missing") {
		t.Fatal("expected unknown code to be rejected")
	}
}
