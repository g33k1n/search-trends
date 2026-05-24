package trends

import (
	"errors"
	"fmt"
	"testing"
)

func TestDomainErrorIsBySentinelIdentity(t *testing.T) {
	if !errors.Is(ErrEmptyQuery, ErrEmptyQuery) {
		t.Fatal("sentinel should match itself via errors.Is")
	}
}

func TestDomainErrorIsByCode(t *testing.T) {
	other := &DomainError{Code: CodeEmptyQuery, Message: "different message"}
	if !errors.Is(other, ErrEmptyQuery) {
		t.Fatal("errors.Is should compare by Code, not pointer")
	}
}

func TestDomainErrorIsDifferentCode(t *testing.T) {
	if errors.Is(ErrEmptyQuery, ErrQueryBlocked) {
		t.Fatal("different codes must not match")
	}
}

func TestDomainErrorIsThroughWrap(t *testing.T) {
	wrapped := fmt.Errorf("ingest: %w", ErrRateLimited)
	if !errors.Is(wrapped, ErrRateLimited) {
		t.Fatal("errors.Is must unwrap")
	}
}

func TestCodeOfDirect(t *testing.T) {
	code, ok := CodeOf(ErrQueryBlocked)
	if !ok || code != CodeQueryBlocked {
		t.Fatalf("CodeOf(ErrQueryBlocked) = (%d, %v), want (%d, true)", code, ok, CodeQueryBlocked)
	}
}

func TestCodeOfWrapped(t *testing.T) {
	wrapped := fmt.Errorf("store: %w", ErrEventOutOfWindow)
	code, ok := CodeOf(wrapped)
	if !ok || code != CodeEventOutOfWindow {
		t.Fatalf("CodeOf(wrapped) = (%d, %v), want (%d, true)", code, ok, CodeEventOutOfWindow)
	}
}

func TestCodeOfNonDomain(t *testing.T) {
	_, ok := CodeOf(errors.New("random"))
	if ok {
		t.Fatal("CodeOf on non-domain error should return false")
	}
}
