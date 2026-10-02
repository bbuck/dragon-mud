package auth

// Hasher hashes and checks passwords off the caller's goroutine, at most a
// fixed number at a time. Each hash holds Params.Memory while it runs, so
// the limit bounds memory however many players log in at once. Requests
// beyond the limit wait their turn.
type Hasher struct {
	params Params
	slots  chan struct{}
}

// NewHasher returns a Hasher that runs at most concurrency hashes at once.
func NewHasher(p Params, concurrency int) *Hasher {
	return &Hasher{params: p, slots: make(chan struct{}, max(concurrency, 1))}
}

// Hash hashes password in the background and calls done with the result.
// done runs on another goroutine.
func (h *Hasher) Hash(password string, done func(hash string, err error)) {
	go func() {
		h.slots <- struct{}{}
		hash, err := HashPassword(password, h.params)
		<-h.slots

		done(hash, err)
	}()
}

// Check checks password against hash in the background and calls done with
// the result. rehash is true when the password matched but hash uses old
// parameters and should be replaced. done runs on another goroutine.
func (h *Hasher) Check(password, hash string, done func(ok, rehash bool, err error)) {
	go func() {
		h.slots <- struct{}{}
		ok, err := CheckPassword(password, hash)
		<-h.slots

		done(ok, ok && NeedsRehash(hash, h.params), err)
	}()
}
