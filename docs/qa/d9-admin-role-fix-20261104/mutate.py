import sys
m=sys.argv[1]
p='handlers/admin_handler.go'
s=open(p).read()
M={
'M1_role_not_applied':("			user.RoleID = newRole.ID\n","			_ = newRole\n"),
'M2_self_demotion_allowed':("				if id == admin.ID {","				if false && id == admin.ID {"),
'M3_no_session_revoke_on_role_change':("roleChanged || wasActive","false && roleChanged || wasActive"),
'M4_list_default_user':("		} else if n, ok := roleNames[u.RoleID]; ok {\n			roleName = n\n","		} else if _, ok := roleNames[u.RoleID]; ok {\n			roleName = \"user\"\n"),
'M5_unknown_role_accepted':("		if newRole == nil {\n			c.JSON(http.StatusBadRequest, gin.H{\"error\": \"Invalid role\"})\n			return\n		}","		if newRole == nil {\n			newRole = &roles[len(roles)-1]\n		}"),
}
a,b=M[m]
assert a in s,m
s=s.replace(a,b) if m=='M3_no_session_revoke_on_role_change' else s.replace(a,b,1)
open(p,'w').write(s)
