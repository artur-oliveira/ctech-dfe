package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// exportRepo is the slice of repositories.ErasureRepository the export reads.
type exportRepo interface {
	GetItem(ctx context.Context, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error)
	QueryIndex(ctx context.Context, table, index, attr, value string, start map[string]types.AttributeValue) (repositories.Page, error)
}

// exportMaxItems bounds one file; the read stops paging at it.
// ponytail: 50k rows covers any person's audit trail; page to S3 if one outgrows memory.
const exportMaxItems = 50_000

// DataExporter builds dfe's part of the personal-data export (LGPD art. 18, V;
// ctech-account docs/specs/2026-10-17-data-export.md): what dfe holds about the
// person as a data subject, not what their companies hold.
//
//   - users: the person's own row;
//   - organization_users: their memberships (which companies, with which role);
//   - account_billing: their plan snapshot (legacy USER_ rows);
//   - audit_logs: what they did, across companies, as the actor.
//
// Not included: the fiscal documents, certificates and configuration of any
// company. They are the company's (the company already exports its XMLs), and
// an e-CPF certificate is a credential. exportInventory classifies every table
// of the erasure catalog; TestEveryTableIsClassifiedForTheExport makes a new
// table choose.
type DataExporter struct {
	repo exportRepo
	now  func() time.Time
}

// DataExporter is the export side of the same repository the purge uses.
func (s *ErasureService) DataExporter() *DataExporter {
	return &DataExporter{repo: s.repo, now: time.Now}
}

// exportInventory says, per table of erasureCatalog, whether it is exported
// ("exported: ...") or not ("excluded: reason").
var exportInventory = map[string]string{
	"users":              "exported: the person's own row",
	"organization_users": "exported: the person's memberships, found through user-index",
	"account_billing":    "exported: the person's plan snapshot (USER_ key)",
	"audit_logs":         "exported: the rows the person wrote as actor, found through user-id-index; the payload (modifications) is the company's and is left out",
}

const exportedByCompany = "excluded: the company's data (the company exports its own documents)"

func exportClassification(table string) string {
	if v, ok := exportInventory[table]; ok {
		return v
	}
	switch table {
	case "roles", "erasure_state", "serie_claims":
		return "excluded: global, derived or the saga's own state; no data of the person"
	case "organization_invitations":
		return "excluded: reachable only per organization, and they hold other people's e-mail addresses"
	case "organization_certificates":
		return "excluded: certificates are credentials of the company; e-CPF files are not exported"
	}
	return exportedByCompany
}

// Part is the dataexport.PartFunc of dfe. Idempotent.
func (e *DataExporter) Part(ctx context.Context, sub string) ([]byte, error) {
	raw := repositories.RawUserID(sub)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var files, truncated []string
	write := func(name string, rows []map[string]any, cut bool) error {
		if len(rows) == 0 {
			return nil
		}
		data, err := json.MarshalIndent(map[string]any{"truncated": cut, "items": rows}, "", "  ")
		if err != nil {
			return err
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		files = append(files, name)
		if cut {
			truncated = append(truncated, name)
		}
		return nil
	}

	var rows []map[string]any
	for _, one := range []struct{ file, table, pk string }{
		{"user.json", tblUsers, repositories.BuildUserPK(raw)},
		{"billing.json", tblAccountBilling, repositories.AccountBillingPK(raw)},
	} {
		item, err := e.repo.GetItem(ctx, one.table, pkKey(one.pk))
		if err != nil {
			return nil, fmt.Errorf("export: %s: %w", one.table, err)
		}
		if item == nil {
			continue
		}
		m, err := plain(item)
		if err != nil {
			return nil, err
		}
		if err := write(one.file, []map[string]any{m}, false); err != nil {
			return nil, err
		}
	}
	for _, idx := range []struct{ file, table, index, attr, value string }{
		{"memberships.json", tblOrgUsers, userIndex, skKey, repositories.BuildMemberSK(raw)},
		{"audit-logs.json", tblAuditLogs, userIDIndex, "user_id", raw},
	} {
		var cut bool
		var err error
		if rows, cut, err = e.readIndex(ctx, idx.table, idx.index, idx.attr, idx.value, idx.table == tblAuditLogs); err != nil {
			return nil, err
		}
		if err := write(idx.file, rows, cut); err != nil {
			return nil, err
		}
	}

	w, err := zw.Create("README.txt")
	if err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte(e.readme(files, truncated))); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readIndex pages an index until exportMaxItems, so the cap bounds memory.
func (e *DataExporter) readIndex(ctx context.Context, table, index, attr, value string, dropPayload bool) ([]map[string]any, bool, error) {
	var out []map[string]any
	var start map[string]types.AttributeValue
	for {
		page, err := e.repo.QueryIndex(ctx, table, index, attr, value, start)
		if err != nil {
			return nil, false, fmt.Errorf("export: %s: %w", table, err)
		}
		for _, it := range page.Items {
			if len(out) >= exportMaxItems {
				return out, true, nil
			}
			m, err := plain(it)
			if err != nil {
				return nil, false, err
			}
			if dropPayload {
				delete(m, "modifications")
			}
			out = append(out, m)
		}
		if page.Next == nil {
			return out, false, nil
		}
		start = page.Next
	}
}

var secretFragments = []string{"password", "secret", "token", "cpf", "pfx", "private"}

func plain(item map[string]types.AttributeValue) (map[string]any, error) {
	var m map[string]any
	if err := attributevalue.UnmarshalMap(item, &m); err != nil {
		return nil, fmt.Errorf("export: decode: %w", err)
	}
	return scrubSecrets(m).(map[string]any), nil
}

func scrubSecrets(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			drop := false
			for _, f := range secretFragments {
				if strings.Contains(strings.ToLower(k), f) {
					drop = true
					break
				}
			}
			if !drop {
				out[k] = scrubSecrets(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = scrubSecrets(e)
		}
		return out
	}
	return v
}

func (e *DataExporter) readme(files, truncated []string) string {
	var b strings.Builder
	b.WriteString("CTech DF-e — a copy of your personal data\nGenerated at " + e.now().UTC().Format(time.RFC3339) + "\n\nFiles:\n")
	for _, f := range files {
		b.WriteString("  " + f + "\n")
	}
	b.WriteString("\nuser.json: your user record. memberships.json: the companies you belong to and your role. billing.json: your plan snapshot.\n")
	b.WriteString("audit-logs.json: what you did in DF-e, across companies (the details of each change belong to the company and are left out).\n")
	b.WriteString("Not included: the fiscal documents, certificates and settings of your companies (they belong to the company, which can export its XML files), and certificate files.\n")
	if len(truncated) > 0 {
		b.WriteString("\nThese files were cut at the size limit: " + strings.Join(truncated, ", ") + "\n")
	}
	return b.String()
}
