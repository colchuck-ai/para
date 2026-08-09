//go:build !para_testhooks

package writeset

// crashPoint does nothing. Production must not read PARA_CRASH_AFTER (§0.2) —
// only a para_testhooks build honours it. See the para_testhooks file beside
// this one for what it is for.
func crashPoint(Op) {}
