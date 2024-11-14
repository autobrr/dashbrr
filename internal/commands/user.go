package commands

import (
	"fmt"

	"github.com/autobrr/dashbrr/internal/types"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func UserCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "user",
		Short: "user",
		Long:  `user`,
		Example: `  dashbrr user 
  dashbrr user --help`,
		SilenceUsage: true,
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	}

	command.AddCommand(UserCreateCommand())
	command.AddCommand(UserChangePasswordCommand())

	return command
}

func UserCreateCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "create",
		Short: "create",
		Long:  `create`,
		Example: `  dashbrr user create <username> <password>
  dashbrr user create --help`,
		//SilenceUsage: true,
		Args: cobra.MinimumNArgs(2),
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		//if len(args) < 3 {
		//	return errors.New("usage: user create <username> <password>")
		//}

		username := args[1]
		password := args[2]

		email := fmt.Sprintf("%s@dashbrr.local", args[1])
		if len(args) >= 3 {
			email = args[3]
		}

		// Validate username and password
		if len(username) < 3 || len(username) > 32 {
			return errors.New("username must be between 3 and 32 characters")
		}
		if len(password) < 8 {
			return errors.New("password must be at least 8 characters long")
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// Check if username or email already exists
		existingUser, err := db.FindUser(cmd.Context(), types.FindUserParams{Username: username, Email: email})
		if err != nil {
			return fmt.Errorf("error checking username: %v", err)
		}
		if existingUser != nil {
			return fmt.Errorf("username %s already exists", username)
		}

		////existingUser, err = c.db.GetUserByEmail(email)
		//existingUser, err = c.db.FindUser(context.Background(), database.FindUserParams{Email: email})
		//if err != nil {
		//	return fmt.Errorf("error checking email: %v", err)
		//}
		//if existingUser != nil {
		//	return fmt.Errorf("email %s already exists", email)
		//}

		// Hash the password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %v", err)
		}

		// Create user
		user := &types.User{
			Username:     username,
			Email:        email,
			PasswordHash: string(passwordHash),
		}

		if err := db.CreateUser(cmd.Context(), user); err != nil {
			return fmt.Errorf("failed to create user: %v", err)
		}

		fmt.Printf("User %s created successfully\n", username)

		return nil
	}

	return command
}

func UserChangePasswordCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "change-password",
		Short: "change-password",
		Long:  `change-password`,
		Example: `  dashbrr user change-password <username> <new-password>
  dashbrr user change-password --help`,
		//SilenceUsage: true,
		Args: cobra.ExactArgs(2),
	}

	command.RunE = func(cmd *cobra.Command, args []string) error {
		username := args[1]
		newPassword := args[2]

		// Validate new password
		if len(newPassword) < 8 {
			return errors.New("new password must be at least 8 characters long")
		}

		db, err := initializeDatabase()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %v", err)
		}

		// Retrieve user
		user, err := db.FindUser(cmd.Context(), types.FindUserParams{Username: username})
		if err != nil {
			return fmt.Errorf("failed to find user: %v", err)
		}
		if user == nil {
			return fmt.Errorf("user %s not found", username)
		}

		// Hash the new password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash new password: %v", err)
		}

		// Update password
		if err := db.UpdateUserPassword(cmd.Context(), user.ID, string(passwordHash)); err != nil {
			return fmt.Errorf("failed to update password: %v", err)
		}

		fmt.Printf("Password changed successfully for user %s\n", username)
		return nil
	}

	return command
}
