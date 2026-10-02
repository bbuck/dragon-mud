package auth

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// cheap keeps tests fast; never use it for real passwords.
var cheap = Params{Memory: 64, Time: 1, Threads: 1}

func TestHashAndCheck(t *testing.T) {
	hash, err := HashPassword("hunter2", cheap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("hash %q isn't in the standard format", hash)
	}

	if ok, err := CheckPassword("hunter2", hash); err != nil || !ok {
		t.Errorf("CheckPassword(correct) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := CheckPassword("hunter3", hash); err != nil || ok {
		t.Errorf("CheckPassword(wrong) = %v, %v; want false, nil", ok, err)
	}

	again, _ := HashPassword("hunter2", cheap)
	if again == hash {
		t.Error("two hashes of the same password share a salt")
	}
}

func TestCheckInvalidHash(t *testing.T) {
	for _, hash := range []string{
		"not a hash",
		"$2a$10$abcdefghijklmnopqrstuv", // bcrypt
		"$argon2id$v=19$m=0,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$!!!$a2V5",
	} {
		if _, err := CheckPassword("hunter2", hash); err == nil {
			t.Errorf("CheckPassword(%q): expected an error", hash)
		}
	}
}

func TestHashRejectsBadParams(t *testing.T) {
	for _, p := range []Params{{Memory: 64, Time: 0, Threads: 1}, {Memory: 4, Time: 1, Threads: 1}} {
		if _, err := HashPassword("hunter2", p); err == nil {
			t.Errorf("HashPassword with %+v: expected error", p)
		}
	}
}

func TestNeedsRehash(t *testing.T) {
	hash, _ := HashPassword("hunter2", cheap)

	if NeedsRehash(hash, cheap) {
		t.Error("hash with current params needs rehash")
	}
	if !NeedsRehash(hash, Params{Memory: 128, Time: 1, Threads: 1}) {
		t.Error("hash with old params doesn't need rehash")
	}
}

func TestHasher(t *testing.T) {
	h := NewHasher(cheap, 1)

	hashes := make(chan string)
	h.Hash("hunter2", func(hash string, err error) {
		if err != nil {
			t.Error(err)
		}
		hashes <- hash
	})
	hash := <-hashes

	type result struct{ ok, rehash bool }
	results := make(chan result)
	h.Check("hunter2", hash, func(ok, rehash bool, err error) { results <- result{ok, rehash} })
	if r := <-results; !r.ok || r.rehash {
		t.Errorf("Check(correct) = %+v", r)
	}
	h.Check("wrong", hash, func(ok, rehash bool, err error) { results <- result{ok, rehash} })
	if r := <-results; r.ok || r.rehash {
		t.Errorf("Check(wrong) = %+v", r)
	}

	stronger := NewHasher(Params{Memory: 128, Time: 1, Threads: 1}, 1)
	stronger.Check("hunter2", hash, func(ok, rehash bool, err error) { results <- result{ok, rehash} })
	if r := <-results; !r.ok || !r.rehash {
		t.Errorf("Check with stronger params = %+v, want ok and rehash", r)
	}
}

func TestHasherLimitsConcurrency(t *testing.T) {
	const limit = 2
	h := NewHasher(cheap, limit)

	// Fill every slot by hand, then check that queued work waits for one.
	for range limit {
		h.slots <- struct{}{}
	}

	var finished atomic.Int32
	done := make(chan struct{})
	h.Hash("hunter2", func(string, error) {
		finished.Add(1)
		close(done)
	})

	time.Sleep(20 * time.Millisecond)
	if finished.Load() != 0 {
		t.Fatal("hash ran while every slot was taken")
	}

	<-h.slots
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("hash never ran after a slot freed up")
	}
}
