package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/go-ignite/ignite/api"
	"github.com/go-ignite/ignite/config"
	"github.com/go-ignite/ignite/model"
	"github.com/go-ignite/ignite/state"
)

var Set = wire.NewSet(wire.Struct(new(Options), "*"), New)

type Options struct {
	StateHandler *state.Handler
	ModelHandler *model.Handler
	Config       config.Service
}

type Service struct {
	opts *Options
}

func New(opts *Options) *Service {
	return &Service{
		opts: opts,
	}
}

func (s *Service) errJSON(c *gin.Context, statusCode int, err error) {
	if v, ok := err.(*api.ErrResponse); ok {
		c.JSON(statusCode, v)
		return
	}

	var message string
	if err == nil {
		message = http.StatusText(statusCode)
	} else {
		message = err.Error()
	}

	c.JSON(statusCode, api.NewErrResponse(statusCode, message))
}

func (s *Service) createToken(id string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(s.opts.Config.TokenDuration).Unix(),
	})

	tokenStr, err := token.SignedString([]byte(s.opts.Config.Secret))
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func checkPassword(password string) error {
	if len(password) < 6 || len(password) > 12 {
		return fmt.Errorf("invalid password")
	}

	return nil
}

func (s *Service) Auth(isAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := request.ParseFromRequest(c.Request, request.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
			b := []byte(s.opts.Config.Secret)
			return b, nil
		})
		if err != nil {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		if !token.Valid {
			_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("token is invalid"))
			return
		}

		claims := token.Claims.(jwt.MapClaims)
		if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
			_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("token is expired"))
			return
		}
		id, ok := claims["id"].(string)
		if !ok {
			_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("token'id is invalid"))
			return
		}
		if (isAdmin && id != s.opts.Config.AdminUsername) || (!isAdmin && id == "") {
			_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("token auth error"))
			return
		}

		if !isAdmin && !s.opts.StateHandler.CheckUserExists(id) {
			_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("user not exists"))
			return
		}

		c.Set("id", claims["id"])
		c.Set("token", token.Raw)
	}
}
