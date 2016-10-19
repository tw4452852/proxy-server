package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestGetConfig(t *testing.T) {
	for name, c := range map[string]struct {
		input     string
		shouldErr bool
		expect    *config
	}{
		"blank": {
			input:     "",
			shouldErr: false,
			expect:    &config{},
		},
		"valid": {
			input: `
			[web]
			url = "https://123"
			[foo]
			bar = 1
			`,
			shouldErr: false,
			expect:    &config{Web: webConfig{Url: "https://123"}},
		},
		"inValid": {
			input: `
			[web]
			url = 123
			`,
			shouldErr: true,
			expect:    nil,
		},
	} {
		c := c
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := getConfig(strings.NewReader(c.input))
			if c.shouldErr && err == nil {
				t.Errorf("not get expected error")
			}
			if !c.shouldErr && err != nil {
				t.Errorf("get the unexpected error: %s", err)
			}
			if !reflect.DeepEqual(got, c.expect) {
				t.Errorf("expect: %v, but got: %v", c.expect, got)
			}
		})
	}
}
