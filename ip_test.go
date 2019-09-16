package main

import (
	"bytes"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIncIPAddress(t *testing.T) {
	testCases := []struct{ t, e net.IP }{
		{
			t: net.IP([]byte{10, 10, 10, 0}),
			e: net.IP([]byte{10, 10, 10, 1}),
		},
		{
			t: net.IP([]byte{10, 10, 10, 255}),
			e: net.IP([]byte{10, 10, 11, 0}),
		},
	}
	for _, test := range testCases {
		incIPAddress(test.t)
		if !bytes.Equal(test.t, test.e) {
			t.Errorf("inIPAddress: expected %v, got %v", test.e, test.t)
		}
	}
}

func TestGetAvailableIPAddresses(t *testing.T) {
	testCases := []struct {
		c    string
		t, e []net.IP
	}{
		{
			c: "10.10.10.0/29",
			t: []net.IP{[]byte{10, 10, 10, 1}, []byte{10, 10, 10, 3}},
			e: []net.IP{[]byte{10, 10, 10, 2}, []byte{10, 10, 10, 4}, []byte{10, 10, 10, 5}, []byte{10, 10, 10, 6}},
		},
	}
	for _, test := range testCases {
		a, err := getAvailableIPAddresses(test.c, test.t)
		if err != nil {
			t.Errorf("getAvailableIPAddresses: %v", err)
		}
		if diff := cmp.Diff(test.e, a); diff != "" {
			t.Errorf("getAvailableIPAddresses: did not get expected result:\n%s", diff)
		}
	}
}
