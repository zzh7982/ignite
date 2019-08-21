package service

import (
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/go-ignite/ignite-agent/protos"
	"github.com/go-ignite/ignite/api"
	"github.com/go-ignite/ignite/model"
)

func (s *Service) UserLogin(c *gin.Context) {
	req := new(api.UserLoginRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	user, err := s.opts.ModelHandler.GetUserByName(req.Username)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	if user == nil {
		s.errJSON(c, http.StatusUnauthorized, nil)
		return
	}

	if err := bcrypt.CompareHashAndPassword(user.HashedPwd, []byte(req.Password)); err != nil {
		s.errJSON(c, http.StatusUnauthorized, nil)
		return
	}

	token, err := s.createToken(user.ID)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, &api.UserLoginResponse{Token: token})
}

func (s *Service) UserRegister(c *gin.Context) {
	req := new(api.UserRegisterRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}
	if err := checkPassword(req.Password); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	user := model.NewUser(req.Username, hashedPass, req.InviteCode)
	if err := s.opts.StateHandler.AddUser(user); err != nil {
		switch err {
		case api.ErrInviteCodeNotExistOrUnavailable:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		case api.ErrInviteCodeExpired:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		case api.ErrUserNameExists:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		default:
			s.errJSON(c, http.StatusInternalServerError, err)
		}

		return
	}

	token, err := s.createToken(user.ID)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, api.UserResisterResponse{Token: token})
}

func (s *Service) UserChangePassword(c *gin.Context) {
	req := new(api.UserChangePasswordRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	if err := checkPassword(req.NewPassword); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	userID := c.GetString("id")
	if err := s.opts.StateHandler.ChangeUserPassword(userID, req.NewPassword, &req.OldPassword); err != nil {
		switch err {
		case api.ErrUserPasswordIncorrect:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		default:
			s.errJSON(c, http.StatusInternalServerError, err)
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

func (s *Service) GetUserInfo(c *gin.Context) {
	userID := c.GetString("id")
	user, err := s.opts.ModelHandler.GetUserByID(userID)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		s.errJSON(c, http.StatusNotFound, nil)
		return
	}

	c.JSON(http.StatusOK, &api.User{
		ID:   userID,
		Name: user.Name,
	})
}

func (s *Service) UserServicesSync(c *gin.Context) {
	userID := c.GetString("id")
	first := true
	c.Stream(func(w io.Writer) bool {
		if !first {
			time.Sleep(s.opts.Config.SyncInterval)
		} else {
			first = false
		}

		r := s.opts.StateHandler.GetSyncResponse(userID)
		if len(r) > 0 {
			c.SSEvent("user_sync", r[0])
		} else {
			return false
		}

		return true
	})
}

func (s *Service) CreateService(c *gin.Context) {
	userID := c.GetString("id")
	req := &api.CreateServiceRequest{}
	if err := c.BindJSON(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	user, err := s.opts.ModelHandler.GetUserByID(userID)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		s.errJSON(c, http.StatusUnauthorized, nil)
		return
	}

	sc := &model.ServiceConfig{
		EncryptionMethod: req.EncryptionMethod,
		Password:         user.ServicePassword,
	}
	service := model.NewService(userID, req.NodeID, req.Type, sc)

	if err := s.opts.StateHandler.AddService(service); err != nil {
		switch err {
		case api.ErrServiceExists:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		case api.ErrNodeNotExist:
			s.errJSON(c, http.StatusBadRequest, err)
		case api.ErrNodeUnavailable:
			s.errJSON(c, http.StatusPreconditionFailed, err)
		default:
			s.errJSON(c, http.StatusInternalServerError, err)
		}
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (s *Service) GetUserServices(c *gin.Context) {
	resp := make([]*api.NodeService, 0)
	for _, ns := range s.opts.StateHandler.GetNodeServices(c.GetString("id"), "") {
		r := &api.NodeService{
			Node: ns.Node,
		}
		if len(ns.Services) > 0 {
			r.Service = ns.Services[0]
		}

		resp = append(resp, r)
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Service) GetServiceOptions(c *gin.Context) {
	sos := make([]*api.ServiceOptions, 0, len(protos.ServiceType_Enum_name))
	for _, t := range []protos.ServiceType_Enum{protos.ServiceType_SS_LIBEV, protos.ServiceType_SSR} {
		so := &api.ServiceOptions{
			Type: t,
		}
		for k := range protos.ServiceEncryptionMethod_Enum_name {
			m := protos.ServiceEncryptionMethod_Enum(k)
			if m == protos.ServiceEncryptionMethod_NOT_SET {
				continue
			}

			if t.Suit(m) {
				so.EncryptionMethods = append(so.EncryptionMethods, m)
			}
		}
		sos = append(sos, so)
	}

	for _, so := range sos {
		sort.Slice(so.EncryptionMethods, func(i, j int) bool {
			return so.EncryptionMethods[i] < so.EncryptionMethods[j]
		})
	}

	c.JSON(http.StatusOK, sos)
}
