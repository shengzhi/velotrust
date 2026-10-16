package runtime

import (
	"crypto/sha256"
	"time"

	"velotrust/gateway/config"
)

const configReloadInterval = 2 * time.Second

// fileConfigSource remembers the last file state so an invalid replacement is
// handled once, while a valid config whose dataplane apply failed can be
// retried until it succeeds or the file changes again.
type fileConfigSource struct {
	path   string
	digest [sha256.Size]byte
	bad    bool
}

func newFileConfigSource(path string, digest [sha256.Size]byte) *fileConfigSource {
	return &fileConfigSource{path: path, digest: digest}
}

// poll returns a config when it changed, or when a previous application needs
// retrying. changed reports whether the file contents or validity changed.
func (s *fileConfigSource) poll(retry bool) (*config.Config, bool, bool, error) {
	cfg, digest, err := config.LoadFileWithDigest(s.path)
	if err != nil {
		changed := !s.bad || digest != s.digest
		if changed {
			s.digest = digest
			s.bad = true
		}
		return nil, false, changed, err
	}
	changed := s.bad || digest != s.digest
	s.digest = digest
	s.bad = false
	if !changed && !retry {
		return nil, false, false, nil
	}
	return &cfg, true, changed, nil
}
