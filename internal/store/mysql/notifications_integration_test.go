//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sse_demo/internal/db"
	"sse_demo/internal/domain"
	"sse_demo/internal/model"
)

func TestMySQLStoreIntegration(t *testing.T) {
	ctx := context.Background()
	dsn, cleanup := setupMySQLContainer(t, ctx)
	defer cleanup()

	dbConn, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	queries := db.New(dbConn)
	store := New(queries, zap.NewNop())

	created, err := store.CreateNotification(ctx, model.Notification{
		Room:  "room-1",
		Type:  domain.NotificationTypeInfo,
		Title: "title",
		Body:  "body",
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Equal(t, "room-1", created.Room)
	require.Equal(t, domain.NotificationTypeInfo, created.Type)

	history, err := store.ListNotifications(ctx, "room-1", 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, created.ID, history[0].ID)
	require.Equal(t, created.Type, history[0].Type)

}

// setupMySQLContainer is defined in testhelpers_integration.go
