//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/docker/go-connections/nat"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
)

func setupMySQLContainer(t require.TestingT, ctx context.Context) (string, func()) {
	const (
		dbName = "sse_demo_test"
		user   = "testuser"
		pass   = "testpass"
	)

	container, err := mysql.RunContainer(
		ctx,
		mysql.WithDatabase(dbName),
		mysql.WithUsername(user),
		mysql.WithPassword(pass),
	)
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, nat.Port("3306/tcp"))
	require.NoError(t, err)

	dsn := user + ":" + pass + "@tcp(" + host + ":" + port.Port() + ")/" + dbName + "?parseTime=true&loc=UTC&multiStatements=true"

	dbConn, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer dbConn.Close()

	applySchema(t, dbConn)

	cleanup := func() {
		_ = container.Terminate(ctx)
	}
	return dsn, cleanup
}

func applySchema(t require.TestingT, dbConn *sql.DB) {
	root, err := os.Getwd()
	require.NoError(t, err)

	schemaPath := filepath.Join(root, "..", "..", "..", "db", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	require.NoError(t, err)

	_, err = dbConn.Exec(string(schema))
	require.NoError(t, err)
}
