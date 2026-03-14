package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newOrgsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "orgs", Short: "Organization management commands"}

	var createName string
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an organization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if createName == "" {
				return fmt.Errorf("name is required")
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/orgs", map[string]any{"name": createName}, true)
		},
	}
	createCmd.Flags().StringVar(&createName, "name", "", "Organization name")

	membersCmd := &cobra.Command{
		Use:   "members",
		Short: "List members in current organization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runJSON(cmd, http.MethodGet, "/api/v1/orgs/members", nil, true)
		},
	}

	var inviteEmail, inviteRole string
	inviteCmd := &cobra.Command{
		Use:   "invite",
		Short: "Invite a member",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if inviteEmail == "" {
				return fmt.Errorf("email is required")
			}
			if inviteRole == "" {
				inviteRole = "developer"
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/orgs/invite", map[string]any{"email": inviteEmail, "role": inviteRole}, true)
		},
	}
	inviteCmd.Flags().StringVar(&inviteEmail, "email", "", "Invitee email")
	inviteCmd.Flags().StringVar(&inviteRole, "role", "developer", "Invitee role: owner|admin|developer|viewer")

	var acceptToken string
	acceptCmd := &cobra.Command{
		Use:   "accept-invite",
		Short: "Accept an organization invite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if acceptToken == "" {
				return fmt.Errorf("token is required")
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/orgs/accept-invite", map[string]any{"token": acceptToken}, true)
		},
	}
	acceptCmd.Flags().StringVar(&acceptToken, "token", "", "Invite token")

	var updateUserID, updateRole string
	updateRoleCmd := &cobra.Command{
		Use:   "update-role",
		Short: "Update a member role (owner only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if updateUserID == "" || updateRole == "" {
				return fmt.Errorf("user-id and role are required")
			}
			path := fmt.Sprintf("/api/v1/orgs/members/%s", updateUserID)
			return runJSON(cmd, http.MethodPatch, path, map[string]any{"role": updateRole}, true)
		},
	}
	updateRoleCmd.Flags().StringVar(&updateUserID, "user-id", "", "Target user ID")
	updateRoleCmd.Flags().StringVar(&updateRole, "role", "", "New role: admin|developer|viewer")

	var removeUserID string
	removeCmd := &cobra.Command{
		Use:   "remove-member",
		Short: "Remove a member from organization",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if removeUserID == "" {
				return fmt.Errorf("user-id is required")
			}
			path := fmt.Sprintf("/api/v1/orgs/members/%s", removeUserID)
			return runJSON(cmd, http.MethodDelete, path, nil, true)
		},
	}
	removeCmd.Flags().StringVar(&removeUserID, "user-id", "", "Target user ID")

	cmd.AddCommand(createCmd, membersCmd, inviteCmd, acceptCmd, updateRoleCmd, removeCmd)
	return cmd
}
