package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/llmprovider/copilot"
	"github.com/mudler/nib/plugin"
	"github.com/mudler/nib/provider"
)

// RunLoginCommand handles `nib login [provider]` and `nib login --list`.
// Without a provider, it lists loginable providers. With a provider, it runs
// the appropriate flow (OAuth or API key).
func RunLoginCommand(programName, baseDir string, args []string) int {
	prog := runnableName(programName)
	root := plugin.BaseDirIn(baseDir)
	store := auth.NewStore(credentialPath(root))

	if len(args) == 0 {
		loginUsage(prog)
		return 1
	}

	if args[0] == "--list" || args[0] == "-l" {
		return loginList(prog, store)
	}

	def, ok := provider.Get(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "%s login: unknown provider %q\n", prog, args[0])
		fmt.Fprintf(os.Stderr, "Available: %s\n", providerIDs(provider.Loginable()))
		return 1
	}

	if def.LoginKind == provider.LoginNone {
		fmt.Fprintf(os.Stderr, "%s login: %s has no login flow (use env var %s)\n", prog, def.ID, def.EnvVar)
		return 1
	}

	ctx := context.Background()

	switch def.LoginKind {
	case provider.LoginOAuthCode:
		return loginOAuth(ctx, prog, store, def)
	case provider.LoginDeviceCode:
		return loginDeviceCode(ctx, prog, store, def)
	case provider.LoginAPIKey:
		return loginAPIKey(prog, store, def)
	case provider.LoginCopilot:
		return loginCopilotToken(prog, store, def)
	default:
		fmt.Fprintf(os.Stderr, "%s login: %s has an unsupported login kind\n", prog, def.ID)
		return 1
	}
}

// RunLogoutCommand handles `nib logout [provider]`.
func RunLogoutCommand(programName, baseDir string, args []string) int {
	prog := runnableName(programName)
	root := plugin.BaseDirIn(baseDir)
	store := auth.NewStore(credentialPath(root))

	if len(args) == 0 {
		return logoutList(prog, store)
	}

	def, ok := provider.Get(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "%s logout: unknown provider %q\n", prog, args[0])
		return 1
	}

	cred, ok, err := store.Get(def.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s logout: %v\n", prog, err)
		return 1
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "%s logout: not logged in to %s\n", prog, def.ID)
		return 1
	}
	_ = cred

	if err := store.Delete(def.ID); err != nil {
		fmt.Fprintf(os.Stderr, "%s logout: %v\n", prog, err)
		return 1
	}
	fmt.Printf("Logged out of %s (%s)\n", def.Name, def.ID)
	return 0
}

func loginOAuth(ctx context.Context, prog string, store *auth.Store, def provider.Definition) int {
	fmt.Printf("Starting OAuth login for %s...\n", def.Name)
	flow, err := auth.StartOAuthFlow(def)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		return 1
	}
	url := flow.AuthorizeURL()
	fmt.Printf("\nOpen this URL in your browser:\n  %s\n\n", url)
	openBrowser(url)
	fmt.Println("Waiting for authorization...")

	cred, err := flow.Complete(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		return 1
	}
	if err := store.Save(cred); err != nil {
		fmt.Fprintf(os.Stderr, "%s login: save credential: %v\n", prog, err)
		return 1
	}
	fmt.Printf("Logged in to %s as %s\n", def.Name, cred.DisplayLabel())
	return 0
}

func loginDeviceCode(ctx context.Context, prog string, store *auth.Store, def provider.Definition) int {
	fmt.Printf("Starting device login for %s...\n", def.Name)
	cred, err := auth.LoginDeviceCode(ctx, store, def, func(instructions, url string) {
		fmt.Printf("\n%s\n", instructions)
		openBrowser(url)
		fmt.Println("Waiting for authorization...")
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		return 1
	}
	fmt.Printf("Logged in to %s as %s\n", def.Name, cred.DisplayLabel())
	return 0
}

func loginCopilotToken(prog string, store *auth.Store, def provider.Definition) int {
	fmt.Printf("Importing GitHub Copilot token for %s...\n", def.Name)
	token, err := copilot.ResolveToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		fmt.Fprintf(os.Stderr, "Install gh CLI and run 'gh auth login', or set %s\n", def.EnvVar)
		return 1
	}
	cred, err := auth.LoginAPIKey(store, def, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		return 1
	}
	fmt.Printf("Logged in to %s (%s)\n", def.Name, cred.StatusLine())
	return 0
}

func loginAPIKey(prog string, store *auth.Store, def provider.Definition) int {
	fmt.Printf("Enter API key for %s (input is hidden): ", def.Name)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s login: read key: %v\n", prog, err)
		return 1
	}
	key := strings.TrimSpace(line)
	if key == "" {
		fmt.Fprintf(os.Stderr, "\n%s login: empty key, aborting\n", prog)
		return 1
	}
	cred, err := auth.LoginAPIKey(store, def, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login: %v\n", prog, err)
		return 1
	}
	fmt.Printf("Logged in to %s (%s)\n", def.Name, cred.DisplayLabel())
	return 0
}

func loginList(prog string, store *auth.Store) int {
	creds, err := store.All()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s login --list: %v\n", prog, err)
		return 1
	}
	if len(creds) == 0 {
		fmt.Println("No providers logged in. Run: nib login <provider>")
		return 0
	}
	for _, c := range creds {
		fmt.Printf("%s  %s\n", c.ProviderID, c.StatusLine())
	}
	return 0
}

func logoutList(prog string, store *auth.Store) int {
	creds, err := store.All()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s logout: %v\n", prog, err)
		return 1
	}
	if len(creds) == 0 {
		fmt.Println("No providers logged in.")
		return 0
	}
	fmt.Println("Logged in to:")
	for _, c := range creds {
		fmt.Printf("  %s  %s\n", c.ProviderID, c.StatusLine())
	}
	fmt.Printf("\nRun: %s logout <provider>\n", prog)
	return 0
}

func loginUsage(prog string) {
	fmt.Printf("Usage: %s login <provider>\n", prog)
	fmt.Printf("       %s login --list\n\n", prog)
	fmt.Println("Providers with login:")
	for _, d := range provider.Loginable() {
		fmt.Printf("  %-12s  %s  (%s)\n", d.ID, d.Name, d.LoginKind)
	}
}

func credentialPath(root string) string {
	return root + string(os.PathSeparator) + "credentials.json"
}

func providerIDs(defs []provider.Definition) string {
	ids := make([]string, len(defs))
	for i, d := range defs {
		ids[i] = d.ID
	}
	return strings.Join(ids, ", ")
}

// openBrowser attempts to open url in the user's default browser. Failure is
// non-fatal — the URL is already printed for manual entry.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
