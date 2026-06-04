// Package sdk is the public Go SDK for Flagstone feature flags.
//
// # Usage
//
//	client, err := sdk.New(
//	    sdk.WithEndpoint("https://api.flagstone.dev"),
//	    sdk.WithAPIKey(os.Getenv("FLAGSTONE_API_KEY")),
//	)
//	if err != nil { return err }
//	defer client.Close()
//
//	ctx := context.Background()
//	_ = client.Start(ctx)  // loads the snapshot, then subscribes to SSE
//
//	// Safe-default API: pass the value to serve if the flag is missing,
//	// the wrong type, or not loaded yet. Never returns an error or blocks.
//	enabled := client.Bool(ctx, "new-checkout", false, map[string]any{
//	    "user_id": "42",
//	    "country": "AR",
//	})
//
//	// Use BoolDetail when you also want the reason or error:
//	d := client.BoolDetail(ctx, "new-checkout", false, nil)
//	_ = d.Reason // RULE_MATCH, DEFAULT, FLAG_NOT_FOUND, ...
//
// # Stability
//
// This package follows semantic versioning. Breaking changes to public types
// (Client, Options functions, evaluation methods) will only happen in a major
// version bump.
package sdk
