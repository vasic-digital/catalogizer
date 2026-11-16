#!/bin/bash
set -u
R=/home/milosvasic/.cache/claude-tmp/umeta.Rx6M5U/repo
cd "$R/catalog-api" || exit 1
O=../docs/qa/horizontal-authz-2026-11-16/o1/mutants.txt
: > $O
run() { # id file sedexpr testcmd
  id=$1; f=$2; expr=$3; shift 3
  cp "$f" "/tmp/o1_bak_$id"
  python3 - "$f" "$expr" <<'PY'
import sys
f,e=sys.argv[1],sys.argv[2]
a,b=e.split('=>>')
s=open(f).read()
assert a in s,"pattern absent"
open(f,'w').write(s.replace(a,b,1))
PY
  if [ $? -ne 0 ]; then echo "$id MUTATION-NOT-APPLIED" >> $O; cp /tmp/o1_bak_$id "$f"; return; fi
  "$@" >/tmp/o1_mut_$id.log 2>&1; rc=$?
  cp /tmp/o1_bak_$id "$f"
  echo "$id rc=$rc $([ $rc -ne 0 ] && echo KILLED || echo SURVIVED)" >> $O
}
run M1_live_drop_user_filter handlers/media_entity_handler.go 'GetByItemAndUser(c.Request.Context(), id, userID)=>>GetByItemAndUser(c.Request.Context(), id, 2)' go test -short -count=1 ./handlers -run UserMetadataReadCallerScoped
run M2_live_drop_404_mapping handlers/media_entity_handler.go 'utils.SendErrorResponse(c, http.StatusNotFound, "User metadata not found", nil)=>>c.JSON(http.StatusOK, gin.H{})' go test -short -count=1 ./handlers -run UserMetadataReadCallerScoped
run M3_unroute main.go 'entityGroup.GET("/:id/user-metadata", mediaEntityHandler.GetUserMetadata)=>>_ = mediaEntityHandler.GetUserMetadata' go test -count=1 -run TestHorizontalAuthz_RealBinary .
run M4_legacy_drop_user_filter internal/handlers/media.go 'WHERE media_item_id = ? AND user_id = ?=>>WHERE media_item_id = ? AND ? = ?' go test -short -count=1 ./internal/handlers -run UserMetadata_CallerScoped
run M5_legacy_drop_anon_guard internal/handlers/media.go 'if userID <= 0 {=>>if false {' go test -short -count=1 ./internal/handlers -run UserMetadata_CallerScoped
run M6_live_drop_auth_401 handlers/media_entity_handler.go 'Authentication required", nil)
		return
	}
	um, err := h.userMetaRepo.GetByItemAndUser=>>Authentication required", nil)
		return
	}
	userID = 2
	um, err := h.userMetaRepo.GetByItemAndUser' go test -short -count=1 ./handlers -run UserMetadataReadCallerScoped
echo ALLDONE >> $O
