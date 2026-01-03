package tests

import (
	"fmt"
	"time"
)

func (s *IntegrationTestSuite) TestCreateOrder_Success() {
	s.seedData(999, "test@example.com")
	resp := s.createOrder(999)

	outboxQuery := `
		SELECT id
		FROM outbox
		WHERE aggregate_id = $1
	`

	var outboxId int64
	err := s.DbPool.QueryRow(s.Ctx, outboxQuery, fmt.Sprintf("%d", resp.OrderId)).
		Scan(&outboxId)
	s.Require().NoError(err)
	s.Require().NotNil(outboxId)

	publishedAtQuery := `
		SELECT published_at
		FROM outbox
		WHERE aggregate_id = $1
	`

	s.Require().Eventually(func() bool {
		var publishedAt *time.Time

		err = s.DbPool.QueryRow(s.Ctx, publishedAtQuery, fmt.Sprintf("%d", resp.OrderId)).
			Scan(&publishedAt)
		if err != nil || publishedAt == nil {
			return false
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}
