// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package user

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/autobrr/dashbrr/internal/commands/base"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/types"
)

type UserCommand struct {
	*base.BaseCommand
	db *database.DB
}

func NewUserCommand(db *database.DB) *UserCommand {
	return &UserCommand{
		BaseCommand: base.NewBaseCommand(
			"user",
			"Manage users in the system",
			"<subcommand> [arguments]\n\n  Subcommands:\n    create <username> <password>\n    change-password <username> <new_password>",
		),
		db: db,
	}
}

func (c *UserCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("insufficient arguments. %s", c.Usage())
	}

	subcommand := args[0]
	switch subcommand {
	case "create":
		if len(args) < 3 {
			return errors.New("usage: user create <username> <password>")
		}
		email := fmt.Sprintf("%s@dashbrr.local", args[1])
		if len(args) > 3 {
			email = args[3]
		}
		return c.createUser(args[1], args[2], email)
	case "change-password":
		if len(args) < 3 {
			return errors.New("usage: user change-password <username> <new_password>")
		}
		return c.changePassword(args[1], args[2])
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func (c *UserCommand) createUser(username, password, email string) error {
	// Validate username and password
	if len(username) < 3 || len(username) > 32 {
		return errors.New("username must be between 3 and 32 characters")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	// Check if username or email already exists
	existingUser, err := c.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("error checking username: %v", err)
	}
	if existingUser != nil {
		return fmt.Errorf("username %s already exists", username)
	}

	existingUser, err = c.db.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("error checking email: %v", err)
	}
	if existingUser != nil {
		return fmt.Errorf("email %s already exists", email)
	}

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

	if err := c.db.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	fmt.Printf("User %s created successfully\n", username)
	return nil
}

func (c *UserCommand) changePassword(username, newPassword string) error {
	// Validate new password
	if len(newPassword) < 8 {
		return errors.New("new password must be at least 8 characters long")
	}

	// Retrieve user
	user, err := c.db.GetUserByUsername(username)
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
	if err := c.db.UpdateUserPassword(user.ID, string(passwordHash)); err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	fmt.Printf("Password changed successfully for user %s\n", username)
	return nil
}
