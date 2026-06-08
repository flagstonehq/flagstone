package openfeature

import (
	"context"

	"github.com/flagstonehq/flagstone/pkg/sdk"
	of "github.com/open-feature/go-sdk/openfeature"
)

var _ of.FeatureProvider = (*Provider)(nil)

// Provider is an OpenFeature FeatureProvider backed by a Flagstone SDK client.
type Provider struct {
	client *sdk.Client
}

// NewProvider wraps a Flagstone SDK client as an OpenFeature FeatureProvider.
func NewProvider(client *sdk.Client) *Provider {
	return &Provider{client: client}
}

// Metadata returns the provider name.
func (p *Provider) Metadata() of.Metadata {
	return of.Metadata{Name: "flagstone"}
}

// Hooks returns no hooks (Flagstone handles observability server-side).
func (p *Provider) Hooks() []of.Hook {
	return nil
}

// BooleanEvaluation evaluates a boolean flag.
func (p *Provider) BooleanEvaluation(ctx context.Context, flagKey string, defaultValue bool, evalCtx of.FlattenedContext) of.BoolResolutionDetail {
	d := p.client.BoolDetail(ctx, flagKey, defaultValue, evalCtx)
	return of.BoolResolutionDetail{
		Value:                    d.Value.(bool),
		ProviderResolutionDetail: toDetail(d),
	}
}

// StringEvaluation evaluates a string flag.
func (p *Provider) StringEvaluation(ctx context.Context, flagKey, defaultValue string, evalCtx of.FlattenedContext) of.StringResolutionDetail {
	d := p.client.StringDetail(ctx, flagKey, defaultValue, evalCtx)
	return of.StringResolutionDetail{
		Value:                    d.Value.(string),
		ProviderResolutionDetail: toDetail(d),
	}
}

// FloatEvaluation evaluates a numeric flag as float64.
func (p *Provider) FloatEvaluation(ctx context.Context, flagKey string, defaultValue float64, evalCtx of.FlattenedContext) of.FloatResolutionDetail {
	d := p.client.NumberDetail(ctx, flagKey, defaultValue, evalCtx)
	return of.FloatResolutionDetail{
		Value:                    d.Value.(float64),
		ProviderResolutionDetail: toDetail(d),
	}
}

// IntEvaluation evaluates a numeric flag as int64 (truncates float64).
func (p *Provider) IntEvaluation(ctx context.Context, flagKey string, defaultValue int64, evalCtx of.FlattenedContext) of.IntResolutionDetail {
	d := p.client.NumberDetail(ctx, flagKey, float64(defaultValue), evalCtx)
	return of.IntResolutionDetail{
		Value:                    int64(d.Value.(float64)),
		ProviderResolutionDetail: toDetail(d),
	}
}

// ObjectEvaluation evaluates a JSON flag.
func (p *Provider) ObjectEvaluation(ctx context.Context, flagKey string, defaultValue any, evalCtx of.FlattenedContext) of.InterfaceResolutionDetail {
	d := p.client.JSONDetail(ctx, flagKey, defaultValue, evalCtx)
	return of.InterfaceResolutionDetail{
		Value:                    d.Value,
		ProviderResolutionDetail: toDetail(d),
	}
}

func toDetail(d sdk.EvaluationDetail) of.ProviderResolutionDetail {
	var err of.ResolutionError
	if d.Error != nil {
		err = toError(d.Reason, d.Error)
	}
	return of.ProviderResolutionDetail{
		ResolutionError: err,
		Reason:          toReason(d.Reason),
		Variant:         d.Variant,
	}
}

func toReason(r sdk.Reason) of.Reason {
	switch r {
	case sdk.ReasonRuleMatch:
		return of.TargetingMatchReason
	case sdk.ReasonDefault:
		return of.DefaultReason
	case sdk.ReasonDisabled:
		return of.DisabledReason
	case sdk.ReasonFlagNotFound, sdk.ReasonFlagArchived:
		return of.ErrorReason
	case sdk.ReasonError:
		return of.ErrorReason
	default:
		return of.UnknownReason
	}
}

func toError(r sdk.Reason, err error) of.ResolutionError {
	switch r {
	case sdk.ReasonFlagNotFound, sdk.ReasonFlagArchived:
		return of.NewFlagNotFoundResolutionError(err.Error())
	default:
		return of.NewGeneralResolutionError(err.Error())
	}
}
