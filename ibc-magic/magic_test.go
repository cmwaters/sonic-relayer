package magic_test

import (
	"fmt"
	"testing"
)

func TestIBCMagic(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success",
			func() {
			},
			true,
		},
	}

	for _, tc := range testCases {
		fmt.Println(tc.name)
	}
}
