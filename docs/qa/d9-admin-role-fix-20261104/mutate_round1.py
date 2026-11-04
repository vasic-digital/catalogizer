import sys
m=sys.argv[1]
H='handlers/admin_handler.go'; R='repository/user_repository.go'
M={
'R3_update_instead_of_updateguarded':(H,"if g, ok := h.userRepo.(guardedUserUpdater); ok {","if g, ok := h.userRepo.(guardedUserUpdater); ok && false {"),
'R4a_count_inactive_admins':(R,"o.is_active = ?","o.is_active IN (?, 0)"),
'R4b_count_locked_admins':(R,"o.is_locked = ?","o.is_locked IN (?, 1)"),
'R5_audit_event_renamed':(H,'"admin_user_role_changed"','"x_role_changed"'),
'R6_first_revoke_ignores_role':(H,"	if needRevoke {\n		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {\n			c.JSON","	if wasActive && !user.IsActive {\n		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {\n			c.JSON"),
'R7_no_tolower_in_role_names':(H,"m[r.ID] = strings.ToLower(r.Name)","m[r.ID] = r.Name"),
'R8_revoke_keyed_on_diff_only':(H,"needRevoke := (updates.Role != nil && (roleChanged || id != admin.ID)) ||","needRevoke := roleChanged ||"),
'R9_update_guard_disabled':(R,"removesAdmin := expectedActive &&","removesAdmin := false && expectedActive &&"),
'R10_delete_guard_disabled':(R,'		del += " AND " + clause\n','		_ = clause\n'),
}
f,a,b=M[m]
s=open(f).read()
assert a in s,m
open(f,'w').write(s.replace(a,b,1))
