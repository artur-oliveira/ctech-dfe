package services

import (
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Doc-type and vehicle-role constants for the required-fields matrix.
const (
	DocTypeMdfe  = "mdfe"
	DocTypeNfe   = "nfe"
	DocTypeCteOS = "cte_os"

	VehicleRoleTractor = "tractor"
	VehicleRoleTrailer = "trailer"
)

// vehicleRequirementFields mirrors the subset of organization_vehicles
// attributes the required-fields matrix inspects.
type vehicleRequirementFields struct {
	Weight   int    `dynamodbav:"weight"`
	Wheelset string `dynamodbav:"wheelset"`
	Bodywork string `dynamodbav:"bodywork"`
	CapKG    int    `dynamodbav:"cap_kg"`
}

// Missing is the single source of truth for which vehicle fields a given
// doc-type + role combination requires beyond plate/plate_uf (always
// required at cadastro). Returns the JSON field names still missing, in a
// fixed order; empty when the vehicle is ready for that doc-type+role.
//
// NF-e and CT-e OS never require anything beyond plate per their XSDs
// (veicTransp/veic have no other required fields) — their rows exist so the
// matrix is ready when CT-e OS emission is built.
func Missing(item map[string]types.AttributeValue, docType, role string) []string {
	var v vehicleRequirementFields
	if err := attributevalue.UnmarshalMap(item, &v); err != nil {
		slog.Warn("vehicle requirement fields decode failed", "err", err)
	}

	var missing []string
	if docType != DocTypeMdfe {
		return missing
	}
	if v.Weight == 0 {
		missing = append(missing, "weight")
	}
	switch role {
	case VehicleRoleTractor:
		if v.Wheelset == "" {
			missing = append(missing, "wheelset")
		}
		if v.Bodywork == "" {
			missing = append(missing, "bodywork")
		}
	case VehicleRoleTrailer:
		if v.CapKG == 0 {
			missing = append(missing, "cap_kg")
		}
		if v.Bodywork == "" {
			missing = append(missing, "bodywork")
		}
	}
	return missing
}
