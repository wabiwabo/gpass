// Package statuscode provides application-level status codes for
// inter-service communication. Maps domain-specific outcomes to
// standard status representations.
package statuscode

// Code is an application status code.
type Code int

const (
	OK                 Code = 0
	InvalidInput       Code = 1001
	NotFound           Code = 1002
	AlreadyExists      Code = 1003
	PermissionDenied   Code = 1004
	Unauthenticated    Code = 1005
	ResourceExhausted  Code = 1006
	FailedPrecondition Code = 1007
	Aborted            Code = 1008
	Internal           Code = 2001
	Unavailable        Code = 2002
	DataLoss           Code = 2003
	Timeout            Code = 2004
	ConsentRequired    Code = 3001
	ConsentExpired     Code = 3002
	NIKInvalid         Code = 3003
	NPWPInvalid        Code = 3004
	NIBInvalid         Code = 3005
	CertRevoked        Code = 3006
	SignatureInvalid   Code = 3007
)

// String returns the code name.
func (c Code) String() string {
	switch c {
	case OK:
		return "ok"
	case InvalidInput:
		return "invalid_input"
	case NotFound:
		return "not_found"
	case AlreadyExists:
		return "already_exists"
	case PermissionDenied:
		return "permission_denied"
	case Unauthenticated:
		return "unauthenticated"
	case ResourceExhausted:
		return "resource_exhausted"
	case FailedPrecondition:
		return "failed_precondition"
	case Aborted:
		return "aborted"
	case Internal:
		return "internal"
	case Unavailable:
		return "unavailable"
	case DataLoss:
		return "data_loss"
	case Timeout:
		return "timeout"
	case ConsentRequired:
		return "consent_required"
	case ConsentExpired:
		return "consent_expired"
	case NIKInvalid:
		return "nik_invalid"
	case NPWPInvalid:
		return "npwp_invalid"
	case NIBInvalid:
		return "nib_invalid"
	case CertRevoked:
		return "cert_revoked"
	case SignatureInvalid:
		return "signature_invalid"
	default:
		return "unknown"
	}
}

// IsOK checks if the code indicates success.
func (c Code) IsOK() bool {
	return c == OK
}

// IsClientError checks if the code is a client-side error (1xxx).
func (c Code) IsClientError() bool {
	return c >= 1000 && c < 2000
}

// IsServerError checks if the code is a server-side error (2xxx).
func (c Code) IsServerError() bool {
	return c >= 2000 && c < 3000
}

// IsDomainError checks if the code is a domain-specific error (3xxx).
func (c Code) IsDomainError() bool {
	return c >= 3000 && c < 4000
}

// IsRetryable checks if the error is worth retrying.
func (c Code) IsRetryable() bool {
	switch c {
	case Internal, Unavailable, Timeout, ResourceExhausted:
		return true
	}
	return false
}

// HTTPStatus returns the appropriate HTTP status code.
func (c Code) HTTPStatus() int {
	switch c {
	case OK:
		return 200
	case InvalidInput:
		return 400
	case Unauthenticated:
		return 401
	case PermissionDenied, ConsentRequired, ConsentExpired:
		return 403
	case NotFound:
		return 404
	case AlreadyExists:
		return 409
	case ResourceExhausted:
		return 429
	case FailedPrecondition, NIKInvalid, NPWPInvalid, NIBInvalid,
		CertRevoked, SignatureInvalid:
		return 422
	case Aborted:
		return 409
	case Internal, DataLoss:
		return 500
	case Unavailable:
		return 503
	case Timeout:
		return 504
	default:
		return 500
	}
}
