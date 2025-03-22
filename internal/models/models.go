// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package models

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type User struct {
	ID        int32              `json:"id"`
	Name      string             `json:"name"`
	Email     string             `json:"email"`
	Password  string             `json:"password"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}
