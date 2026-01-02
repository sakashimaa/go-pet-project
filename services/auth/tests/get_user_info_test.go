package tests

import "github.com/sakashimaa/go-pet-project/auth/internal/repository"

func (s *IntegrationTestSuite) TestGetUserInfo_Success() {
	email := "test@example.com"
	password := "supersecretqwerty123"

	res, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(res)
	s.Require().NotZero(res.ID)

	info, err := s.AuthService.GetUserInfo(
		s.Ctx,
		res.ID,
	)

	s.Require().NoError(err)
	s.Require().NotNil(info)
	s.Require().Equal(info.ID, res.ID)
	s.Require().Equal(email, info.Email)
}

func (s *IntegrationTestSuite) TestGetUserInfo_NotFound_Failed() {
	info, err := s.AuthService.GetUserInfo(s.Ctx, 99999)

	s.Require().Error(err)
	s.Require().Nil(info)
	s.Require().ErrorIs(err, repository.ErrUserNotFound, "Should return specific domain error")
}
