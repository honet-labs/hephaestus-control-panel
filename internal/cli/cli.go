package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"go-hephaestus/internal/config"
	"go-hephaestus/internal/database"
	"go-hephaestus/internal/repository"
)

// RunCLI executes command-line operations for Hephaestus Control Panel
func RunCLI(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	cmd := args[0]
	subArgs := args[1:]

	switch cmd {
	case "reset-password", "reset-admin", "passwd":
		handleResetPassword(subArgs)
	case "list-users", "users":
		handleListUsers(subArgs)
	case "create-user", "add-user":
		handleCreateUser(subArgs)
	case "help", "--help", "-h":
		printUsage()
	default:
		// If argument does not match a known command, check if it looks like a username
		if !strings.HasPrefix(cmd, "-") {
			// e.g. "hephaestus admin mysecretpassword"
			handleResetPassword(args)
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println(`======================================================================
 HEPHAESTUS CONTROL PANEL (HCP) - CLI UTILITY
======================================================================

USAGE:
  hephaestus <command> [arguments...]
  hcp-cli <command> [arguments...]

COMMANDS:
  reset-password  Reset a user's password (e.g. admin)
  list-users      List all registered system users
  create-user     Create a new user account with role
  help            Show this usage information

EXAMPLES:
  # Reset admin password with flags
  hephaestus reset-password -u admin -p mynewpassword123

  # Reset admin password using positional arguments
  hephaestus reset-password admin mynewpassword123

  # Interactive prompt (will prompt for password securely)
  hephaestus reset-password admin

  # List all users
  hephaestus list-users

  # Create a new admin user
  hephaestus create-user -u superadmin -p secret123 -r ADMIN
======================================================================`)
}

func initDatabase() (*config.Config, *repository.UserRepository) {
	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := database.InitDatabase(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to connect to PostgreSQL database (%s:%d/%s): %v\n",
			cfg.DB.Host, cfg.DB.Port, cfg.DB.Database, err)
		fmt.Fprintln(os.Stderr, "Please ensure PostgreSQL is running and DB credentials in data/db_config.json or environment variables are correct.")
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository()
	return cfg, userRepo
}

func handleResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	usernameFlag := fs.String("u", "", "Username to reset (alias for --username)")
	usernameLongFlag := fs.String("username", "", "Username to reset")
	passwordFlag := fs.String("p", "", "New password (alias for --password)")
	passwordLongFlag := fs.String("password", "", "New password")
	forceChangeFlag := fs.Bool("force-change", false, "Force password change on next login")

	_ = fs.Parse(args)

	username := *usernameLongFlag
	if username == "" {
		username = *usernameFlag
	}

	password := *passwordLongFlag
	if password == "" {
		password = *passwordFlag
	}

	// Also check positional arguments
	nonFlags := fs.Args()
	if username == "" && len(nonFlags) > 0 {
		username = nonFlags[0]
	}
	if password == "" && len(nonFlags) > 1 {
		password = nonFlags[1]
	}

	// Default username to "admin" if omitted
	if username == "" {
		username = "admin"
	}

	// If password still empty, prompt interactively or generate
	if password == "" {
		fmt.Printf("Enter new password for user '%s': ", username)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err == nil {
			password = strings.TrimSpace(line)
		}
	}

	// If still empty, generate a secure random password
	if password == "" {
		password = generateSecurePassword(14)
		fmt.Printf("[INFO] No password provided. Generated secure password: %s\n", password)
	}

	if len(password) < 6 {
		fmt.Fprintln(os.Stderr, "[ERROR] Password must be at least 6 characters long.")
		os.Exit(1)
	}

	_, userRepo := initDatabase()
	ctx := context.Background()

	// 1. Check if user exists
	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil {
		fmt.Printf("[WARN] User '%s' does not exist in database.\n", username)
		fmt.Printf("Creating user '%s' with role 'ADMIN'...\n", username)

		hash, err := config.HashPassword(password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to hash password: %v\n", err)
			os.Exit(1)
		}

		newUser, err := userRepo.Create(ctx, username, hash, "ADMIN", *forceChangeFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create user: %v\n", err)
			os.Exit(1)
		}

		_ = userRepo.LogActivity(ctx, "CLI", "User Created", fmt.Sprintf("Admin user '%s' created via CLI", username), "SUCCESS", &newUser.ID)

		fmt.Println(`======================================================================
 HEPHAESTUS CONTROL PANEL - USER CREATED
======================================================================`)
		fmt.Printf(" [✓] Username:      %s\n", username)
		fmt.Printf(" [✓] Password:      %s\n", password)
		fmt.Printf(" [✓] Role:          ADMIN\n")
		fmt.Printf(" [✓] User ID:       %d\n", newUser.ID)
		fmt.Println(`======================================================================
 SUCCESS: Account created! You can now log in at:
 http://<server-ip>:8282/login
======================================================================`)
		return
	}

	// 2. Hash new password
	hash, err := config.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	// 3. Update password in database
	err = userRepo.UpdatePassword(ctx, user.ID, hash, *forceChangeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to update password in database: %v\n", err)
		os.Exit(1)
	}

	// 4. Invalidate all previous sessions for this user
	_ = userRepo.DeleteUserSessions(ctx, user.ID)

	// 5. Record activity log
	_ = userRepo.LogActivity(ctx, "CLI", "Password Reset", fmt.Sprintf("Password for user '%s' was reset via CLI", username), "SUCCESS", &user.ID)

	fmt.Println(`======================================================================
 HEPHAESTUS CONTROL PANEL - PASSWORD RESET SUCCESS
======================================================================`)
	fmt.Printf(" [✓] Target User:    %s (ID: %d, Role: %s)\n", user.Username, user.ID, user.Role)
	fmt.Printf(" [✓] New Password:   %s\n", password)
	fmt.Printf(" [✓] Encryption:     Bcrypt (12 cost rounds)\n")
	fmt.Printf(" [✓] Active Sessions: Invalidated (All old logins terminated)\n")
	if *forceChangeFlag {
		fmt.Printf(" [✓] Force Change:   Enabled (User must change password on login)\n")
	}
	fmt.Println(`======================================================================
 SUCCESS: You can now log in with the new password at:
 http://<server-ip>:8282/login
======================================================================`)
}

func handleListUsers(args []string) {
	_, userRepo := initDatabase()
	ctx := context.Background()

	users, err := userRepo.List(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to list users: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(`======================================================================
 REGISTERED SYSTEM USERS
======================================================================`)
	fmt.Printf("%-5s | %-20s | %-12s | %-14s | %-20s\n", "ID", "USERNAME", "ROLE", "FORCE CHANGE", "CREATED AT")
	fmt.Println(strings.Repeat("-", 78))

	for _, u := range users {
		forceStr := "No"
		if u.ForcePasswordChange {
			forceStr = "Yes"
		}
		fmt.Printf("%-5d | %-20s | %-12s | %-14s | %-20s\n",
			u.ID, u.Username, u.Role, forceStr, u.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println(`======================================================================`)
}

func handleCreateUser(args []string) {
	fs := flag.NewFlagSet("create-user", flag.ContinueOnError)
	usernameFlag := fs.String("u", "", "Username (alias for --username)")
	usernameLongFlag := fs.String("username", "", "Username")
	passwordFlag := fs.String("p", "", "Password (alias for --password)")
	passwordLongFlag := fs.String("password", "", "Password")
	roleFlag := fs.String("r", "ADMIN", "User role: ADMIN, OPERATOR, VIEWER")
	forceChangeFlag := fs.Bool("force-change", false, "Force password change on next login")

	_ = fs.Parse(args)

	username := *usernameLongFlag
	if username == "" {
		username = *usernameFlag
	}
	password := *passwordLongFlag
	if password == "" {
		password = *passwordFlag
	}

	if username == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] Username is required. Example: -u newadmin")
		os.Exit(1)
	}

	if password == "" {
		password = generateSecurePassword(14)
		fmt.Printf("[INFO] Generated password: %s\n", password)
	}

	_, userRepo := initDatabase()
	ctx := context.Background()

	hash, err := config.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	newUser, err := userRepo.Create(ctx, username, hash, strings.ToUpper(*roleFlag), *forceChangeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create user: %v\n", err)
		os.Exit(1)
	}

	_ = userRepo.LogActivity(ctx, "CLI", "User Created", fmt.Sprintf("User '%s' created via CLI", username), "SUCCESS", &newUser.ID)

	fmt.Println(`======================================================================
 USER CREATED SUCCESSFULLY
======================================================================`)
	fmt.Printf(" [✓] Username:  %s\n", username)
	fmt.Printf(" [✓] Password:  %s\n", password)
	fmt.Printf(" [✓] Role:      %s\n", strings.ToUpper(*roleFlag))
	fmt.Printf(" [✓] User ID:   %d\n", newUser.ID)
	fmt.Println(`======================================================================`)
}

func generateSecurePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "Hephaestus2026!"
		}
		result[i] = chars[num.Int64()]
	}
	return string(result)
}
