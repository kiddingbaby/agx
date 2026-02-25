package usecase

import (
	"errors"
	"fmt"
	"testing"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
)

var (
	benchmarkFindByIdentifierKey *domainkey.Key
	benchmarkFindByIdentifierErr error
)

func buildBenchmarkKeys(n int) []domainkey.Key {
	keys := make([]domainkey.Key, n)
	for i := 0; i < n; i++ {
		keys[i] = domainkey.Key{
			ID:       fmt.Sprintf("bench-id-%04d-abcdef", i),
			Name:     fmt.Sprintf("bench-key-%04d", i),
			Provider: domainkey.ProviderClaude,
		}
	}
	return keys
}

func BenchmarkFindByIdentifier(b *testing.B) {
	const keyCount = 1000

	b.Run("NameHit", func(b *testing.B) {
		svc := NewKeyService(&fakeKeyRepo{keys: buildBenchmarkKeys(keyCount)})
		target := fmt.Sprintf("bench-key-%04d", keyCount-1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			k, err := svc.FindByIdentifier(target)
			if err != nil {
				b.Fatalf("FindByIdentifier(name) error = %v", err)
			}
			benchmarkFindByIdentifierKey = k
		}
	})

	b.Run("PrefixHit", func(b *testing.B) {
		svc := NewKeyService(&fakeKeyRepo{keys: buildBenchmarkKeys(keyCount)})
		target := fmt.Sprintf("bench-id-%04d", keyCount-1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			k, err := svc.FindByIdentifier(target)
			if err != nil {
				b.Fatalf("FindByIdentifier(prefix) error = %v", err)
			}
			benchmarkFindByIdentifierKey = k
		}
	})

	b.Run("Miss", func(b *testing.B) {
		svc := NewKeyService(&fakeKeyRepo{keys: buildBenchmarkKeys(keyCount)})
		target := "missing-identifier"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			k, err := svc.FindByIdentifier(target)
			if err == nil {
				b.Fatal("FindByIdentifier(miss) expected error")
			}
			if !errors.Is(err, ErrKeyNotFound) {
				b.Fatalf("FindByIdentifier(miss) err = %v, want ErrKeyNotFound", err)
			}
			benchmarkFindByIdentifierKey = k
			benchmarkFindByIdentifierErr = err
		}
	})
}
