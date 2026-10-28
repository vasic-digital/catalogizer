#!/usr/bin/env bash
# init_env_security <dir>
# Creates <dir>/.env.security from <dir>/.env.security.example (placeholders
# only, mode 0600). Never overwrites an existing file and refuses to touch a
# path tracked by git, so a real credential can never be written into (or be
# committed from) a tracked file. Prints no values.
init_env_security() {
    local dir="${1:?init_env_security: directory required}"
    local target="$dir/.env.security" example="$dir/.env.security.example"
    if git -C "$dir" ls-files --error-unmatch .env.security >/dev/null 2>&1; then
        echo "refusing: .env.security is tracked by git (run: git rm --cached .env.security)" >&2
        return 3
    fi
    if [ -e "$target" ]; then
        echo ".env.security already exists; left untouched"
        return 0
    fi
    if [ ! -f "$example" ]; then
        echo "refusing: $example missing; nothing written" >&2
        return 2
    fi
    ( umask 077 && cp "$example" "$target" ) || return 1
    chmod 600 "$target"
    echo "Created .env.security from .env.security.example (placeholders; fill in real values, never commit)"
}
