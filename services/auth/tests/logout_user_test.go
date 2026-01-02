package tests

import (
	"github.com/sakashimaa/go-pet-project/auth/pkg/utils/tests"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
)

func (s *IntegrationTestSuite) TestLogoutUser_Success() {
	_, err := s.DbPool.Exec(s.Ctx, "DELETE FROM refresh_sessions")
	s.Require().NoError(err)

	email := "test@example.com"
	password := "qwertysecret123"

	res, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(res)

	access, refresh, err := s.AuthService.Login(s.Ctx, email, password)
	s.Require().NoError(err)
	s.Require().NotEmpty(access)
	s.Require().NotEmpty(refresh)

	tests.ValidateTokens(s.T(), access, refresh)

	sessionQuery := `
		SELECT token
		FROM refresh_sessions
		WHERE user_id = $1;
	`

	var sessionToken string
	err = s.DbPool.QueryRow(s.Ctx, sessionQuery, res.ID).
		Scan(&sessionToken)
	s.Require().NoError(err)

	s.Require().NotEmpty(sessionToken)
	s.Require().Equal(refresh, sessionToken)

	logoutRes, err := s.AuthService.Logout(s.Ctx, &pb.LogoutRequest{RefreshToken: refresh})

	s.Require().NoError(err)
	s.Require().NotNil(logoutRes)
	s.Require().True(logoutRes.Success)

	var count int
	err = s.DbPool.QueryRow(s.Ctx, "SELECT count(*) FROM refresh_sessions WHERE user_id = $1", res.ID).Scan(&count)
	s.Require().NoError(err)
	s.Require().Equal(0, count, "В базе не должно остаться ни одной сессии для этого юзера")
}

func (s *IntegrationTestSuite) TestLogoutUser_Failure() {
	_, err := s.DbPool.Exec(s.Ctx, "DELETE FROM refresh_sessions")
	s.Require().NoError(err)

	res, err := s.AuthService.Logout(s.Ctx, &pb.LogoutRequest{RefreshToken: "invalidrefresh"})
	s.Require().Error(err)
	s.Require().Nil(res)

	email := "test@example.com"
	password := "qwertysuper123"

	registerRes, err := s.AuthService.Register(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotNil(registerRes)

	access, refresh, err := s.AuthService.Login(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotEmpty(access)
	s.Require().NotEmpty(refresh)

	tests.ValidateTokens(s.T(), access, refresh)

	sessionQuery := `
		SELECT token
		FROM refresh_sessions
		WHERE user_id = $1
	`

	var sessionToken string
	err = s.DbPool.QueryRow(s.Ctx, sessionQuery, registerRes.ID).
		Scan(&sessionToken)

	s.Require().NoError(err)
	s.Require().Equal(refresh, sessionToken)

	logoutRes, err := s.AuthService.Logout(s.Ctx, &pb.LogoutRequest{RefreshToken: refresh})

	s.Require().NoError(err)
	s.Require().NotNil(logoutRes)
	s.Require().True(logoutRes.Success)

	err = s.DbPool.QueryRow(s.Ctx, sessionQuery, registerRes.ID).
		Scan(&sessionToken)

	s.Require().Error(err)

	logoutRes, err = s.AuthService.Logout(s.Ctx, &pb.LogoutRequest{RefreshToken: refresh})

	s.Require().Error(err)
	s.Require().Nil(logoutRes)
}

func (s *IntegrationTestSuite) TestMultiDevice_Success() {
	_, err := s.DbPool.Exec(s.Ctx, "DELETE FROM refresh_sessions")
	s.Require().NoError(err)

	email := "test@example.com"
	password := "qwertysuper123"

	res, err := s.AuthService.Register(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotNil(res)

	access, refresh, err := s.AuthService.Login(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotEmpty(access)
	s.Require().NotEmpty(refresh)

	tests.ValidateTokens(s.T(), access, refresh)

	mobAccess, mobRefresh, err := s.AuthService.Login(s.Ctx, email, password)

	s.Require().NoError(err)
	s.Require().NotEmpty(mobAccess)
	s.Require().NotEmpty(mobRefresh)

	tests.ValidateTokens(s.T(), mobAccess, mobRefresh)

	s.Require().NotEqual(mobRefresh, refresh, "Токены с ПК и мобильных устройств не должны совпадать!")

	logoutRes, err := s.AuthService.Logout(s.Ctx, &pb.LogoutRequest{RefreshToken: refresh})

	s.Require().NoError(err)
	s.Require().NotNil(logoutRes)

	refreshQuery := `
		SELECT user_id
		FROM refresh_sessions
		WHERE token = $1
	`

	var userId int64
	err = s.DbPool.QueryRow(s.Ctx, refreshQuery, mobRefresh).
		Scan(&userId)

	s.Require().NoError(err)
	s.Require().Equal(userId, res.ID)
}
