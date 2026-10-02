package telnet

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestIACFilter(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "look\r\n", "look\r\n"},
		{"option negotiation", "\xff\xfb\x1flook\r\n", "look\r\n"},
		{"subnegotiation", "\xff\xfa\x18\x00xterm\xff\xf0say hi\r\n", "say hi\r\n"},
		{"escaped iac", "a\xff\xffb", "a\xffb"},
		{"bare command", "\xff\xf1ok", "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// One byte at a time, so sequences split across reads are covered.
			r := &iacFilter{r: iotest.OneByteReader(strings.NewReader(tt.in))}

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
