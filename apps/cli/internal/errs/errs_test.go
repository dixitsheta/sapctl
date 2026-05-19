package errs

import (
	"errors"
	"testing"
)

func TestCodeOfNil(t *testing.T) {
	if got := CodeOf(nil); got != ExitSuccess {
		t.Fatalf("CodeOf(nil) = %d, want %d", got, ExitSuccess)
	}
}

func TestCodeOfPlainError(t *testing.T) {
	if got := CodeOf(errors.New("boom")); got != ExitUserError {
		t.Fatalf("CodeOf(plain) = %d, want %d", got, ExitUserError)
	}
}

func TestCodeOfSapctlError(t *testing.T) {
	cases := map[int]*Error{
		ExitNotFound:    New(ExitNotFound, "x.not_found", "missing"),
		ExitConflict:    New(ExitConflict, "x.conflict", "dup"),
		ExitAuth:        New(ExitAuth, "auth.expired", "expired"),
		ExitRateLimited: New(ExitRateLimited, "api.rate_limited", "slow down"),
	}
	for want, e := range cases {
		if got := CodeOf(e); got != want {
			t.Errorf("CodeOf(%v) = %d, want %d", e.Tag, got, want)
		}
	}
}

func TestWrapPreservesCode(t *testing.T) {
	inner := errors.New("inner")
	e := Wrap(ExitAuth, "auth.token_expired", "could not refresh", inner)
	if CodeOf(e) != ExitAuth {
		t.Fatalf("wrapped err lost code")
	}
	if !errors.Is(e, inner) {
		t.Fatalf("errors.Is failed: should chain inner")
	}
	if e.Error() == "" {
		t.Fatalf("Error() empty")
	}
}

func TestErrorsAs(t *testing.T) {
	src := New(ExitConflict, "x.conflict", "dup")
	var dst *Error
	if !errors.As(error(src), &dst) {
		t.Fatalf("errors.As failed")
	}
	if dst.Tag != "x.conflict" {
		t.Fatalf("As recovered wrong tag: %q", dst.Tag)
	}
}
