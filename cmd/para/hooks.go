//go:build !para_testhooks

package main

// testHooksEnabled reports whether this binary honours PARA_NOW and PARA_TZ.
// Production must not (§0.2), which is why the script suite is only meaningful
// under the para_testhooks tag — see TestScripts.
const testHooksEnabled = false
