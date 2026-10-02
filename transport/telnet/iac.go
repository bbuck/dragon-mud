package telnet

import "io"

// Telnet protocol bytes.
const (
	iac  = 255 // interpret as command
	sb   = 250 // subnegotiation begin
	se   = 240 // subnegotiation end
	will = 251
	wont = 252
	dont = 254

	optEcho = 1
)

// iacFilter strips telnet negotiation from client input so only typed text
// reaches the game. Negotiation itself (GMCP and friends) comes later.
type iacFilter struct {
	r     io.Reader
	state int
}

const (
	stateData = iota
	stateIAC
	stateOption
	stateSub
	stateSubIAC
)

func (f *iacFilter) Read(p []byte) (int, error) {
	for {
		n, err := f.r.Read(p)
		out := 0

		for _, b := range p[:n] {
			switch f.state {
			case stateData:
				if b == iac {
					f.state = stateIAC
					continue
				}
				p[out] = b
				out++

			case stateIAC:
				switch {
				case b == iac:
					// An escaped 255 is a literal byte.
					p[out] = b
					out++
					f.state = stateData
				case b >= will && b <= dont:
					f.state = stateOption
				case b == sb:
					f.state = stateSub
				default:
					f.state = stateData
				}

			case stateOption:
				f.state = stateData

			case stateSub:
				if b == iac {
					f.state = stateSubIAC
				}

			case stateSubIAC:
				if b == se {
					f.state = stateData
				} else {
					f.state = stateSub
				}
			}
		}

		// Don't report zero bytes with a nil error just because the whole
		// read was negotiation; read again instead.
		if out > 0 || err != nil {
			return out, err
		}
	}
}
