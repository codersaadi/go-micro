package micro

import (
	"go.uber.org/zap"
)

// Field helpers
func MethodField(method string) zap.Field {
	return zap.String("method", method)
}

func UserIDField(id int32) zap.Field {
	return zap.Int32("user_id", id)
}

func EmailField(email string) zap.Field {
	return zap.String("email", email)
}

func ErrorField(err error) zap.Field {
	return zap.Error(err)
}
