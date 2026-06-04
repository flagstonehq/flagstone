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
//	go client.Start(ctx)  // subscribes to SSE for live updates
//
//	enabled, _ := client.Bool(ctx, "new-checkout", map[string]any{
//	    "user_id": 42,
//	    "country": "AR",
//	})
//
// # Stability
//
// This package follows semantic versioning. Breaking changes to public types
// (Client, Options functions, evaluation methods) will only happen in a major
// version bump.
package sdk
