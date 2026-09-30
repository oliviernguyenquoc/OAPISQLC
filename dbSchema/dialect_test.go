package dbSchema_test

import (
	"testing"

	"github.com/oliviernguyenquoc/oapisqlc/dbSchema"
)

func TestPostgresType(t *testing.T) {
	t.Parallel()

	dialect := dbSchema.NewDialect()

	tests := []struct {
		name       string
		dataType   string
		dataFormat string
		want       string
		wantCode   string
	}{
		{name: "type and format", dataType: "integer", dataFormat: "int64", want: "BIGINT"},
		{name: "type alone", dataType: "string", want: "TEXT"},
		{
			name:       "unknown format keeps the type",
			dataType:   "string",
			dataFormat: "uri",
			want:       "TEXT",
		},
		{name: "uuid gets the native type", dataType: "string", dataFormat: "uuid", want: "UUID"},
		{name: "unknown type", dataType: "mystery", wantCode: dbSchema.EINVALID},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := dialect.PostgresType(test.dataType, test.dataFormat)

			if code := dbSchema.ErrorCode(err); code != test.wantCode {
				t.Fatalf("error code = %q, want %q (err: %v)", code, test.wantCode, err)
			}

			if got != test.want {
				t.Errorf("PostgresType(%q, %q) = %q, want %q",
					test.dataType, test.dataFormat, got, test.want)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	t.Parallel()

	dialect := dbSchema.NewDialect()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "ordinary name", in: "username", want: "username"},
		{name: "reserved word", in: "primary", want: `"primary"`},
		{name: "reserved word in any case", in: "Select", want: `"Select"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := dialect.QuoteIdentifier(test.in); got != test.want {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
