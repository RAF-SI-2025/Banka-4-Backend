package errors

import (
	stderrors "errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
)

// AppError represents a structured API error with an HTTP status code and message.
// It implements the error interface and is used throughout the application
// to provide consistent error responses across all services.
//
// Code holds the HTTP status code, Message is the human-readable description
// sent to the client, and Timestamp records when the error occurred.
//
// AppError should not be created using struct literals. Use the provided
// constructors instead, or NewAppError for custom errors if these aren't useful:
//
//	return errors.NotFoundErr("user not found")
//	return errors.BadRequestErr("invalid input")
//	return errors.InternalErr(err)
//	return errors.NewAppError(http.StatusPaymentRequired, "insufficient funds", nil)
//
// In services, return errors using the constructors:
//
//	func (s *userService) GetUser(ctx context.Context, id string) (*domain.User, error) {
//	    user, err := s.repo.GetByID(ctx, id)
//	    if err != nil {
//	        return nil, errors.InternalErr(err)
//	    }
//	    if user == nil {
//	        return nil, errors.NotFoundErr("User not found")
//	    }
//	    return user, nil
type AppError struct {
	Code      int       `json:"-"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Err       error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:      code,
		Status:    http.StatusText(code),
		Message:   message,
		Timestamp: time.Now(),
		Err:       err,
	}
}

func MapGrpcToHttpError(err error) error {
	var appErr *AppError
	if !stderrors.As(err, &appErr) {
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
	switch appErr.Code {
	case http.StatusNotFound:
		return status.Error(codes.NotFound, appErr.Message)
	case http.StatusBadRequest:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case http.StatusConflict:
		return status.Error(codes.AlreadyExists, appErr.Message)
	case http.StatusServiceUnavailable:
		return status.Error(codes.Unavailable, appErr.Message)
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, appErr.Message)
	default:
		return status.Error(codes.Internal, appErr.Message)
	}
}

func BadRequestErr(message string) *AppError {
	return NewAppError(http.StatusBadRequest, message, nil)
}

func UnauthorizedErr(message string) *AppError {
	return NewAppError(http.StatusUnauthorized, message, nil)
}

func ForbiddenErr(message string) *AppError {
	return NewAppError(http.StatusForbidden, message, nil)
}

func NotFoundErr(message string) *AppError {
	return NewAppError(http.StatusNotFound, message, nil)
}

func MethodNotAllowedErr(message string) *AppError {
	return NewAppError(http.StatusMethodNotAllowed, message, nil)
}

func ConflictErr(message string) *AppError {
	return NewAppError(http.StatusConflict, message, nil)
}

func UnprocessableEntityErr(message string) *AppError {
	return NewAppError(http.StatusUnprocessableEntity, message, nil)
}

func RateLimitErr(message string) *AppError {
	return NewAppError(http.StatusTooManyRequests, message, nil)
}

func ServiceUnavailableErr(err error) *AppError {
	return NewAppError(http.StatusServiceUnavailable, "Service Unavailable", err)
}

func GatewayTimeoutErr(err error) *AppError {
	return NewAppError(http.StatusGatewayTimeout, "Gateway Timeout", err)
}

func InternalErr(err error) *AppError {
	return NewAppError(http.StatusInternalServerError, "Internal Server Error", err)
}

func ErrorHandler() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Next()

		contextErrors := context.Errors
		if len(contextErrors) == 0 {
			return
		}

		lastError := contextErrors.Last().Err
		if appErr, ok := stderrors.AsType[*AppError](lastError); ok {
			logError(context, appErr)
			context.JSON(appErr.Code, appErr)
		} else {
			logUnexpectedError(context, lastError)

			context.JSON(
				http.StatusInternalServerError,
				NewAppError(http.StatusInternalServerError, "Internal Server Error", nil),
			)
		}
	}
}

func logError(c *gin.Context, appErr *AppError) {
	if appErr.Code < 500 {
		return
	}

	fields := []zap.Field{
		zap.Int("status", appErr.Code),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
	}

	if appErr.Err != nil {
		fields = append(fields, zap.Error(appErr.Err))
	} else {
		fields = append(fields, zap.String("message", appErr.Message))
	}

	logging.Error("request failed", fields...)
}

func logUnexpectedError(c *gin.Context, err error) {
	logging.Error(
		"unhandled request error",
		zap.Int("status", http.StatusInternalServerError),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
		zap.Error(err),
	)
}
