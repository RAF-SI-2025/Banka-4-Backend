package errors

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewAppError(t *testing.T) {
	inner := fmt.Errorf("wrapped")
	err := NewAppError(http.StatusBadRequest, "bad request", inner)

	require.Equal(t, http.StatusBadRequest, err.Code)
	require.Equal(t, "Bad Request", err.Status)
	require.Equal(t, "bad request", err.Message)
	require.Equal(t, inner, err.Err)
	require.False(t, err.Timestamp.IsZero())
}

func TestAppError_ErrorWithWrapped(t *testing.T) {
	inner := fmt.Errorf("db connection failed")
	err := InternalErr(inner)
	assert.Equal(t, "db connection failed", err.Error())
}

func TestAppError_ErrorWithoutWrapped(t *testing.T) {
	err := BadRequestErr("invalid input")
	assert.Equal(t, "invalid input", err.Error())
}

func TestAppError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	err := InternalErr(inner)
	assert.Equal(t, inner, err.Unwrap())
}

func TestAppError_UnwrapNil(t *testing.T) {
	err := BadRequestErr("no wrap")
	assert.Nil(t, err.Unwrap())
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name    string
		err     *AppError
		code    int
		message string
	}{
		{"BadRequest", BadRequestErr("bad"), http.StatusBadRequest, "bad"},
		{"Unauthorized", UnauthorizedErr("unauth"), http.StatusUnauthorized, "unauth"},
		{"Forbidden", ForbiddenErr("forbidden"), http.StatusForbidden, "forbidden"},
		{"NotFound", NotFoundErr("not found"), http.StatusNotFound, "not found"},
		{"MethodNotAllowed", MethodNotAllowedErr("method"), http.StatusMethodNotAllowed, "method"},
		{"Conflict", ConflictErr("conflict"), http.StatusConflict, "conflict"},
		{"UnprocessableEntity", UnprocessableEntityErr("invalid"), http.StatusUnprocessableEntity, "invalid"},
		{"RateLimit", RateLimitErr("slow down"), http.StatusTooManyRequests, "slow down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.message, tt.err.Message)
			assert.Nil(t, tt.err.Err)
		})
	}
}

func TestConstructorsWithWrappedError(t *testing.T) {
	inner := fmt.Errorf("root")

	tests := []struct {
		name string
		err  *AppError
		code int
	}{
		{"ServiceUnavailable", ServiceUnavailableErr(inner), http.StatusServiceUnavailable},
		{"GatewayTimeout", GatewayTimeoutErr(inner), http.StatusGatewayTimeout},
		{"Internal", InternalErr(inner), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, inner, tt.err.Err)
			assert.Equal(t, "root", tt.err.Error())
		})
	}
}

func TestMapGrpcToHttpError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		grpcCode codes.Code
		msg      string
	}{
		{"NotFound", NotFoundErr("user not found"), codes.NotFound, "user not found"},
		{"BadRequest", BadRequestErr("invalid"), codes.InvalidArgument, "invalid"},
		{"Unauthorized", UnauthorizedErr("no token"), codes.Unauthenticated, "no token"},
		{"Forbidden", ForbiddenErr("denied"), codes.PermissionDenied, "denied"},
		{"Conflict", ConflictErr("exists"), codes.AlreadyExists, "exists"},
		{"ServiceUnavailable", ServiceUnavailableErr(fmt.Errorf("down")), codes.Unavailable, "Service Unavailable"},
		{"RateLimit", RateLimitErr("throttled"), codes.ResourceExhausted, "throttled"},
		{"InternalDefault", NewAppError(http.StatusInternalServerError, "server error", nil), codes.Internal, "server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := MapGrpcToHttpError(tt.input)
			st, ok := status.FromError(grpcErr)
			require.True(t, ok)
			assert.Equal(t, tt.grpcCode, st.Code())
			assert.Equal(t, tt.msg, st.Message())
		})
	}
}

func TestMapGrpcToHttpError_NonAppError(t *testing.T) {
	grpcErr := MapGrpcToHttpError(fmt.Errorf("plain error"))
	st, ok := status.FromError(grpcErr)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "plain error")
}
