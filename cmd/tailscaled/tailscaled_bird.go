// Copyright (c) 2021 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.19 && (linux || darwin || freebsd || openbsd) && !ts_omit_bird

package main

import (
	"tailscale.com/chirp"
	"tailscale.com/wgengine"
)

func init() {
	createBIRDClient = func(ctlSocket string) (wgengine.BIRDClient, error) {
		return chirp.New(ctlSocket)
	}
}
