package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateSubscriber(ctx context.Context, email string, services string) (*model.Subscriber, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO Subscriber (email, verified, subscribed_services, created_at, updated_at) VALUES (?, 0, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, email, now, services, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.Subscriber{
		ID:                id,
			SubscribedServices: services,
		Email:     email,
		Verified:  false,
		CreatedAt: now,
		UpdatedAt: now,
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
