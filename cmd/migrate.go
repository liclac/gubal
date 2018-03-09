package cmd

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"go.uber.org/multierr"

	"github.com/mattes/migrate"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func migrationsDir() string {
	wd, _ := os.Getwd()
	return path.Join(wd, "migrations")
}

func newMigrate() (*migrate.Migrate, error) {
	return migrate.New("file://"+migrationsDir(), viper.GetString("db"))
}

// Helper to make error handling from migrate.Migrate.Close() cleaner.
func closeAfterMigrate(m *migrate.Migrate, err error) error {
	srcerr, dberr := m.Close()
	return multierr.Combine(err, srcerr, dberr)
}

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Run database migrations.`,
}

// migrateCreateCmd represents the migrate create command
var migrateCreateCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "Create a new migration",
	Long:  `Create a new migration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := migrationsDir()
		basename := fmt.Sprintf("%d_%s", time.Now().Unix(), args[0])
		if _, err := os.Create(path.Join(dir, basename+".up.sql")); err != nil {
			return err
		}
		if _, err := os.Create(path.Join(dir, basename+".down.sql")); err != nil {
			return err
		}
		return nil
	},
}

// migrateUpCmd represents the migrate up command
var migrateUpCmd = &cobra.Command{
	Use:   "up [steps]",
	Short: "Apply migrations",
	Long:  `Apply migrations.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrate()
		if err != nil {
			return err
		}
		if len(args) > 0 {
			steps, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			return closeAfterMigrate(m, m.Steps(steps))
		}
		return closeAfterMigrate(m, m.Up())
	},
}

// migrateDownCmd represents the migrate down command
var reallyMigrateDown = false
var migrateDownCmd = &cobra.Command{
	Use:   "down [steps]",
	Short: "Revert migrations",
	Long:  `Revert migrations.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrate()
		if err != nil {
			return err
		}
		if len(args) > 0 {
			steps, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			return closeAfterMigrate(m, m.Steps(-steps))
		}
		if !reallyMigrateDown {
			return errors.New("this will revert every migration, pass a number of migrations (eg. `migrate down 1`) to revert only a few steps, or --yes if you really meant to do that")
		}
		return closeAfterMigrate(m, m.Down())
	},
}

// migrateForceCommand represents the migrate force command
var migrateForceCommand = &cobra.Command{
	Use:   "force VERSION",
	Short: "Force set version, eg. after a failed migration",
	Long:  `Force set version, eg. after a failed migration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrate()
		if err != nil {
			return err
		}
		ver, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}
		return closeAfterMigrate(m, m.Force(ver))
	},
}

// migrateVersionCommand represents the migrate version command
var migrateVersionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print the current database version",
	Long:  `Print the current database version.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := newMigrate()
		if err != nil {
			return err
		}
		ver, dirty, err := m.Version()
		fmt.Printf("%d (dirty: %v)\n", ver, dirty)
		return closeAfterMigrate(m, err)
	},
}

// migrateDropCmd represents the migrate drop command
var reallyMigrateDrop = false
var migrateDropCmd = &cobra.Command{
	Use:   "drop",
	Short: "Drop all tables in the database",
	Long:  `Drop all tables in the database.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !reallyMigrateDrop {
			return errors.New("this will drop all tables in your entire database, pass --yes if you really meant to do that")
		}
		m, err := newMigrate()
		if err != nil {
			return err
		}
		return closeAfterMigrate(m, m.Drop())
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.AddCommand(migrateCreateCmd)

	migrateCmd.AddCommand(migrateUpCmd)

	migrateCmd.AddCommand(migrateDownCmd)
	migrateDownCmd.Flags().BoolVar(&reallyMigrateDown, "yes", false, "really revert all migrations?")

	migrateCmd.AddCommand(migrateForceCommand)

	migrateCmd.AddCommand(migrateVersionCommand)

	migrateCmd.AddCommand(migrateDropCmd)
	migrateDropCmd.Flags().BoolVar(&reallyMigrateDrop, "yes", false, "really drop your whole database?")
}
