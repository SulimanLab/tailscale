// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cli contains the cmd/tailscale CLI code in a package that can be included
// in other wrapper binaries such as the Mac and Windows clients.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	flag2 "github.com/spf13/pflag"
	"tailscale.com/client/tailscale"
	"tailscale.com/paths"
)

var Stderr io.Writer = os.Stderr
var Stdout io.Writer = os.Stdout

func errf(format string, a ...any) {
	fmt.Fprintf(Stderr, format, a...)
}

func printf(format string, a ...any) {
	fmt.Fprintf(Stdout, format, a...)
}

// outln is like fmt.Println in the common case, except when Stdout is
// changed (as in js/wasm).
//
// It's not named println because that looks like the Go built-in
// which goes to stderr and formats slightly differently.
func outln(a ...any) {
	fmt.Fprintln(Stdout, a...)
}

// ActLikeCLI reports whether a GUI application should act like the
// CLI based on os.Args, GOOS, the context the process is running in
// (pty, parent PID), etc.
func ActLikeCLI() bool {
	// This function is only used on macOS.
	if runtime.GOOS != "darwin" {
		return false
	}

	// Escape hatch to let people force running the macOS
	// GUI Tailscale binary as the CLI.
	if v, _ := strconv.ParseBool(os.Getenv("TAILSCALE_BE_CLI")); v {
		return true
	}

	// If our parent is launchd, we're definitely not
	// being run as a CLI.
	if os.Getppid() == 1 {
		return false
	}

	// Xcode adds the -NSDocumentRevisionsDebugMode flag on execution.
	// If present, we are almost certainly being run as a GUI.
	for _, arg := range os.Args {
		if arg == "-NSDocumentRevisionsDebugMode" {
			return false
		}
	}

	// Looking at the environment of the GUI Tailscale app (ps eww
	// $PID), empirically none of these environment variables are
	// present. But all or some of these should be present with
	// Terminal.all and bash or zsh.
	for _, e := range []string{
		"SHLVL",
		"TERM",
		"TERM_PROGRAM",
		"PS1",
	} {
		if os.Getenv(e) != "" {
			return true
		}
	}
	return false
}

func newFlagSet(name string) *flag2.FlagSet {
	onError := flag2.ExitOnError
	if runtime.GOOS == "js" {
		onError = flag2.ContinueOnError
	}
	fs := flag2.NewFlagSet(name, onError)
	fs.SetOutput(Stderr)
	return fs
}

// CleanUpArgs rewrites command line arguments for simplicity and backwards compatibility.
// In particular, it rewrites --authkey to --auth-key.
func CleanUpArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		// Rewrite --authkey to --auth-key, and --authkey=x to --auth-key=x,
		// and the same for the -authkey variant.
		switch {
		case arg == "--authkey", arg == "-authkey":
			arg = "--auth-key"
		case strings.HasPrefix(arg, "--authkey="), strings.HasPrefix(arg, "-authkey="):
			arg = strings.TrimLeft(arg, "-")
			arg = strings.TrimPrefix(arg, "authkey=")
			arg = "--auth-key=" + arg
		}
		out = append(out, arg)
	}
	return out
}

var localClient tailscale.LocalClient

// Run runs the CLI. The args do not include the binary name.
func Run(args []string) (err error) {
	args = CleanUpArgs(args)

	if len(args) == 1 && (args[0] == "-V" || args[0] == "--version") {
		args = []string{"version"}
	}

	var warnOnce sync.Once
	tailscale.SetVersionMismatchHandler(func(clientVer, serverVer string) {
		warnOnce.Do(func() {
			fmt.Fprintf(Stderr, "Warning: client version %q != tailscaled server version %q\n", clientVer, serverVer)
		})
	})

	rootfs := newFlagSet("tailscale")
	rootfs.StringVar(&rootArgs.socket, "socket", paths.DefaultTailscaledSocket(), "path to tailscaled socket")

	root := &cobra.Command{
		Use:   "tailscale [flags] <subcommand> [command flags]",
		Short: "Tailscale client",
		Long: strings.TrimSpace(`
For help on subcommands, add --help after: "tailscale status --help".

This CLI is still under active development. Commands and flags will
change in the future.
`),
		Args: cobra.NoArgs,
	}
	root.PersistentFlags().AddFlagSet(rootfs)
	root.AddCommand(
		newDownCmd,
	)

	localClient.Socket = rootArgs.socket
	rootfs.Visit(func(f *flag.Flag) {
		if f.Name == "socket" {
			localClient.UseSocketOnly = true
		}
	})

	if tailscale.IsAccessDeniedError(err) && os.Getuid() != 0 && runtime.GOOS != "windows" {
		return fmt.Errorf("%v\n\nUse 'sudo tailscale %s' or 'tailscale up --operator=$USER' to not require root.", err, strings.Join(args, " "))
	}
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func fatalf(format string, a ...any) {
	if Fatalf != nil {
		Fatalf(format, a...)
		return
	}
	log.SetFlags(0)
	log.Fatalf(format, a...)
}

// Fatalf, if non-nil, is used instead of log.Fatalf.
var Fatalf func(format string, a ...any)

var rootArgs struct {
	socket string
}

// usageFuncNoDefaultValues is like usageFunc but doesn't print default values.
func usageFuncNoDefaultValues(c *ffcli.Command) string {
	return usageFuncOpt(c, false)
}

func usageFunc(c *ffcli.Command) string {
	return usageFuncOpt(c, true)
}

func usageFuncOpt(c *ffcli.Command, withDefaults bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "USAGE\n")
	if c.ShortUsage != "" {
		fmt.Fprintf(&b, "  %s\n", c.ShortUsage)
	} else {
		fmt.Fprintf(&b, "  %s\n", c.Name)
	}
	fmt.Fprintf(&b, "\n")

	if c.LongHelp != "" {
		fmt.Fprintf(&b, "%s\n\n", c.LongHelp)
	}

	if len(c.Subcommands) > 0 {
		fmt.Fprintf(&b, "SUBCOMMANDS\n")
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		for _, subcommand := range c.Subcommands {
			fmt.Fprintf(tw, "  %s\t%s\n", subcommand.Name, subcommand.ShortHelp)
		}
		tw.Flush()
		fmt.Fprintf(&b, "\n")
	}

	if countFlags(c.FlagSet) > 0 {
		fmt.Fprintf(&b, "FLAGS\n")
		tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		c.FlagSet.VisitAll(func(f *flag.Flag) {
			var s string
			name, usage := flag.UnquoteUsage(f)
			if strings.HasPrefix(usage, "HIDDEN: ") {
				return
			}
			if isBoolFlag(f) {
				s = fmt.Sprintf("  --%s, --%s=false", f.Name, f.Name)
			} else {
				s = fmt.Sprintf("  --%s", f.Name) // Two spaces before --; see next two comments.
				if len(name) > 0 {
					s += " " + name
				}
			}
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			s += "\n    \t"
			s += strings.ReplaceAll(usage, "\n", "\n    \t")

			showDefault := f.DefValue != "" && withDefaults
			// Issue 6766: don't show the default Windows socket path. It's long
			// and distracting. And people on on Windows aren't likely to ever
			// change it anyway.
			if runtime.GOOS == "windows" && f.Name == "socket" && strings.HasPrefix(f.DefValue, `\\.\pipe\ProtectedPrefix\`) {
				showDefault = false
			}
			if showDefault {
				s += fmt.Sprintf(" (default %s)", f.DefValue)
			}

			fmt.Fprintln(&b, s)
		})
		tw.Flush()
		fmt.Fprintf(&b, "\n")
	}

	return strings.TrimSpace(b.String())
}

func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface {
		IsBoolFlag() bool
	})
	return ok && bf.IsBoolFlag()
}

func countFlags(fs *flag.FlagSet) (n int) {
	fs.VisitAll(func(*flag.Flag) { n++ })
	return n
}