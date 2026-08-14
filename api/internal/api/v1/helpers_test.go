package v1

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// sampleLastEvaluatedKey returns a fixed-shape DynamoDB key. n only varies
// the "sk" suffix (same digit width throughout) so payload size never shifts
// for reasons unrelated to what's under test.
func sampleLastEvaluatedKey(n int) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "hom#CNPJ_11647612000197"},
		"sk": &types.AttributeValueMemberS{Value: fmt.Sprintf("sk-value-%03d", n%1000)},
	}
}

// TestBuildNextCursor_ChainedPages_StaysBounded regression-tests the bug
// where each cursor embedded the ENTIRE previous cursor string (not just its
// key), so cursor size recompounded ~4/3x per page and eventually exceeded
// request header size limits (431 Request Header Fields Too Large) after a
// handful of pages. Cursor size must stay roughly constant across pages.
func TestBuildNextCursor_ChainedPages_StaysBounded(t *testing.T) {
	cursor := ""
	// steadyLen tracks cursor length once every cursor carries a "p" key
	// (from page 2 onward) — the old design recompounded ~4/3x per hop here
	// (thousands of bytes by page 30); the fix keeps it exactly flat.
	var steadyLen int
	for i := 0; i < 30; i++ {
		next := buildNextCursor(sampleLastEvaluatedKey(i), cursor)
		if next == nil {
			t.Fatalf("page %d: expected non-nil cursor", i)
		}
		cursor = *next
		if i == 1 {
			steadyLen = len(cursor)
		} else if i > 1 && len(cursor) != steadyLen {
			t.Fatalf("page %d: cursor length drifted from steady state %d to %d", i, steadyLen, len(cursor))
		}
	}
}

func TestPrevCursorOf_RoundTripsToPriorPageKey(t *testing.T) {
	cursor1 := buildNextCursor(sampleLastEvaluatedKey(1), "")
	if cursor1 == nil {
		t.Fatal("expected non-nil cursor1")
	}
	cursor2 := buildNextCursor(sampleLastEvaluatedKey(2), *cursor1)
	if cursor2 == nil {
		t.Fatal("expected non-nil cursor2")
	}

	prev := prevCursorOf(*cursor2)
	if prev == nil {
		t.Fatal("expected non-nil previous cursor")
	}
	// The reconstructed previous cursor must decode to the same DynamoDB key
	// cursor1 carries, so resending it reproduces the page cursor1 fetched.
	wantKey := decodeCursor(*cursor1)
	gotKey := decodeCursor(*prev)
	if len(gotKey) != len(wantKey) {
		t.Fatalf("previous cursor key mismatch: got %v, want %v", gotKey, wantKey)
	}
}

func TestPrevCursorOf_FirstPage_ReturnsNil(t *testing.T) {
	cursor1 := buildNextCursor(sampleLastEvaluatedKey(1), "")
	if cursor1 == nil {
		t.Fatal("expected non-nil cursor1")
	}
	if got := prevCursorOf(*cursor1); got != nil {
		t.Fatalf("expected nil (no page before the first), got %v", *got)
	}
}

func TestPrevCursorOf_EmptyOrMalformed_ReturnsNil(t *testing.T) {
	if got := prevCursorOf(""); got != nil {
		t.Fatalf("expected nil for empty cursor, got %v", *got)
	}
	if got := prevCursorOf("not-valid-base64!!"); got != nil {
		t.Fatalf("expected nil for malformed cursor, got %v", *got)
	}
}

func TestExtractCrt_ReadsNestedPersonField(t *testing.T) {
	item := map[string]any{
		"person": map[string]any{
			"crt": float64(3),
		},
	}
	crt := extractCrt(item)
	if crt == nil || *crt != 3 {
		t.Fatalf("crt = %v, want 3", crt)
	}
}

func TestExtractCrt_MissingPersonReturnsNil(t *testing.T) {
	if crt := extractCrt(map[string]any{}); crt != nil {
		t.Errorf("crt = %v, want nil", crt)
	}
}
