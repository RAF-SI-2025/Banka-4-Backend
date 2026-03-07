package errors

import (
	stderrors "errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	Code      int       `json:"code"`
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

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Timestamp: time.Now(), Err: err}
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
			context.JSON(appErr.Code, appErr.Message)
		} else {
			context.JSON(500, &AppError{Code: 500, Message: "Internal Server Error"})
		}
	}
}
