package tests

import "github.com/sakashimaa/go-pet-project/order/internal/domain"

func (s *IntegrationTestSuite) TestUserRegistered_Success() {
	var id int64 = 1
	email := "test@example.com"

	err := s.OrderService.HandleUserRegistered(s.Ctx, &domain.UserRegisteredEvent{
		UserID: id,
		Email:  email,
	})
	s.Require().NoError(err)

	var dbEmail string
	query := `
		SELECT email
		FROM users
		WHERE id = $1
	`

	err = s.DbPool.QueryRow(s.Ctx, query, id).
		Scan(&dbEmail)
	s.Require().NoError(err)
	s.Require().Equal(email, dbEmail)
}

func (s *IntegrationTestSuite) TestUserRegisteredValidation_Failure() {
	tests := []struct {
		name   string
		userID int64
		email  string
	}{
		{"Invalid ID", 0, "valid@example.com"},
		{"Invalid email", 999, ""},
		{"Both Invalid", 0, ""},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := s.OrderService.HandleUserRegistered(s.Ctx, &domain.UserRegisteredEvent{
				UserID: tt.userID,
				Email:  tt.email,
			})
			s.Require().Error(err)
		})
	}
}
