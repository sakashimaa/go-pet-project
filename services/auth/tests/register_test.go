package tests

import (
	"fmt"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func (s *IntegrationTestSuite) TestRegisterUser_Success() {
	email := "test@example.com"
	password := "secret123qwe"

	user, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(user)

	outboxQuery := `
		SELECT id
		FROM outbox
		WHERE aggregate_id = $1;
	`

	var outboxId int64
	err = s.DbPool.QueryRow(s.Ctx, outboxQuery, fmt.Sprintf("%d", user.ID)).
		Scan(&outboxId)
	s.Require().NoError(err)
	s.Require().NotZero(outboxId)

	var dbEmail string
	var dbPassHash string
	err = s.DbPool.QueryRow(s.Ctx, "SELECT email, password_hash FROM users WHERE id=$1", user.ID).
		Scan(&dbEmail, &dbPassHash)
	s.Require().NoError(err)

	s.Equal(email, dbEmail)
	s.NotEqual(password, dbPassHash)

	query := `
		SELECT published_at
		FROM outbox
		WHERE aggregate_id = $1
	`

	s.Require().Eventually(func() bool {
		var publishedAt *time.Time
		err := s.DbPool.QueryRow(s.Ctx, query, fmt.Sprintf("%d", user.ID)).
			Scan(&publishedAt)

		if err != nil || publishedAt == nil {
			return false
		}

		return true
	}, 5*time.Second, 100*time.Millisecond, "Сообщение должно быть опубликовано в течении 5 секунд")
}

func (s *IntegrationTestSuite) TestRegisterUser_DuplicateEmail_Fails() {
	email := "test@example.com"
	password := "supersecret123qwe"

	user, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().NoError(err)
	s.Require().NotNil(user)

	user2, err := s.AuthService.Register(
		s.Ctx,
		email,
		password,
	)

	s.Require().Error(err)
	s.Require().Nil(user2)
}
