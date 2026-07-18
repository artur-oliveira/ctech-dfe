package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// DocumentEventRepository stores SEFAZ communication events for all doc types.
//
// Table structure (nfe_events, nfce_events, etc.):
//
//	pk        = access_key (44-digit chave de acesso)
//	sk        = {uuid}
//	event_key = {access_key}#{event_type}#{sequence:03d}
type DocumentEventRepository struct {
	Base
}

func NewDocumentEventRepository(db *dynamodb.Client, cfg *config.Config, docType string) *DocumentEventRepository {
	return &DocumentEventRepository{Base: NewBase(db, cfg, fmt.Sprintf("%s_events", docType))}
}

func (r *DocumentEventRepository) CreateEvent(
	ctx context.Context,
	accessKey, eventType string,
	sequenceNumber int,
	status string,
	sefazStatus, sefazMotive, xmlS3Key *string,
	userID, userName string,
) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	now := NowStr()

	item := map[string]types.AttributeValue{
		"pk":              &types.AttributeValueMemberS{Value: accessKey},
		"sk":              &types.AttributeValueMemberS{Value: id},
		"access_key":      &types.AttributeValueMemberS{Value: accessKey},
		"event_key":       &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%03d", accessKey, eventType, sequenceNumber)},
		"event_type":      &types.AttributeValueMemberS{Value: eventType},
		"sequence_number": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", sequenceNumber)},
		"status":          &types.AttributeValueMemberS{Value: status},
		"user_id":         &types.AttributeValueMemberS{Value: userID},
		"user_name":       &types.AttributeValueMemberS{Value: userName},
		"created_at":      &types.AttributeValueMemberS{Value: now},
		"updated_at":      &types.AttributeValueMemberS{Value: now},
	}
	setNullableStr(item, "sefaz_status", sefazStatus)
	setNullableStr(item, "sefaz_motive", sefazMotive)
	setNullableStr(item, "xml_s3_key", xmlS3Key)

	return item, r.PutItem(ctx, item)
}

func (r *DocumentEventRepository) UpdateEvent(ctx context.Context, accessKey, sk string, updates map[string]any) error {
	updates["updated_at"] = NowStr()
	_, err := r.UpdateItem(ctx, accessKey, &sk, updates)
	return err
}

// GetDocumentEvents lists all events for a document by access_key (pk).
func (r *DocumentEventRepository) GetDocumentEvents(ctx context.Context, accessKey string, limit int, startKey map[string]types.AttributeValue) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{
		PK: accessKey, Limit: limit, ExclusiveStartKey: startKey,
	})
}

func (r *DocumentEventRepository) GetEvent(ctx context.Context, accessKey, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, accessKey, sk)
}

func setNullableStr(item map[string]types.AttributeValue, key string, val *string) {
	if val != nil {
		item[key] = &types.AttributeValueMemberS{Value: *val}
	}
	// nil → omit the attribute (no NULL stored)
}
