package services

import (
	"archive/zip"
	"cmp"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dataexport"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// exportRepo is the slice of repositories.ErasureRepository the export reads.
type exportRepo interface {
	GetItem(ctx context.Context, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error)
	QueryIndex(ctx context.Context, table, index, attr, value string, start map[string]types.AttributeValue) (repositories.Page, error)
}

// exportMaxItems bounds one file; the read stops paging at it.
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
	// maxItems overrides exportMaxItems (tests).
	maxItems int
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

// exportFiles is what one export wrote, for the README.
type exportFiles struct {
	zw               *zip.Writer
	files, truncated []string
}

// one writes a file holding a single row.
func (f *exportFiles) one(name string, row map[string]any) error {
	it, err := dataexport.BeginItems(f.zw, name)
	if err != nil {
		return err
	}
	f.files = append(f.files, name)
	if err := it.Add(row); err != nil {
		return err
	}
	return it.End(false)
}

// Write is the dataexport.PartStreamFunc of dfe: it writes the zip to w as it
// reads, a page of an index at a time, so a person with a long audit trail costs
// one page of memory. Idempotent.
func (e *DataExporter) Write(ctx context.Context, sub string, w io.Writer) error {
	raw := repositories.RawUserID(sub)
	f := &exportFiles{zw: zip.NewWriter(w)}

	for _, one := range []struct{ file, table, pk string }{
		{"user.json", tblUsers, repositories.BuildUserPK(raw)},
		{"billing.json", tblAccountBilling, repositories.AccountBillingPK(raw)},
	} {
		item, err := e.repo.GetItem(ctx, one.table, pkKey(one.pk))
		if err != nil {
			return fmt.Errorf("export: %s: %w", one.table, err)
		}
		if item == nil {
			continue
		}
		m, err := plain(item)
		if err != nil {
			return err
		}
		if err := f.one(one.file, m); err != nil {
			return err
		}
	}
	for _, idx := range []struct{ file, table, index, attr, value string }{
		{"memberships.json", tblOrgUsers, userIndex, skKey, repositories.BuildMemberSK(raw)},
		{"audit-logs.json", tblAuditLogs, userIDIndex, "user_id", raw},
	} {
		if err := e.writeIndex(ctx, f, idx.file, idx.table, idx.index, idx.attr, idx.value, idx.table == tblAuditLogs); err != nil {
			return err
		}
	}

	rw, err := f.zw.Create("README.txt")
	if err != nil {
		return err
	}
	if _, err := rw.Write([]byte(e.readme(f.files, f.truncated))); err != nil {
		return err
	}
	return f.zw.Close()
}

// writeIndex streams an index into one file, a page at a time, up to the cap; a
// file with no rows is not written.
func (e *DataExporter) writeIndex(ctx context.Context, f *exportFiles, name, table, index, attr, value string, dropPayload bool) error {
	var it *dataexport.Items
	cut := false
	var start map[string]types.AttributeValue
paging:
	for {
		page, err := e.repo.QueryIndex(ctx, table, index, attr, value, start)
		if err != nil {
			return fmt.Errorf("export: %s: %w", table, err)
		}
		for _, row := range page.Items {
			if it != nil && it.Count() >= cmp.Or(e.maxItems, exportMaxItems) {
				cut = true
				break paging
			}
			m, err := plain(row)
			if err != nil {
				return err
			}
			if dropPayload {
				delete(m, "modifications")
			}
			if it == nil {
				if it, err = dataexport.BeginItems(f.zw, name); err != nil {
					return err
				}
				f.files = append(f.files, name)
			}
			if err := it.Add(m); err != nil {
				return err
			}
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	if it == nil {
		return nil
	}
	if cut {
		f.truncated = append(f.truncated, name)
	}
	return it.End(cut)
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
