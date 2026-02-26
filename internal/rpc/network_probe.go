// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ResolveNetwork probes all known Stellar networks concurrently and returns the
// first one on which hash is found. It is called by the debug command when
// --network is not explicitly provided by the user.
func ResolveNetwork(ctx context.Context, hash string, token string) (Network, error) {
	return resolveNetwork(ctx, hash, token, nil)
}

// resolveNetwork is the testable core. overrideURLs maps each Network to a
// custom Horizon URL; when nil or a network is absent, the default URL is used.
func resolveNetwork(ctx context.Context, hash string, token string, overrideURLs map[Network]string) (Network, error) {
	candidates := []Network{Mainnet, Testnet, Futurenet}

	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	found := make(chan Network, 1)
	var wg sync.WaitGroup

	for _, net := range candidates {
		wg.Add(1)
		go func(n Network) {
			defer wg.Done()

			opts := []ClientOption{WithNetwork(n), WithToken(token)}
			if url, ok := overrideURLs[n]; ok {
				opts = append(opts, WithHorizonURL(url))
			}

			client, err := NewClient(opts...)
			if err != nil {
				return
			}

			_, err = client.GetTransaction(probeCtx, hash)
			if err == nil {
				select {
				case found <- n:
					cancel()
				default:
				}
			}
		}(net)
	}

	go func() {
		wg.Wait()
		close(found)
	}()

	if n, ok := <-found; ok {
		return n, nil
	}

	return "", fmt.Errorf("transaction not found on %s",
		strings.Join([]string{string(Mainnet), string(Testnet), string(Futurenet)}, ", "))
}
