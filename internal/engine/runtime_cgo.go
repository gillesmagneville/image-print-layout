// SPDX-License-Identifier: GPL-3.0-only

//go:build cgo

package engine

import (
	"os"
	"runtime"
	"sort"
)

// ConfigureRuntime points the libraries the engine uses at the data shipped
// next to the program (see BundledEnvironment). It acts on Windows only, where
// the packages carry their own fontconfig and image loaders; on Linux the
// distribution provides them. A variable the user has already set is left
// alone. It must run before the first use of the engine: the libraries read
// their configuration once, when they first need it.
func ConfigureRuntime() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	env := BundledEnvironment(exe, func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	})
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if os.Getenv(k) != "" {
			continue
		}
		if err := setEnv(k, env[k]); err != nil {
			return err
		}
	}
	return nil
}
