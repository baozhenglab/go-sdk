package middleware

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/baozhenglab/sdkcm"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v9"
)

func Recover(sc ServiceContext) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logger := sc.Logger("service")
		lvLogger := logger.GetLevel()

		defer func() {
			if err := recover(); err != nil {
				if appErr, ok := err.(sdkcm.AppError); ok {
					appErr.RootCause = appErr.RootError()
					logger.Errorln(appErr.RootCause)

					if appErr.RootCause != nil {
						appErr.Log = appErr.RootCause.Error()
					}
					if lvLogger == logrus.TraceLevel.String() {
						panic(err)
					}
					c.Status(appErr.StatusCode).JSON(appErr)
				} else if formErr, ok := err.(validator.ValidationErrors); ok {
					message := sdkcm.GetErrors(formErr)
					errc := sdkcm.ErrUnprocessableEntity(message)
					c.Status(errc.StatusCode).JSON(errc)
				} else if e, ok := err.(error); ok {

					appErr := sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
					logger.Errorln(e.Error())
					c.Status(appErr.StatusCode).JSON(appErr)
				} else {
					appErr := sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
					logger.Errorln(e)
					c.Status(appErr.StatusCode).JSON(appErr)
				}

				if lvLogger == logrus.TraceLevel.String() {
					panic(err)
				}
			}
		}()

		return c.Next()
	}
}
