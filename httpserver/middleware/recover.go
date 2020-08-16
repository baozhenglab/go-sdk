package middleware

import (
	"net/http"

	"github.com/baozhenglab/sdkcm"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v9"
)

func Recover(sc ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
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

					c.AbortWithStatusJSON(appErr.StatusCode, appErr)

					if lvLogger == logrus.TraceLevel.String() {
						panic(err)
					}
				} else if formErr, ok := err.(validator.ValidationErrors); ok {
					message := sdkcm.GetErrors(formErr)
					errc := sdkcm.ErrUnprocessableEntity(message)
					c.AbortWithStatusJSON(errc.StatusCode, errc)
				} else if e, ok := err.(error); ok {

					appErr := sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
					logger.Errorln(e.Error())
					c.AbortWithStatusJSON(appErr.StatusCode, appErr)
				} else {
					appErr := sdkcm.AppError{StatusCode: http.StatusInternalServerError, Message: "internal server error"}
					logger.Errorln(e)
					c.AbortWithStatusJSON(appErr.StatusCode, appErr)
				}

				if lvLogger == logrus.TraceLevel.String() {
					panic(err)
				}
			}
		}()

		c.Next()
	}
}
