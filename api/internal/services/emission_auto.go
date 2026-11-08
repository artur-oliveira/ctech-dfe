package services

// emission_auto.go: the API pre-check of the automatic contingency (docs/specs/2026-11-08-contingencia.md, 4.1,
// "Pre-checagem na API"). Emit already reads the fiscal configuration; when the worker's breaker is open and the
// owner enabled the switch, the document type moves to the contingency the owner chose BEFORE the access key is
// generated, so the first and only emission of that number is already in contingency (no tpEmis 1 attempt left
// in flight, hence no duplicate risk). Best effort: any failure leaves the emission normal.

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	dfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/go-dfe/autocontingency"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// autoSwitchEnabled is autocontingency.SwitchEnabled; a variable only so tests can turn it on.
var autoSwitchEnabled = autocontingency.SwitchEnabled

// AutoSwitcher is the repository side of the pre-check (FiscalConfigRepository.SwitchAuto).
type AutoSwitcher interface {
	SwitchAuto(ctx context.Context, orgPK, target string, since time.Time, reason string) (bool, error)
}

// PrecheckAutoContingency returns the configuration row to emit with: the same row, or a copy already in the
// automatic contingency when the breaker is open. model is ModelNFe/ModelNFCe/ModelCTe/ModelMDFe; uf is the
// issuer's UF (only the SVC targets need it).
func PrecheckAutoContingency(ctx context.Context, repo AutoSwitcher, model, orgPK, uf string, cfg map[string]types.AttributeValue, now time.Time) map[string]types.AttributeValue {
	if !autoSwitchEnabled || len(cfg) == 0 {
		return cfg
	}
	auto, _ := cfg[repositories.AttrAutoContingency].(*types.AttributeValueMemberBOOL)
	acfg := autocontingency.Config{
		Enabled: true, Auto: auto != nil && auto.Value, Type: attrStrAV(cfg, repositories.AttrAutoContingencyType),
		Options: emissionOptions[model], Current: repositories.EmissionTypeOf(cfg),
	}
	target, ok := autocontingency.ShouldSwitch(acfg, autocontingency.Decode(attrStrAV(cfg, repositories.AttrAutoBreaker)))
	if !ok || !targetUsable(model, target, uf) {
		return cfg
	}
	since := now.UTC().Truncate(time.Second)
	switched, err := repo.SwitchAuto(ctx, orgPK, target, since, autocontingency.AutoReason)
	if err != nil {
		slog.WarnContext(ctx, "automatic contingency switch failed; emitting normally", "org_pk", orgPK, "model", model, "err", err)
		return cfg
	}
	if !switched { // another emission switched first, or the owner changed the row: not this call's decision
		return cfg
	}
	slog.InfoContext(ctx, "automatic contingency: emission type switched", "org_pk", orgPK, "model", model, "emission_type", target)
	out := maps.Clone(cfg)
	out[repositories.AttrEmissionType] = &types.AttributeValueMemberS{Value: target}
	out[repositories.AttrContingencySince] = &types.AttributeValueMemberS{Value: since.Format(time.RFC3339)}
	out[repositories.AttrContingencyReason] = &types.AttributeValueMemberS{Value: autocontingency.AutoReason}
	out[repositories.AttrContingencyAuto] = &types.AttributeValueMemberBOOL{Value: true}
	return out
}

// targetUsable: the SVC needs a designated SVC for the issuer's UF (the resolvers refuse otherwise, and a switch
// that makes every emission fail is worse than emitting normally).
func targetUsable(model, target, uf string) bool {
	if target != repositories.EmissionSVC {
		return true
	}
	if model == ModelCTe {
		_, ok := dfe.SVCForCTe(uf)
		return ok
	}
	_, ok := dfe.SVCFor(uf)
	return ok
}
