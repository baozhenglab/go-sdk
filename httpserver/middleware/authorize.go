package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v2"

	"github.com/baozhenglab/go-sdk/v2/logger"
	"github.com/baozhenglab/go-sdk/v2/util"
	"github.com/baozhenglab/oauthclient"
	"github.com/baozhenglab/sdkcm"
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

type CurrentUserContext struct {
	sdkcm.OAuth
	sdkcm.User
}

func Authorize(cup CurrentUserProvider, isRequired ...bool) fiber.Handler {
	required := len(isRequired) == 0

	return func(c *fiber.Ctx) error {
		token := accessTokenFromRequest(c.Request())

		if token == "" {
			if required {
				panic(sdkcm.ErrUnauthorized(nil, sdkcm.ErrAccessTokenInvalid))
			} else {
				c.Locals("current_user", util.EncodeUser(guest{}))
				return c.Next()
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
		u, err := cup.GetCurrentUser(c.Context(), tokenInfo.UserId)

		if err != nil {
			panic(sdkcm.ErrUnauthorized(err, sdkcm.ErrUserNotFound))
		}

		c.Locals("current_user", sdkcm.CurrentUser(tokenInfo, u))
		return c.Next()
	}
}

func RequireRoles(roles ...fmt.Stringer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		r := c.Get("current_user")

		if r == "" {
			panic(sdkcm.ErrUnauthorized(sdkcm.ErrNoPermission, sdkcm.ErrNoPermission))
		}
		var requester CurrentUserContext
		util.DecodeUser(r, &requester)
		reqRole := sdkcm.ParseSystemRole(requester.GetSystemRole())

		for _, v := range roles {
			if v.String() == reqRole.String() {
				return c.Next()
			}
		}

		panic(sdkcm.ErrUnauthorized(nil, sdkcm.ErrNoPermission))
		return nil
	}
}

func accessTokenFromRequest(req *fasthttp.Request) string {
	// According to https://tools.ietf.org/html/rfc6750 you can pass tokens through:
	// - Form-Encoded Body Parameter. Recommended, more likely to appear. e.g.: Authorization: Bearer mytoken123
	// - URI Query Parameter e.g. access_token=mytoken123

	auth := string(req.Header.Peek("Authorization"))

	split := strings.SplitN(auth, " ", 2)
	if len(split) != 2 || !strings.EqualFold(split[0], "bearer") {
		// Nothing in Authorization header, try access_token
		// Empty string returned if there's no such parameter
		return ""
	}

	return split[1]
}

type guest struct{}

func (g guest) OAuthID() string       { return "" }
func (g guest) UserID() uint32        { return 0 }
func (g guest) GetSystemRole() string { return sdkcm.SysRoleGuest.String() }
