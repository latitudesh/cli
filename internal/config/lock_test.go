package config

import (
	"path/filepath"
	"sync"
	"testing"
)

// TestUpdateSerializesConcurrentWriters covers the transaction guarantee: two
// processes saving different keys at the same time must both survive. Without
// the lock each writer holds its own snapshot and the last rename drops the
// other's entry — losing a secret the API only returns once.
func TestUpdateSerializesConcurrentWriters(t *testing.T) {
	t.Setenv("LSH_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	if err := Update(func(f *File) error {
		f.SetProfile("p", Profile{Authorization: "token"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- Update(func(f *File) error {
				_, p, err := f.Resolve("p")
				if err != nil {
					return err
				}
				p.SetObjectStorageKey(string(rune('a'+i)), StoredAccessKey{AccessKeyID: "AK", Scope: ScopeFullAccess})
				f.SetProfile("p", p)
				return nil
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
	}

	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	_, p, err := f.Resolve("p")
	if err != nil {
		t.Fatal(err)
	}
	keys := p.ObjectStorageKeys()
	if len(keys) != writers {
		t.Errorf("%d keys survived, want %d — a concurrent save overwrote another: %v", len(keys), writers, keys)
	}
	if p.Authorization != "token" {
		t.Errorf("the rest of the profile must be preserved, got %+v", p)
	}
}

// TestUpdateMutateErrorLeavesFileAlone keeps a failed transaction from writing.
func TestUpdateMutateErrorLeavesFileAlone(t *testing.T) {
	t.Setenv("LSH_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	if err := Update(func(f *File) error {
		f.SetProfile("p", Profile{Authorization: "first"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentinel := errNoWrite{}
	if err := Update(func(f *File) error {
		f.SetProfile("p", Profile{Authorization: "second"})
		return sentinel
	}); err != sentinel {
		t.Fatalf("Update returned %v, want the mutate error", err)
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	_, p, err := f.Resolve("p")
	if err != nil {
		t.Fatal(err)
	}
	if p.Authorization != "first" {
		t.Errorf("a failed transaction must not be written, got %q", p.Authorization)
	}
}

type errNoWrite struct{}

func (errNoWrite) Error() string { return "no write" }
