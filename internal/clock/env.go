//go:build !para_testhooks

package clock

// FromEnv reports no override. Production must not read PARA_NOW or PARA_TZ
// (§0.2) — only a para_testhooks build honors them.
func FromEnv() (Clock, error) {
	return nil, nil
}
