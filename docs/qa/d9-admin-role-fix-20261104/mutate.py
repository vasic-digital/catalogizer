import sys
m=sys.argv[1]
H='handlers/admin_handler.go'; R='repository/user_repository.go'
M={
'M1_role_not_applied':[(H,"			user.RoleID = newRole.ID\n","			_ = newRole\n")],
'M2_self_demotion_allowed':[(H,"				if id == admin.ID {","				if false && id == admin.ID {")],
'M3_no_session_revoke_on_role_change':[(H,"needRevoke := (updates.Role != nil && (roleChanged || id != admin.ID)) ||","needRevoke := false && roleChanged ||")],
'M4_list_default_user':[(H,"		} else if n, ok := roleNames[u.RoleID]; ok {\n			roleName = n\n","		} else if _, ok := roleNames[u.RoleID]; ok {\n			roleName = \"user\"\n")],
'M5_unknown_role_accepted':[(H,"		if newRole == nil {\n			c.JSON(http.StatusBadRequest, gin.H{\"error\": \"Invalid role\"})\n			return\n		}","		if newRole == nil {\n			newRole = &roles[len(roles)-1]\n		}")],
# M6 (round 1): self-demotion check AND the atomic repository last-admin guard both removed.
'M6_self_and_last_admin_checks_removed':[(H,"				if id == admin.ID {","				if false && id == admin.ID {"),(R,"removesAdmin := expectedActive &&","removesAdmin := false && expectedActive &&")],
}
for f,a,b in M[m]:
    s=open(f).read(); assert a in s,(m,a[:40]); open(f,'w').write(s.replace(a,b,1))
