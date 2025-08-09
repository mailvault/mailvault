package pg

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var (
	dockPool *dockertest.Pool
	dockRes  *dockertest.Resource
)

func TestMain(m *testing.M) {
	var err error
	dockPool, err = dockertest.NewPool("")
	if err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	opts := &dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_USER=postgres",
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_DB=privatemail_test",
		},
		ExposedPorts: []string{"5432/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"5432/tcp": {{HostIP: "127.0.0.1", HostPort: ""}}, // request random host port
		},
	}

	dockRes, err = dockPool.RunWithOptions(opts)
	if err != nil {
		panic(fmt.Sprintf("Could not start postgres: %s", err))
	}

	// Wait for the DB to be ready
	if err := dockPool.Retry(func() error {
		conn, err := pgxpool.New(context.Background(), testDSN())
		if err != nil {
			return err
		}
		defer conn.Close()
		return conn.Ping(context.Background())
	}); err != nil {
		panic(fmt.Sprintf("Could not connect to postgres: %s", err))
	}

	// Ensure required extensions exist before migrations
	if err := withConn(func(pool *pgxpool.Pool) error {
		_, err := pool.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS pgcrypto")
		return err
	}); err != nil {
		panic(fmt.Sprintf("Could not create extension: %s", err))
	}

	// Run migrations from this package's migrations directory
	mig, err := migrate.New(
		"file://migrations",
		testDSN(),
	)
	if err != nil {
		panic(fmt.Sprintf("Could not create migrator: %s", err))
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		panic(fmt.Sprintf("Could not run migrations: %s", err))
	}

	code := m.Run()

	// Cleanup the container
	if err := dockPool.Purge(dockRes); err != nil {
		panic(fmt.Sprintf("Could not purge docker resource: %s", err))
	}

	os.Exit(code)
}

func testDSN() string {
	// Use the dynamically assigned host port
	hostPort := dockRes.GetPort("5432/tcp")
	return fmt.Sprintf("postgres://postgres:postgres@localhost:%s/privatemail_test?sslmode=disable", hostPort)
}

func withConn(fn func(*pgxpool.Pool) error) error {
	pool, err := pgxpool.New(context.Background(), testDSN())
	if err != nil {
		return err
	}
	defer pool.Close()
	return fn(pool)
}
