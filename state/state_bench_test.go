package state

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func BenchmarkStateMutate(b *testing.B) {
	tmpDir := b.TempDir()
	b.Setenv("XDG_DATA_HOME", tmpDir)
	xdg.Reload()

	app := &InstalledApp{
		Repository: "test/bench",
		Version:    "1.0.0",
		TargetPath: filepath.Join(tmpDir, "bin"),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Mutate(func(s *State) error {
			if s.Apps == nil {
				s.Apps = make(map[string]*InstalledApp)
			}
			app.Version = fmt.Sprintf("1.0.%d", i)
			s.Apps["test/bench"] = app
			return nil
		})
		if err != nil {
			b.Fatalf("mutate failed: %v", err)
		}
	}
}
