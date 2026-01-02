package tests

import (
	"github.com/sakashimaa/go-pet-project/auth/pkg/utils/tests"
)

func (s *IntegrationTestSuite) TestLogin_Failure() {
	email := "test@example.com"
	password := "supersecret123qwe"

	invalidEmail := "invalid@example.com"
	invalidPassword := "invalidsecret"

	user, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(user)

	access, refresh, err := s.AuthService.Login(
		s.Ctx,
		invalidEmail,
		invalidPassword,
	)

	s.Require().Error(err)
	s.Require().Empty(access)
	s.Require().Empty(refresh)

	access, refresh, err = s.AuthService.Login(
		s.Ctx,
		email,
		invalidPassword,
	)

	s.Require().Error(err)
	s.Require().Empty(access)
	s.Require().Empty(refresh)

	access, refresh, err = s.AuthService.Login(
		s.Ctx,
		invalidEmail,
		password,
	)

	s.Require().Error(err)
	s.Require().Empty(access)
	s.Require().Empty(refresh)
}

func (s *IntegrationTestSuite) TestLogin_Success() {
	email := "test@example.com"
	password := "supersecret123qwe"

	user, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(user)

	access, refresh, err := s.AuthService.Login(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)

	tests.ValidateTokens(s.T(), access, refresh)
}
