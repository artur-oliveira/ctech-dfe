package ciot

import "strconv"

// Rejection groups (spec section 8).
const (
	GroupValidation  = "VALIDATION"
	GroupCarrier     = "CARRIER"
	GroupDates       = "DATES"
	GroupState       = "STATE"
	GroupDeadline    = "DEADLINE"
	GroupPayment     = "PAYMENT"
	GroupCargoRoute  = "CARGO_ROUTE"
	GroupFloor       = "FLOOR"
	GroupCredential  = "CREDENTIAL"
	GroupService     = "SERVICE"
	GroupUnknown     = "UNKNOWN"
	internalPrefix   = "CIOT_"
	internalUnmapped = internalPrefix + "UNMAPPED"
)

// RejectionInfo maps an ANTT code to a stable internal code. Rule is the PDF rule id(s); mappings between a code
// and a rule inferred from the position in the spec table are marked Inferred (confirm in homologation).
type RejectionInfo struct {
	Code     int
	Rule     string
	Group    string
	Internal string
	Inferred bool
}

func rej(code int, rule, group string, inferred bool) RejectionInfo {
	return RejectionInfo{code, rule, group, internalPrefix + group + "_" + strconv.Itoa(code), inferred}
}

// rejectionTable is the data of spec section 8.
var rejectionTable = []RejectionInfo{
	rej(229, "B1", GroupValidation, false), rej(205, "B4,B60", GroupValidation, false),
	rej(207, "B3", GroupCarrier, false), rej(216, "B14", GroupCarrier, false), rej(217, "B15", GroupCarrier, false),
	rej(228, "B20", GroupCarrier, false), rej(307, "B107", GroupCarrier, false), rej(316, "B117", GroupCarrier, false),
	rej(208, "B6", GroupDates, true), rej(225, "B12", GroupDates, true), rej(270, "B13", GroupDates, true),
	rej(234, "B21", GroupDates, true), rej(271, "B22", GroupDates, true), rej(214, "B96", GroupDates, true),
	rej(227, "B19", GroupDates, true), rej(269, "B11", GroupDates, true), rej(250, "B30", GroupDates, true),
	rej(CodeDuplicateID, "B17", GroupState, false), rej(244, "B24-B33", GroupState, false),
	rej(245, "B24-B33", GroupState, false), rej(247, "B24-B33", GroupState, false),
	rej(264, "B24-B33", GroupState, false), rej(265, "B24-B33", GroupState, false),
	rej(266, "B24-B33", GroupState, false), rej(220, "B34", GroupState, false),
	rej(222, "B37", GroupState, false), rej(251, "B38,B86", GroupState, false),
	rej(221, "B35", GroupDeadline, false), rej(235, "B36", GroupDeadline, true), rej(237, "B40", GroupDeadline, true),
	rej(236, "B39", GroupDeadline, false), rej(320, "B121", GroupDeadline, false),
	rej(233, "B92", GroupPayment, false), rej(224, "B99", GroupPayment, false), rej(204, "B100", GroupPayment, false),
	rej(252, "B103,B104", GroupPayment, true), rej(215, "B105,B106", GroupPayment, true), rej(317, "B118", GroupPayment, false),
	rej(211, "B9", GroupCargoRoute, false), rej(226, "B18", GroupCargoRoute, false), rej(292, "B82", GroupCargoRoute, false),
	rej(308, "B109", GroupCargoRoute, true), rej(309, "B110", GroupCargoRoute, true), rej(310, "B111", GroupCargoRoute, true),
	rej(318, "B119", GroupCargoRoute, false),
	rej(291, "B80", GroupFloor, false), rej(319, "B120", GroupFloor, false), rej(313, "B114", GroupFloor, false),
	rej(311, "B112", GroupCredential, true), rej(312, "B113", GroupCredential, true), rej(314, "B115", GroupCredential, true),
	rej(218, "B42", GroupCredential, false), rej(258, "B71", GroupCredential, false), rej(321, "B122", GroupCredential, false),
	rej(203, "B2", GroupService, false),
}

var rejectionByCode = func() map[int]RejectionInfo {
	m := make(map[int]RejectionInfo, len(rejectionTable))
	for _, r := range rejectionTable {
		m[r.Code] = r
	}
	return m
}()

// LookupRejection returns the stable mapping; an unknown code maps to CIOT_UNMAPPED (still carrying the ANTT code).
func LookupRejection(code int) RejectionInfo {
	if r, ok := rejectionByCode[code]; ok {
		return r
	}
	return RejectionInfo{Code: code, Group: GroupUnknown, Internal: internalUnmapped}
}

// IsCredentialProblem reports a certificate/credential configuration error (alert, not a business error).
func (r RejectionInfo) IsCredentialProblem() bool { return r.Group == GroupCredential }

// newRejection builds the error for a non-success Result.
func newRejection(res Result) *Rejection {
	info := LookupRejection(res.Codigo)
	return &Rejection{Result: res, Internal: info.Internal, Group: info.Group}
}
