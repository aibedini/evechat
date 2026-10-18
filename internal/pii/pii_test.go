package pii

import (
	"strings"
	"testing"
)

func TestMaskPhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"09121234567", "091***67"},
		{" +989121234567 ", "+98***67"},
		{"0912 123 4567", "091***67"},
		{"123456", "***"},
		{"1234567", "123***67"},
		{"", "***"},
		{"   ", "***"},
		{"۰۹۱۲۱۲۳۴۵۶۷", "۰۹۱***۶۷"},
	}
	for _, c := range cases {
		if got := MaskPhone(c.in); got != c.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskPhoneHidesTheMiddle(t *testing.T) {
	const number = "09121234567"
	masked := MaskPhone(number)
	if strings.Contains(masked, "1234567") || strings.Contains(masked, number) {
		t.Fatalf("masked value still contains the full number: %q", masked)
	}
	if len([]rune(masked)) >= len([]rune(number)) {
		t.Fatalf("masked value is not shorter than the original: %q", masked)
	}
}
