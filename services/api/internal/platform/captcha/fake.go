package captcha

import "context"

// FakeVerifier is a test/dev verifier with a fixed outcome.
type FakeVerifier struct{ Pass bool }

func (f FakeVerifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	return f.Pass, nil
}
