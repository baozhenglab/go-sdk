package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/baozhenglab/go-sdk/v2/logger"
	"github.com/baozhenglab/oauthclient"
	"github.com/baozhenglab/sdkcm"
	"github.com/gin-gonic/gin"
)

type ServiceContext interface {
	Logger(prefix string) logger.Logger
	Get(prefix string) (interface{}, bool)
	MustGet(prefix string) interface{}
}

type CurrentUserProvider interface {
	GetCurrentUser(ctx context.Context, oauthID string) (sdkcm.User, error)
	ServiceContext
}

func Authorize(cup CurrentUserProvider, isRequired ...bool) gin.HandlerFunc {
	required := len(isRequired) == 0

	return func(c *gin.Context) {
		token := accessTokenFromRequest(c.Request)

		if token == "" {
			if required {
				panic(sdkcm.ErrUnauthorized(nil, sdkcm.ErrAccessTokenInvalid))
			} else {
				c.Set("current_user", guest{})
				c.Next()
				return
			}

		}

		tc := cup.MustGet("oauth").(oauthclient.TrustedClient)
		tokenInfo, err := tc.Introspect(token)

		if err != nil {
			panic(sdkcm.ErrUnauthorized(err, sdkcm.ErrAccessTokenInactivated))
		}

		if !tokenInfo.Active {
			panic(sdkcm.ErrUnauthorized(sdkcm.ErrAccessTokenInactivated, sdkcm.ErrAccessTokenInactivated))
		}

		// Fetch user info from db
		u, err := cup.GetCurrentUser(c.Request.Context(), tokenInfo.UserId)

		if err != nil {
			panic(sdkcm.ErrUnauthorized(err, sdkcm.ErrUserNotFound))
		}

		c.Set("current_user", sdkcm.CurrentUser(tokenInfo, u))
	}
}

func RequireRoles(roles ...fmt.Stringer) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, ok := c.Get("current_user")

		if !ok {
			panic(sdkcm.ErrUnauthorized(sdkcm.ErrNoPermission, sdkcm.ErrNoPermission))
		}

		requester := r.(sdkcm.Requester)
		reqRole := sdkcm.ParseSystemRole(requester.GetSystemRole())

		for _, v := range roles {
			if v.String() == reqRole.String() {
				c.Next()
				return
			}
		}

		panic(sdkcm.ErrUnauthorized(nil, sdkcm.ErrNoPermission))
	}
}

func accessTokenFromRequest(req *http.Request) string {
	// According to https://tools.ietf.org/html/rfc6750 you can pass tokens through:
	// - Form-Encoded Body Parameter. Recommended, more likely to appear. e.g.: Authorization: Bearer mytoken123
	// - URI Query Parameter e.g. access_token=mytoken123

	auth := req.Header.Get("Authorization")
	split := strings.SplitN(auth, " ", 2)
	if len(split) != 2 || !strings.EqualFold(split[0], "bearer") {
		// Nothing in Authorization header, try access_token
		// Empty string returned if there's no such parameter
		if err := req.ParseMultipartForm(1 << 20); err != nil && err != http.ErrNotMultipart {
			return ""
		}
		return req.Form.Get("access_token")
	}

	return split[1]
}

type guest struct{}

func (g guest) OAuthID() string       { return "" }
func (g guest) UserID() uint32        { return 0 }
func (g guest) GetSystemRole() string { return sdkcm.SysRoleGuest.String() }
