package sqlite

import (
	"context"

	errwrap "github.com/pkg/errors"
	"github.com/rahmatrdn/go-ch-manager/entity"
	"github.com/rahmatrdn/go-ch-manager/internal/helper"
	"gorm.io/gorm"
)

type ConnectionRepository interface {
	Create(ctx context.Context, conn *entity.CHConnection) error
	FindAll(ctx context.Context) ([]*entity.CHConnection, error)
	FindByID(ctx context.Context, id int64) (*entity.CHConnection, error)
	Delete(ctx context.Context, id int64) error
	Update(ctx context.Context, conn *entity.CHConnection) error
}

type Connection struct {
	db *gorm.DB
}

func NewConnectionRepository(db *gorm.DB) *Connection {
	return &Connection{db: db}
}

func (r *Connection) Create(ctx context.Context, conn *entity.CHConnection) error {
	funcName := "ConnectionRepository.Create"
	if err := helper.CheckDeadline(ctx); err != nil {
		return errwrap.Wrap(err, funcName)
	}

	return r.db.WithContext(ctx).Create(conn).Error
}

func (r *Connection) FindAll(ctx context.Context) ([]*entity.CHConnection, error) {
	funcName := "ConnectionRepository.FindAll"
	if err := helper.CheckDeadline(ctx); err != nil {
		return nil, errwrap.Wrap(err, funcName)
	}

	var conns []*entity.CHConnection
	err := r.db.WithContext(ctx).Find(&conns).Error
	if err != nil {
		return nil, errwrap.Wrap(err, funcName)
	}
	return conns, nil
}

func (r *Connection) FindByID(ctx context.Context, id int64) (*entity.CHConnection, error) {
	funcName := "ConnectionRepository.FindByID"
	if err := helper.CheckDeadline(ctx); err != nil {
		return nil, errwrap.Wrap(err, funcName)
	}

	var conn entity.CHConnection
	err := r.db.WithContext(ctx).First(&conn, id).Error
	if err != nil {
		if errwrap.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil if not found, let usecase handle standard error
		}
		return nil, errwrap.Wrap(err, funcName)
	}
	return &conn, nil
}

func (r *Connection) Delete(ctx context.Context, id int64) error {
	funcName := "ConnectionRepository.Delete"
	if err := helper.CheckDeadline(ctx); err != nil {
		return errwrap.Wrap(err, funcName)
	}

	return r.db.WithContext(ctx).Delete(&entity.CHConnection{}, id).Error
}

func (r *Connection) Update(ctx context.Context, conn *entity.CHConnection) error {
	funcName := "ConnectionRepository.Update"
	if err := helper.CheckDeadline(ctx); err != nil {
		return errwrap.Wrap(err, funcName)
	}

	return r.db.WithContext(ctx).Save(conn).Error
}
