package v1

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/validation"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// PaginatedResponse is the standard envelope for list endpoints.
type PaginatedResponse struct {
	Items          any     `json:"items"`
	NextCursor     *string `json:"next_cursor"`
	HasNext        bool    `json:"has_next"`
	PreviousCursor *string `json:"previous_cursor"`
	HasPrevious    bool    `json:"has_previous"`
}

// cursorPayload is the JSON structure embedded in every base64 cursor.
// k holds the DynamoDB ExclusiveStartKey serialized as a plain Go map (via
// attributevalue.UnmarshalMap) so that standard JSON round-trips preserve N/S types.
// p holds only the single previous page's key (not the previous cursor
// string) — embedding the whole prior cursor would nest it inside every
// subsequent one, recompounding through base64+JSON on every page and
// eventually exceeding request header size limits (431).
type cursorPayload struct {
	Key  map[string]any `json:"k"`
	Prev map[string]any `json:"p,omitempty"`
}

// sendProblem writes a RFC 7807 Problem response. Detects *problem.Problem for
// the correct status code; all other errors become 500.
func sendProblem(c fiber.Ctx, err error) error {
	if p, ok := errors.AsType[*problem.Problem](err); ok {
		return p.Send(c)
	}
	slog.ErrorContext(c.Context(), "unhandled error", "path", c.Path(), "err", err)
	return problem.InternalServer(err.Error()).Send(c)
}

// currentAccessToken extracts the raw Bearer token from the Authorization header.
func currentAccessToken(c fiber.Ctx) string {
	return strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
}

// resolveActor extracts the caller's user_id (from auth middleware locals) and
// resolves their display name for audit attribution. Every mutating route that
// needs to attribute a change calls this once, right after request validation.
func resolveActor(c fiber.Ctx, userSvc *services.UserService) (userID, userName string) {
	userID = middleware.GetUserID(c)
	accessToken := currentAccessToken(c)
	_, userName = userSvc.ResolveActor(c.Context(), userID, accessToken)
	return userID, userName
}

// readOptionalUpload reads the full bytes of a multipart file field. Returns
// (nil, nil) when the field is absent — callers decide whether that's an error.
func readOptionalUpload(c fiber.Ctx, field string) ([]byte, error) {
	file, err := c.FormFile(field)
	if err != nil {
		return nil, nil // field not present
	}
	f, err := file.Open()
	if err != nil {
		return nil, problem.BadRequest("não foi possível abrir o arquivo enviado")
	}
	defer func() { _ = f.Close() }()
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, problem.BadRequest("não foi possível ler o arquivo enviado")
	}
	return buf, nil
}

// attrStr extracts a string attribute from a DynamoDB item, or "" if absent.
func attrStr(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}

// unmarshal converts a DynamoDB attribute map to map[string]any for JSON response.
func unmarshal(item map[string]types.AttributeValue) (map[string]any, error) {
	var out map[string]any
	return out, attributevalue.UnmarshalMap(item, &out)
}

// extractCrt reads the nested person.crt out of an already-unmarshalled
// person/organization item, for backfilling a partial update body (when the
// caller omits crt) before re-validating RequirePJFields.
func extractCrt(item map[string]any) *int {
	personRaw, _ := item["person"].(map[string]any)
	if v, ok := personRaw["crt"]; ok && v != nil {
		if n, ok := v.(float64); ok {
			return new(int(n))
		}
	}
	return nil
}

// unmarshalList converts a slice of DynamoDB items to []map[string]any.
func unmarshalList(items []map[string]types.AttributeValue) ([]map[string]any, error) {
	out := make([]map[string]any, len(items))
	for i, item := range items {
		m, err := unmarshal(item)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

// buildNextCursor encodes a DynamoDB LastEvaluatedKey as the next-page cursor.
// incomingCursor is the cursor the client sent for THIS request; it becomes the
// "prev" pointer embedded in the new cursor so the caller can navigate backwards.
// Returns nil when key is empty (no next page).
func buildNextCursor(key map[string]types.AttributeValue, incomingCursor string) *string {
	if len(key) == 0 {
		return nil
	}
	// Convert DynamoDB attribute values to a plain Go map so the cursor round-trips
	// correctly through standard JSON (types.AttributeValue is a sealed interface and
	// cannot be unmarshaled via encoding/json).
	var plainKey map[string]any
	if err := attributevalue.UnmarshalMap(key, &plainKey); err != nil {
		return nil
	}
	raw, err := json.Marshal(cursorPayload{Key: plainKey, Prev: decodeCursorKey(incomingCursor)})
	if err != nil {
		return nil
	}
	return new(base64.StdEncoding.EncodeToString(raw))
}

// decodeCursorKey extracts a cursor's own plain-map key (the "k" field, not
// converted to DynamoDB attribute values) — used to seed the next cursor's
// "p" pointer with a single bounded key instead of the whole cursor string.
func decodeCursorKey(cursor string) map[string]any {
	if cursor == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload.Key
}

// decodeCursor extracts the DynamoDB ExclusiveStartKey from a cursor string.
// Cursors are encoded as base64(JSON{k: plainGoMap, p: prevCursor}).
// The plain Go map is converted back to DynamoDB attribute values via MarshalMap.
func decodeCursor(cursor string) map[string]types.AttributeValue {
	if cursor == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Key) == 0 {
		return nil
	}
	avKey, err := attributevalue.MarshalMap(payload.Key)
	if err != nil {
		return nil
	}
	return avKey
}

// prevCursorOf builds the cursor for the page before the one incomingCursor
// fetched, from the single previous key embedded in incomingCursor's "p"
// field. Returns nil when the cursor is empty, malformed, or carries no
// previous key (e.g. it points at the first page).
func prevCursorOf(cursor string) *string {
	if cursor == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Prev) == 0 {
		return nil
	}
	out, err := json.Marshal(cursorPayload{Key: payload.Prev})
	if err != nil {
		return nil
	}
	return new(base64.StdEncoding.EncodeToString(out))
}

// intQuery reads a query param as int; returns def on missing/invalid.
func intQuery(c fiber.Ctx, key string, def int) int {
	s := c.Query(key)
	if s == "" {
		return def
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return def
	}
	return v
}

// ptrIntQuery reads a query param as *int; returns nil on missing.
func ptrIntQuery(c fiber.Ctx, key string) *int {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return nil
	}
	return &v
}

// ptrQuery reads a query param as *string; returns nil on missing.
func ptrQuery(c fiber.Ctx, key string) *string {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	return &s
}

// safeFilename keeps only characters valid in a quoted Content-Disposition
// filename. Route params reach here URL-decoded, so a raw quote or CRLF would
// escape the header value.
func safeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		}
		return -1
	}, s)
}

// sendAttachment replies with data as a download named filename.ext.
func sendAttachment(c fiber.Ctx, data []byte, contentType, filename, ext string) error {
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+safeFilename(filename)+ext+`"`)
	return c.Send(data)
}

// sendXML replies with an XML attachment named filename.xml.
func sendXML(c fiber.Ctx, data []byte, filename string) error {
	return sendAttachment(c, data, fiber.MIMEApplicationXML, filename, ".xml")
}

// bindJSON strictly decodes the JSON request body into dst, rejecting unknown
// fields, then runs struct validation. Returns nil on success, or a
// *problem.Problem (400 for malformed JSON, 422 for validation failures) ready
// to be passed to sendProblem. This is the single entry point for validated
// request binding — no route should call c.Bind().JSON directly for typed bodies.
func bindJSON[T any](c fiber.Ctx, dst *T) *problem.Problem {
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return problem.BadRequest("corpo JSON inválido: " + err.Error())
	}
	return validation.Struct(dst)
}

// bindAVValidated strictly binds and validates the request body into a DTO of
// type T, then converts it to a DynamoDB attribute map. Returns the AV map, or a
// *problem.Problem (400/422) on malformed or invalid input.
func bindAVValidated[T any](c fiber.Ctx) (map[string]types.AttributeValue, *problem.Problem) {
	var dto T
	if p := bindJSON(c, &dto); p != nil {
		return nil, p
	}
	av, err := structToAV(dto)
	if err != nil {
		return nil, problem.InternalServer(err.Error())
	}
	return av, nil
}

// structToMap converts a validated DTO into a plain map keyed by JSON tags, so
// it can flow through the existing service/repository layers (which persist
// dynamic maps) without changing storage keys.
func structToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// structToAV converts a validated DTO into a DynamoDB attribute map (nulls
// omitted), preserving the JSON snake_case keys used throughout storage.
func structToAV(v any) (map[string]types.AttributeValue, error) {
	m, err := structToMap(v)
	if err != nil {
		return nil, err
	}
	return repositories.MarshalMapOmitNull(m)
}

// sendItem unmarshals a DynamoDB item and writes it as a JSON response.
func sendItem(c fiber.Ctx, item map[string]types.AttributeValue) error {
	m, err := unmarshal(item)
	if err != nil {
		return sendProblem(c, err)
	}
	return c.JSON(m)
}

// sendCreated unmarshals a DynamoDB item and writes it as a 201 Created
// response. Collapses the repeated `unmarshal + c.Status(201).JSON` tail
// shared by every resource-creation handler.
func sendCreated(c fiber.Ctx, item map[string]types.AttributeValue) error {
	m, err := unmarshal(item)
	if err != nil {
		return sendProblem(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(m)
}

// sendPage unmarshals a QueryResult's items and writes them as a paginated JSON response.
// incomingCursor is the raw cursor string the client sent for this request; it is
// embedded in the next cursor and surfaced as previous_cursor for backward navigation.
func sendPage(c fiber.Ctx, result *repositories.QueryResult, incomingCursor string) error {
	items, err := unmarshalList(result.Items)
	if err != nil {
		return sendProblem(c, err)
	}
	prevCursor := prevCursorOf(incomingCursor)
	return c.JSON(PaginatedResponse{
		Items:          items,
		NextCursor:     buildNextCursor(result.LastEvaluatedKey, incomingCursor),
		HasNext:        len(result.LastEvaluatedKey) > 0,
		PreviousCursor: prevCursor,
		HasPrevious:    incomingCursor != "",
	})
}

// fiscalConfigSvc is the common interface implemented by Nfe/Nfce/Cte/MdfeConfigService.
type fiscalConfigSvc interface {
	Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error)
	Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error)
}

// registerFiscalConfig mounts GET and PUT handlers for a fiscal config sub-resource
// (e.g. /nfe-config, /nfce-config) under an already-authed scoped router. bind is
// the variant-specific validated binder (FiscalConfigBody or NfceConfigBody).
// userSvc resolves the caller's identity for audit attribution on PUT.
func registerFiscalConfig(scoped fiber.Router, path, getPerm, putPerm string, svc fiscalConfigSvc, perm *middleware.PermChecker,
	bind func(fiber.Ctx) (map[string]types.AttributeValue, *problem.Problem), userSvc *services.UserService) {
	scoped.Get(path, perm.Require(getPerm), func(c fiber.Ctx) error {
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c))
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})
	scoped.Put(path, perm.Require(putPerm), func(c fiber.Ctx) error {
		av, p := bind(c)
		if p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Upsert(c.Context(), middleware.GetOrgPK(c), av, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})
}
