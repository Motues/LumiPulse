package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

// CreateSubscriber 新建订阅。立即生效（本项目不做双重确认），
// 因此 verified 直接置 1，表示这条订阅是可投递的有效订阅。
//
// 注意参数顺序必须与 INSERT 列顺序一致：曾经把 now 和 services 写反，
// 导致 subscribed_services 存进了时间戳、created_at 存进了服务列表。
func (r *repo) CreateSubscriber(ctx context.Context, email string, services string) (*model.Subscriber, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO Subscriber (email, verified, subscribed_services, created_at, updated_at) VALUES (?, 1, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, email, services, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.Subscriber{
		ID:                 id,
		SubscribedServices: services,
		Email:              email,
		Verified:           true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func (r *repo) ListSubscribers(ctx context.Context) ([]*model.Subscriber, error) {
	var subscribers []*model.Subscriber
	err := r.db.SelectContext(ctx, &subscribers, "SELECT * FROM Subscriber ORDER BY created_at DESC")
	return subscribers, err
}

func (r *repo) DeleteSubscriber(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Subscriber WHERE id = ?", id)
	return err
}

func (r *repo) GetSubscriberByEmail(ctx context.Context, email string) (*model.Subscriber, error) {
	var s model.Subscriber
	err := r.db.GetContext(ctx, &s, "SELECT * FROM Subscriber WHERE email = ?", email)
	return &s, err
}

func (r *repo) UpdateSubscriberServices(ctx context.Context, email string, services string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := r.db.ExecContext(ctx, "UPDATE Subscriber SET subscribed_services=?, updated_at=? WHERE email=?", services, now, email)
	return err
}
