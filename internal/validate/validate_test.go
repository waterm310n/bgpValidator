package validate

import "testing"

func TestValidate(t *testing.T) {
	r := NewRoutinatorValidator("http", "172.19.220.163:8323", "validity")
	testcases := []struct {
		inOriginAsn string
		inPrefix    string
		wantRes     bool
	}{
		{"50131", "45.43.29.0/24", true},
		{"8393", "91.203.20.0/24", true},
		{"12345", "91.203.20.0/24", false},
	}
	for _, tc := range testcases {
		res := r.Validate(tc.inOriginAsn, tc.inPrefix)
		if res != tc.wantRes {
			t.Errorf("originAsn %s, prefix: %s,result: %v, want %v", tc.inOriginAsn, tc.inPrefix, res, tc.wantRes)
		}
	}
}
