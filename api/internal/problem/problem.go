// Package problem implements RFC 7807 Problem Details, mirroring the Python
// ProblemException hierarchy in api/app/core/exceptions.py.
package problem

import (
	"encoding/json"
	"net/http"

	"github.com/gofiber/fiber/v3"

	commonproblem "gopkg.aoctech.app/api-commons/problem"
)

const ContentType = "application/problem+json"

// Problem type URIs (RFC 7807 "type" member). Defined as constants so they are
// never scattered as raw string literals across the codebase.
const (
	TypeBadRequest      = "/problems/bad-request"
	TypeNoCertificate   = "/problems/no-certificate"
	TypeSefazRejection  = "/problems/sefaz-rejection"
	TypeUnauthorized    = "/problems/unauthorized"
	TypeForbidden       = "/problems/forbidden"
	TypeNotFound        = "/problems/not-found"
	TypeConflict        = "/problems/conflict"
	TypeValidation      = "/problems/validation-error"
	TypeTooManyRequests = "/problems/too-many-requests"
	TypeInternalServer  = "/problems/internal-server-error"
	TypeNotImplemented  = "/problems/not-implemented"
	TypePayloadTooLarge = "/problems/payload-too-large"
)

// FieldError is a single field-level validation failure. It mirrors the shape
// the frontend Zod layer produces so the UI can map each error back to its input.
type FieldError = commonproblem.FieldError

// Problem is the RFC 7807 response body. Errors carries field-level validation
// failures (only populated for validation problems; omitted otherwise).
type Problem struct {
	commonproblem.Problem
}

// Error implements the error interface so problems can be returned as errors.
// This overrides the embedded commonproblem.Problem.Error() to preserve dfe's
// existing semantics (Detail alone, not "Title: Detail").
func (p *Problem) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

func (p *Problem) Send(c fiber.Ctx) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	c.Status(p.Status)
	c.Set(fiber.HeaderContentType, ContentType)
	return c.Send(b)
}

func wrap(p *commonproblem.Problem) *Problem { return &Problem{Problem: *p} }

func New(status int, typ, title, detail string) *Problem {
	return wrap(commonproblem.New(status, typ, title, detail))
}

func BadRequest(detail string) *Problem {
	return wrap(commonproblem.BadRequest(detail))
}

func NoCertificate(detail string) *Problem {
	return wrap(commonproblem.New(http.StatusBadRequest, TypeNoCertificate, "No Certificate Found", detail))
}

func SefazRejection(detail string) *Problem {
	return wrap(commonproblem.New(http.StatusBadRequest, TypeSefazRejection, "Sefaz Rejection", detail))
}

func Unauthorized(detail string) *Problem {
	return wrap(commonproblem.Unauthorized(detail))
}

func Forbidden(detail string) *Problem {
	return wrap(commonproblem.Forbidden(detail))
}

func NotFound(detail string) *Problem {
	return wrap(commonproblem.NotFound(detail))
}

// Validation returns a 422 problem carrying field-level validation failures.
// Used by the request-binding layer when a request body fails struct validation.
func Validation(errs []FieldError) *Problem {
	p := wrap(commonproblem.New(http.StatusUnprocessableEntity, TypeValidation, "Validation Error",
		"the request body failed validation"))
	p.Errors = errs
	return p
}

func FromFiber(err *fiber.Error) *Problem {
	switch err.Code {
	case http.StatusNotFound:
		return NotFound(err.Error())
	default:
		return InternalServer(err.Error())
	}
}

func Conflict(detail string) *Problem {
	return wrap(commonproblem.Conflict(detail))
}

func TooManyRequests(detail string) *Problem {
	return wrap(commonproblem.TooManyRequests(detail))
}

func PayloadTooLarge(detail string) *Problem {
	return wrap(commonproblem.New(http.StatusRequestEntityTooLarge, TypePayloadTooLarge, "Payload Too Large", detail))
}

// NotImplemented reports a capability the requested provider/backend genuinely
// does not have (e.g. the ABRASF 2.04 layout defines no standard DANFSE PDF) —
// not a missing implementation on our side. 501 keeps that distinguishable from
// a 400 (bad input) and a 500 (our bug).
func NotImplemented(detail string) *Problem {
	return wrap(commonproblem.New(http.StatusNotImplemented, TypeNotImplemented, "Not Implemented", detail))
}

func InternalServer(detail string) *Problem {
	return wrap(commonproblem.InternalServer(detail))
}
