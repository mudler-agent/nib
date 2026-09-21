package agentmcp

import (
	"os"
	"testing"
)

// TestMain points the default config root at a throwaway directory, so a
// session built with an empty BaseDir never reads the developer's real
// credentials.json or the endpoint saved by /endpoint and /login
// (provider.json). Without it these tests restore whichever endpoint the
// developer last picked and talk to that provider instead of the fake server
// they just started, which makes their result depend on what a live model
// felt like answering. The chat package guards itself the same way.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nib-agentmcp-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
