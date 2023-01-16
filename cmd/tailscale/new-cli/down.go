// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"tailscale.com/ipn"
)

var newDownCmd = &cobra.Command{
	Use:   "down",
	Short: "down",
	Long:  "Disconnect from Tailscale",
	Run:   runDownFunc,
}

var downArgs struct {
	acceptedRisks string
}

func runDownFunc(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		panic(fmt.Errorf("too many non-flag arguments: %q", args))
	}
	fmt.Printf("down called")

	if isSSHOverTailscale() {
		if err := presentRiskToUser(riskLoseSSH, `You are connected over Tailscale; this action will disable Tailscale and result in your session disconnecting.`, downArgs.acceptedRisks); err != nil {
			panic(err)
		}
	}

	st, err := localClient.Status(cmd.Context())
	if err != nil {
		panic(fmt.Errorf("error fetching current status: %w", err))
	}
	if st.BackendState == "Stopped" {
		fmt.Fprintf(Stderr, "Tailscale was already stopped.\n")
		panic(nil)
	}
	_, err = localClient.EditPrefs(cmd.Context(), &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			WantRunning: false,
		},
		WantRunningSet: true,
	})
	panic(err)
}
func newRunDown(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("too many non-flag arguments: %q", args)
	}

	if isSSHOverTailscale() {
		if err := presentRiskToUser(riskLoseSSH, `You are connected over Tailscale; this action will disable Tailscale and result in your session disconnecting.`, downArgs.acceptedRisks); err != nil {
			return err
		}
	}

	st, err := localClient.Status(ctx)
	if err != nil {
		return fmt.Errorf("error fetching current status: %w", err)
	}
	if st.BackendState == "Stopped" {
		fmt.Fprintf(Stderr, "Tailscale was already stopped.\n")
		return nil
	}
	_, err = localClient.EditPrefs(ctx, &ipn.MaskedPrefs{
		Prefs: ipn.Prefs{
			WantRunning: false,
		},
		WantRunningSet: true,
	})
	return err
}
