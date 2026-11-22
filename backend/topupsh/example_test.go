// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

package topupsh_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/topupsh/topupsh-tracker/backend/topupsh"
)

// A service charges before a tool call, refunds when the call fails, and
// decides itself what happens while topup.sh is unavailable.
func Example() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"requestId":"r1","charged":false,"limited":true,"reason":"user_limit","renewsAt":"2026-12-01T00:00:00Z"}`))
	}))
	defer srv.Close()
	c, err := topupsh.New(topupsh.Config{BaseURL: srv.URL, APIKey: "tpk_example"})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	ch, err := c.Charge(ctx, topupsh.ChargeRequest{Org: "acme", User: "ada", Credits: 1, Operation: "tool_call"})
	switch {
	case errors.Is(err, topupsh.ErrUnavailable):
		fmt.Println("topup.sh is down: refuse, or allow uncharged")
		return
	case err != nil:
		panic(err)
	case !ch.Charged:
		fmt.Println("refused:", ch.Reason, "until", ch.RenewsAt.Format("2006-01-02"))
		return
	}
	// Run the tool call; when it fails, give the credits back with
	// c.Refund(ctx, ch.RequestID).
	// Output: refused: user_limit until 2026-12-01
}
