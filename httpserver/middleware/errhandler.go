package middleware

import (
	"net/http"

	"github.com/baozhenglab/go-sdk/v2/logger"

	"github.com/baozhenglab/sdkcm"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/go-playground/validator.v9"
)

func ErrorHandler(logger logger.Logger) func(*fiber.Ctx, error) error {
	return func(c *fiber.Ctx, err error) error {
		var appErr sdkcm.AppError
		if appErr, ok := err.(sdkcm.AppError); ok {
			appErr.RootCause = appErr.RootError()
			logger.Errorln(appErr.RootCause)
			if appErr.RootCause != nil {
				appErr.Log = appErr.RootCause.Error()
			}
			//if lvLogger == logrus.TraceLevel.String() {
			//	panic(err)
			//}
			return c.Status(appErr.StatusCode).JSON(appErr)
		} else if formErr, ok := err.(validator.ValidationErrors); ok {
			message := sdkcm.GetErrors(formErr)
			appErr = sdkcm.ErrUnprocessableEntity(message)
			return c.Status(appErr.StatusCode).JSON(appErr)
		} else if e, ok := err.(error); ok {

			appErr = sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
			logger.Errorln(e.Error())
			return c.Status(appErr.StatusCode).JSON(appErr)
		} else {
			appErr = sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
			logger.Errorln(err)
			return c.Status(appErr.StatusCode).JSON(appErr)
		}

		//if lvLogger == logrus.TraceLevel.String() {
		//	panic(err)
		//}
		return c.Status(appErr.StatusCode).JSON(appErr)
	}
}
