package accesslog

import "testing"

func TestRedactSecrets(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"login", `{"password":"hunter2"}`, `{"password":"[redacted]"}`},
		{"connection form", `{"params":{"dbType":"mysql","host":"db","port":3306,"username":"root","password":"pw"}}`,
			`{"params":{"dbType":"mysql","host":"db","password":"[redacted]","port":3306,"username":"root"}}`},
		{"saved connection list", `{"code":200,"data":[{"name":"a","password":"x"},{"name":"b","dbPassword":"y"}]}`,
			`{"code":200,"data":[{"name":"a","password":"[redacted]"},{"dbPassword":"[redacted]","name":"b"}]}`},
		{"no secrets kept byte-for-byte", `{"b": 1, "a": 2.50}`, `{"b": 1, "a": 2.50}`},
		{"empty password kept", `{"password":""}`, `{"password":""}`},
		{"not JSON", `plain text password=x`, `plain text password=x`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(redactSecrets([]byte(tc.in))); got != tc.want {
				t.Fatalf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}
