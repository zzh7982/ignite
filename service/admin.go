package service

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/go-ignite/ignite/api"
	"github.com/go-ignite/ignite/model"
)

// --- admin

func (s *Service) AdminLogin(c *gin.Context) {
	req := new(api.AdminLoginRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	if req.Username != s.opts.Config.AdminUsername || req.Password != s.opts.Config.AdminPassword {
		s.errJSON(c, http.StatusUnauthorized, nil)
		return
	}

	token, err := s.createToken(s.opts.Config.AdminUsername)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, api.AdminLoginResponse{Token: token})
}

// --- account

func (s *Service) GetAccountList(c *gin.Context) {
	req := new(api.PagingRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	users, total, err := s.opts.ModelHandler.GetUserList(req.Keyword, req.PageIndex, req.PageSize)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	resp := make([]*api.User, 0, len(users))
	for _, user := range users {
		resp = append(resp, user.Output())
	}

	c.JSON(http.StatusOK, api.PagingResponse{
		List:          resp,
		Total:         total,
		PagingRequest: *req,
	})
}

func (s *Service) DestroyAccount(c *gin.Context) {
	userID := c.Param("id")

	f := func() error {
		// TODO clean up containers
		return nil
	}

	if err := s.opts.ModelHandler.DestroyUser(userID, f); err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// --- invite code

func (s *Service) GetInviteCodeList(c *gin.Context) {
	req := new(api.PagingRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	inviteCodes, total, err := s.opts.ModelHandler.GetAvailableInviteCodeList(req.PageIndex, req.PageSize)
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	resp := make([]*api.InviteCode, 0, len(inviteCodes))
	for _, ic := range inviteCodes {
		resp = append(resp, ic.Output())
	}

	c.JSON(http.StatusOK, api.PagingResponse{
		List:          resp,
		Total:         total,
		PagingRequest: *req,
	})
}

func (s *Service) RemoveInviteCode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	if err := s.opts.ModelHandler.DeleteInviteCode(id); err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (s *Service) GenerateInviteCodes(c *gin.Context) {
	req := new(api.GenerateCodesRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	var codes []*model.InviteCode
	for i := 0; i < int(req.Amount); i++ {
		codes = append(codes, model.NewInviteCode(req.Limit, req.ExpiredAt))
	}

	if err := s.opts.ModelHandler.CreateInviteCodes(codes); err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// --- node

func (s *Service) AddNode(c *gin.Context) {
	req := new(api.AddNodeRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	node := model.NewNode(req.Name, req.Comment, req.RequestAddress, req.ConnectionAddress, req.PortFrom, req.PortTo)
	f := func() error {
		return s.opts.ModelHandler.CreateNode(node)
	}

	if err := s.opts.StateHandler.AddNode(c.Request.Context(), node, f); err != nil {
		switch err {
		case api.ErrNodeNameExists:
			s.errJSON(c, http.StatusBadRequest, err)
		case api.ErrNodeRequestAddressExists:
			s.errJSON(c, http.StatusBadRequest, err)
		default:
			s.errJSON(c, http.StatusInternalServerError, err)
		}

		return
	}

	c.JSON(http.StatusCreated, node.Output())
}

func (s *Service) GetAllNodes(c *gin.Context) {
	nodes, err := s.opts.ModelHandler.GetAllNodes()
	if err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	resp := make([]*api.Node, 0, len(nodes))
	for _, node := range nodes {
		resp = append(resp, node.Output())
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Service) DeleteNode(c *gin.Context) {
	id := c.Param("id")

	f := func() error {
		s.opts.StateHandler.RemoveNode(id)
		return nil
	}

	if err := s.opts.ModelHandler.DeleteNode(id, f); err != nil {
		s.errJSON(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (s *Service) UpdateNode(c *gin.Context) {
	req := new(api.UpdateNodeRequest)
	if err := c.ShouldBind(req); err != nil {
		s.errJSON(c, http.StatusBadRequest, err)
		return
	}

	node := &model.Node{
		ID:                c.Param("id"),
		Name:              req.Name,
		Comment:           req.Comment,
		ConnectionAddress: req.ConnectionAddress,
		PortFrom:          req.PortFrom,
		PortTo:            req.PortTo,
	}

	f := func() error {
		return s.opts.ModelHandler.UpdateNode(node)
	}
	if err := s.opts.StateHandler.UpdateNode(node, f); err != nil {
		switch err {
		case api.ErrNodeNotExist:
			s.errJSON(c, http.StatusNotFound, err)
		case api.ErrNodeHasServicesExceedPortRange:
			s.errJSON(c, http.StatusBadRequest, err)
		default:
			s.errJSON(c, http.StatusInternalServerError, err)
		}

		return
	}

	c.JSON(http.StatusNoContent, nil)
}