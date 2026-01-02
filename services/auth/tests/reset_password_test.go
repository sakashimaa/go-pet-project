package tests

import (
	"github.com/sakashimaa/go-pet-project/auth/pkg/utils/tests"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
)

func (s *IntegrationTestSuite) TestResetPassword_Success() {
	email := "test@example.com"
	password := "secretpass123qwe"

	res, err := s.AuthService.Register(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotNil(res)

	forgotRes, err := s.AuthService.ForgotPassword(s.Ctx, &pb.ForgotPasswordRequest{Email: email})

	s.Require().NoError(err)
	s.Require().NotNil(forgotRes)
	s.Require().True(forgotRes.Success)

	tokenQuery := `
		SELECT forgot_password_token
		FROM users
		WHERE email = $1
	`

	var token string
	if err := s.DbPool.QueryRow(s.Ctx, tokenQuery, email).
		Scan(&token); err != nil {
		s.Require().NoError(err, "error querying row")
	}

	newPassword := "recoverypass123"
	resetRes, err := s.AuthService.ResetPassword(
		s.Ctx,
		&pb.ResetPasswordRequest{Token: token, Password: newPassword},
	)

	s.Require().NoError(err)
	s.Require().NotNil(resetRes)
	s.Require().True(resetRes.Success)

	_, _, err = s.AuthService.Login(
		s.Ctx,
		email,
		password,
	)

	s.Require().Error(err, "Old password must fail")

	access, refresh, err := s.AuthService.Login(
		s.Ctx,
		email,
		newPassword,
	)

	tests.ValidateTokens(s.T(), access, refresh)
}

func (s *IntegrationTestSuite) TestResetPassword_Failure() {
	email := "test@example.com"
	password := "secretpass123qwe"

	res, err := s.AuthService.Register(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotNil(res)

	forgotRes, err := s.AuthService.ForgotPassword(s.Ctx, &pb.ForgotPasswordRequest{Email: "notexisting@email.com"})

	s.Require().Error(err)
	s.Require().Nil(forgotRes)

	forgotRes, err = s.AuthService.ForgotPassword(s.Ctx, &pb.ForgotPasswordRequest{Email: email})

	s.Require().NoError(err)
	s.Require().NotNil(forgotRes)
	s.Require().True(forgotRes.Success)

	resetRes, err := s.AuthService.ResetPassword(
		s.Ctx,
		&pb.ResetPasswordRequest{Token: "faketoken123", Password: "fakepass123"},
	)

	s.Require().Error(err)
	s.Require().Nil(resetRes)
}

func (s *IntegrationTestSuite) TestResetPasswordToken_Reuse_Failure() {
	email := "test@example.com"
	password := "secretpass123A1"

	res, err := s.AuthService.Register(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotNil(res)

	forgotRes, err := s.AuthService.ForgotPassword(
		s.Ctx,
		&pb.ForgotPasswordRequest{
			Email: email,
		},
	)

	s.Require().NoError(err)
	s.Require().NotNil(forgotRes)
	s.Require().True(forgotRes.Success)

	queryToken := `
		SELECT forgot_password_token
		FROM users
		WHERE email = $1;
	`

	var token string
	if err := s.DbPool.QueryRow(s.Ctx, queryToken, email).
		Scan(&token); err != nil {
		s.Require().NoError(err, "error querying row")
	}

	resetRes, err := s.AuthService.ResetPassword(
		s.Ctx,
		&pb.ResetPasswordRequest{
			Token:    token,
			Password: "supersecret123",
		},
	)

	s.Require().NoError(err)
	s.Require().NotNil(resetRes)
	s.Require().True(resetRes.Success)

	failedRes, err := s.AuthService.ResetPassword(
		s.Ctx,
		&pb.ResetPasswordRequest{
			Token:    token,
			Password: "supersecretqwerty123",
		},
	)

	s.Require().Error(err)
	s.Require().Nil(failedRes)
}
