import sys, pathlib
p = pathlib.Path(sys.argv[1])
name = sys.argv[2]
M = {
    "R1-null-guard-dropped": ("q += `duration_total IS NULL`", "q += `(duration_total IS NULL OR 1=1)`"),
    "R2-unit-guard-dropped": ("AND position_unit = ? AND `", "AND (position_unit = ? OR 1=1) AND `"),
    "R3a-value-guard-ge": ("q += `duration_total = ?`", "q += `duration_total >= ?`"),
    "R3b-value-guard-le": ("q += `duration_total = ?`", "q += `duration_total <= ?`"),
    "R4-no-insert-retry": ("\t\t\tif attempt == 0 {\n\t\t\t\tcontinue", "\t\t\tif false {\n\t\t\t\tcontinue"),
    "R5-zero-rows-break": ("\t\t\t\tcontinue\n\t\t\t}\n\t\t\tbreak", "\t\t\t\tbreak\n\t\t\t}\n\t\t\tbreak"),
    "R6-max-attempts-2": ("const maxAttempts = 3", "const maxAttempts = 2"),
    "R7-update-not-user-scoped": ("WHERE user_id = ? AND media_item_id = ? AND position_unit", "WHERE (user_id = ? OR 1=1) AND media_item_id = ? AND position_unit"),
    "R8-unit-guard-literal-seconds": ("args := []interface{}{dur, pos, now, callerID, id, unit}", "args := []interface{}{dur, pos, now, callerID, id, \"seconds\"}"),
}
old, new = M[name]
s = p.read_text()
assert s.count(old) == 1, (name, s.count(old))
p.write_text(s.replace(old, new))
print("applied", name)
